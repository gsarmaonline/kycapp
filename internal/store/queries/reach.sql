-- Queries for the relation-graph edge table.
--
-- ListReachEdges is the whole read path of the evaluator, and it is an exact
-- prefix of the primary key, so it is an index lookup however large the table
-- grows. Expiry is deliberately not filtered here: the evaluator is handed the
-- time to decide against, so a test can pin it and production cannot disagree
-- with itself between the query and the walk.

-- name: ListReachEdges :many
SELECT * FROM reach_edges
WHERE namespace = $1
  AND object_type = $2
  AND object_id = $3
  AND relation = $4;

-- name: WriteReachEdge :exec
INSERT INTO reach_edges (
    namespace, object_type, object_id, relation,
    subject_type, subject_id, subject_relation, expires_at, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation)
DO UPDATE SET expires_at = EXCLUDED.expires_at, source = EXCLUDED.source;

-- name: DeleteReachEdge :exec
DELETE FROM reach_edges
WHERE namespace = $1
  AND object_type = $2
  AND object_id = $3
  AND relation = $4
  AND subject_type = $5
  AND subject_id = $6
  AND subject_relation = $7;

-- ListReachEdgesForSubject is the sweep: every edge naming one principal. It is
-- what offboarding uses, and it is the reason ownership can be lifecycle only.

-- name: ListReachEdgesForSubject :many
SELECT * FROM reach_edges
WHERE namespace = $1 AND subject_type = $2 AND subject_id = $3
ORDER BY object_type, object_id, relation;

-- The graph's version, so a cache can be invalidated by a revocation.
--
-- BumpReachNamespaceVersion runs in the same transaction as the write or delete
-- it accompanies. A delete is the case the whole thing exists for: it moves no
-- timestamp on any surviving row, so without this a revocation is invisible and
-- a cache keeps serving the wider permission.

-- name: BumpReachNamespaceVersion :one
INSERT INTO reach_namespace_versions (namespace, version, updated_at)
VALUES ($1, 1, now())
ON CONFLICT (namespace) DO UPDATE
SET version = reach_namespace_versions.version + 1, updated_at = now()
RETURNING version;

-- GetReachNamespaceVersion returns 0 for a namespace nothing has ever written,
-- which is the correct answer rather than a missing row: there is nothing to
-- have gone stale.
-- name: GetReachNamespaceVersion :one
SELECT COALESCE(
    (SELECT version FROM reach_namespace_versions WHERE namespace = $1),
    0
)::bigint AS version;

-- name: CountReachEdgesBySource :many
SELECT source, count(*)::bigint AS total
FROM reach_edges
WHERE namespace = $1
GROUP BY source
ORDER BY source;

-- ListLiveEdges reads the view that presents the current authorisation tables
-- as edges, unioned with any edge written directly. This is the evaluator's
-- source during the cutover: it reads the same rows the previous engine read,
-- so the two cannot disagree about state, only about how state is interpreted.

-- name: ListLiveEdges :many
SELECT * FROM reach_edges_live
WHERE namespace = $1
  AND object_type = $2
  AND object_id = $3
  AND relation = $4;

-- name: ListLiveEdgesForSubject :many
SELECT * FROM reach_edges_live
WHERE namespace = $1 AND subject_type = $2 AND subject_id = $3
ORDER BY object_type, object_id, relation;

-- ListOperatorRoleHierarchy answers "what do owner, admin and member actually
-- mean here?", which the schema map cannot: those three are rows, not types.
--
-- The chain was real in the database from the moment role_extends landed and
-- visible nowhere, because the operator roles UI is hidden and a membership
-- shows only its own role key.
-- name: ListOperatorRoleHierarchy :many
SELECT r.id, r.key, r.name,
       COALESCE(ARRAY(
         SELECT p.key FROM role_extends e
         JOIN roles p ON p.id = e.parent_id
         WHERE e.role_id = r.id
         ORDER BY p.key
       ), '{}')::text[] AS parent_keys,
       COALESCE(ARRAY(
         SELECT pm.key FROM role_permissions rp
         JOIN permissions pm ON pm.id = rp.permission_id
         WHERE rp.role_id = r.id
         ORDER BY pm.key
       ), '{}')::text[] AS permission_keys
FROM roles r
WHERE r.organisation_id = $1
ORDER BY r.key;

-- Instances, for the map to draw beside the model it belongs to.
--
-- The schema map answers "what can exist here". It cannot answer "what does
-- exist", because editor and apollo are rows, not types, and core/reach draws
-- the schema on purpose: the edge set is unbounded and drawing it whole
-- produces a hairball. This is the bounded middle. It reports the distinct
-- nodes per type, capped, so the picture gains the instances without gaining
-- the edges between them.
--
-- Six sources, because a merchant's model is still kept in two stores.
-- reach_edges is what the evaluator walks, and ProjectMerchant is what fills it
-- from the grant store -- but nothing in the running system calls that yet, so
-- a role created through the admin pages exists only in app_roles. Reading the
-- edge table alone would have drawn a map with no roles on it, which is the
-- exact complaint this feature answers. Each type is therefore read from every
-- place it can live.
--
-- UNION rather than UNION ALL throughout, and that is load-bearing rather than
-- tidy. A project named by a grant and by a containment edge is one project,
-- and ALL would report it twice and inflate the count the cap notice quotes.
--
-- Every branch is an index prefix or an organisation-scoped table scan on a
-- table a person authors by hand. The two large ones are the edge halves, and
-- those are the primary key and reach_edges_subject_idx.
--
-- The star node is excluded. object_id '*' is not an instance, it is the
-- wildcard standing for every instance of a kind, and counting it would report
-- one more project than the merchant has. Summary.wildcards already says the
-- schema has them.
--
-- total is the exact count per type, computed over the same materialised set
-- the ranking uses, so the caller can say "100 of 3,204" rather than "at least
-- 100" and a cap is visible as a cap.

-- name: ListMerchantInstances :many
WITH nodes AS (
    SELECT object_type AS node_type, object_id AS node_id
    FROM reach_edges
    WHERE reach_edges.namespace = @namespace::text AND reach_edges.object_id <> '*'
    UNION
    SELECT subject_type, subject_id
    FROM reach_edges
    WHERE reach_edges.namespace = @namespace::text AND reach_edges.subject_id <> '*'
    UNION
    -- Scopes a grant is written at. A merchant who only uses the admin pages
    -- has never written an edge, and this is the only place project:apollo is
    -- recorded for them.
    SELECT app_grants.scope_kind, app_grants.scope_id
    FROM app_grants
    WHERE app_grants.organisation_id = @org_id::text
      AND NOT app_grants.all_scopes
      AND app_grants.scope_id <> '*'
    UNION
    SELECT 'role'::text, app_roles.id
    FROM app_roles WHERE app_roles.organisation_id = @org_id::text
    UNION
    SELECT 'group'::text, app_user_groups.id
    FROM app_user_groups WHERE app_user_groups.organisation_id = @org_id::text
    UNION
    -- Archived customers are left out. They are still rows, so a walk can still
    -- name one, but they are not part of the model a merchant is looking at and
    -- they would crowd the cap out with people who have left.
    SELECT 'app_user'::text, app_users.id
    FROM app_users
    WHERE app_users.organisation_id = @org_id::text AND app_users.status <> 'archived'
),
ranked AS (
    SELECT node_type, node_id,
           row_number() OVER (PARTITION BY node_type ORDER BY node_id) AS rn,
           count(*)     OVER (PARTITION BY node_type)                  AS total
    FROM nodes
)
SELECT node_type, node_id, total::bigint AS total
FROM ranked
WHERE rn <= @per_type::int
ORDER BY node_type, node_id;

-- Labels for the opaque ids the projection writes.
--
-- app_users is the one instance table with no ceiling, so this takes the ids
-- the cap already selected rather than listing the organisation. At most
-- per_type rows come back however many customers exist.
--
-- name: ListAppUserLabels :many
SELECT id, display_name, email, external_id
FROM app_users
WHERE organisation_id = $1 AND id = ANY(@ids::text[]);


-- What relates one instance to another.
--
-- Instances alone are a fan of identical chips. Three roles hanging off the
-- role type node say only "three roles exist", which a reader could already
-- have guessed, and the picture actively misleads: it draws member, admin and
-- viewer as interchangeable when the whole point of having three is that they
-- are not.
--
-- What separates them is here. admin extends member is an edge between two
-- rows, and it has been true since app_role_extends landed and drawn nowhere.
-- The same holds for a group inside a group, a document inside a project, and a
-- grant tying a scope to a role.
--
-- Both ends must be in the drawn set, which is what keeps this bounded. The cap
-- has already reduced each type to at most per_type nodes, so the edges among
-- them cannot run away even when the underlying table holds millions. An edge
-- to a node the cap excluded is dropped rather than drawn into empty space.
--
-- max_edges is the second bound, for the pathological case the first one misses:
-- a hundred documents in a hundred projects is ten thousand containment edges,
-- all of them within the cap. Ordered before it is applied, so the sample is
-- the same one every time rather than whatever the planner returned first.

-- name: ListMerchantInstanceEdges :many
WITH nodes AS (
    SELECT object_type AS node_type, object_id AS node_id
    FROM reach_edges
    WHERE reach_edges.namespace = @namespace::text AND reach_edges.object_id <> '*'
    UNION
    SELECT subject_type, subject_id
    FROM reach_edges
    WHERE reach_edges.namespace = @namespace::text AND reach_edges.subject_id <> '*'
    UNION
    -- Scopes a grant is written at. A merchant who only uses the admin pages
    -- has never written an edge, and this is the only place project:apollo is
    -- recorded for them.
    SELECT app_grants.scope_kind, app_grants.scope_id
    FROM app_grants
    WHERE app_grants.organisation_id = @org_id::text
      AND NOT app_grants.all_scopes
      AND app_grants.scope_id <> '*'
    UNION
    SELECT 'role'::text, app_roles.id
    FROM app_roles WHERE app_roles.organisation_id = @org_id::text
    UNION
    SELECT 'group'::text, app_user_groups.id
    FROM app_user_groups WHERE app_user_groups.organisation_id = @org_id::text
    UNION
    -- Archived customers are left out. They are still rows, so a walk can still
    -- name one, but they are not part of the model a merchant is looking at and
    -- they would crowd the cap out with people who have left.
    SELECT 'app_user'::text, app_users.id
    FROM app_users
    WHERE app_users.organisation_id = @org_id::text AND app_users.status <> 'archived'
),
ranked AS (
    SELECT node_type, node_id,
           row_number() OVER (PARTITION BY node_type ORDER BY node_id) AS rn
    FROM nodes
),
drawn AS (
    SELECT node_type, node_id FROM ranked WHERE rn <= @per_type::int
),
rel AS (
    -- The graph's own edges: containment, ownership, and any grant a merchant
    -- wrote directly. relation is kept verbatim, because can_read and parent
    -- mean different things and collapsing them would lose the distinction this
    -- query exists to restore.
    SELECT reach_edges.object_type AS from_type, reach_edges.object_id AS from_id,
           reach_edges.relation    AS label,
           reach_edges.subject_type AS to_type, reach_edges.subject_id AS to_id
    FROM reach_edges
    WHERE reach_edges.namespace = @namespace::text
      AND reach_edges.object_id <> '*'
      AND reach_edges.subject_id <> '*'
    UNION
    -- Role inheritance. The one relation a merchant can see the effect of and
    -- not the cause, because a role page shows its own capabilities and says
    -- nothing about the parent it draws the rest from.
    SELECT 'role'::text, e.role_id, 'extends'::text, 'role'::text, e.parent_id
    FROM app_role_extends e
    JOIN app_roles r ON r.id = e.role_id
    WHERE r.organisation_id = @org_id::text
    UNION
    SELECT 'group'::text, x.group_id, 'inside'::text, 'group'::text, x.parent_id
    FROM app_user_group_extends x
    JOIN app_user_groups g ON g.id = x.group_id
    WHERE g.organisation_id = @org_id::text
    UNION
    SELECT 'group'::text, m.group_id, 'member'::text, 'app_user'::text, m.app_user_id
    FROM app_user_group_members m
    JOIN app_user_groups mg ON mg.id = m.group_id
    WHERE mg.organisation_id = @org_id::text
    UNION
    -- A grant, as the one arrow it is rather than the fan of can_* edges the
    -- projection expands it into. A merchant wrote "maintainer, on apollo", and
    -- that is the sentence to draw.
    SELECT ag.scope_kind, ag.scope_id, 'grants'::text, 'role'::text, ag.role_id
    FROM app_grants ag
    WHERE ag.organisation_id = @org_id::text
      AND NOT ag.all_scopes
      AND ag.scope_id <> '*'
)
SELECT rel.from_type, rel.from_id, rel.label, rel.to_type, rel.to_id
FROM rel
JOIN drawn a ON a.node_type = rel.from_type AND a.node_id = rel.from_id
JOIN drawn b ON b.node_type = rel.to_type   AND b.node_id = rel.to_id
ORDER BY rel.from_type, rel.from_id, rel.label, rel.to_type, rel.to_id
LIMIT @max_edges::int;
