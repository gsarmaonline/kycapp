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
