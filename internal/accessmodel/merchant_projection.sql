-- Projection of a merchant's access model into edges.
--
-- KYC built this exact bridge for its own tier and stopped. projection.sql
-- turns roles, role_permissions, role_extends and memberships into edges, and
-- every statement hard-codes the namespace 'kyc'. The merchant tier has the
-- same five shapes and had no projection at all, so POST /check could not see a
-- merchant's roles, their inheritance, their groups, or who was in them. Only
-- the vocabulary crossed over, because MerchantSchema reads app_scope_types and
-- app_capabilities to generate the schema. None of the data did.
--
-- That is the seam. Six of the eight customer-access pages wrote to a store the
-- graph could not read, and a merchant had no way to know which of the two to
-- use, or that using one made the other lie.
--
-- Every statement is ON CONFLICT DO NOTHING, so this is idempotent and safe to
-- re-run. The parameter is the organisation id, and the namespace is
-- 'org:' || $1, which is what keeps one merchant's open vocabulary out of every
-- other's.
--
-- Note for anyone editing: statements are split on the semicolon, so a comment
-- may not contain one.

-- Role inheritance. role_id extends parent_id, so whoever holds the child also
-- holds the parent. Stored as a userset rather than expanded, so the walk
-- resolves the chain and editing a base role reaches everything built on it.
--
-- app_roles.effective_capabilities stays the materialised form for the grant
-- statements below, which need a flat set to fan out over. The two are not
-- redundant: this edge carries holders, that column carries capabilities.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'org:' || $1, 'role', e.parent_id, 'holder', 'role', e.role_id, 'holder', 'app_role_extends'
FROM app_role_extends e
JOIN app_roles r ON r.id = e.role_id
WHERE r.organisation_id = $1
ON CONFLICT DO NOTHING;

-- Group nesting, which is the same mechanism under a second name. Both nest and
-- both confer through membership, which is why the proposal puts them on one
-- page.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'org:' || $1, 'group', x.parent_id, 'member_of', 'group', x.group_id, 'member_of', 'app_user_group_extends'
FROM app_user_group_extends x
JOIN app_user_groups g ON g.id = x.group_id
WHERE g.organisation_id = $1
ON CONFLICT DO NOTHING;

-- Group membership.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'org:' || $1, 'group', m.group_id, 'member_of', 'app_user', m.app_user_id, '', 'app_user_group_members'
FROM app_user_group_members m
JOIN app_user_groups g ON g.id = m.group_id
WHERE g.organisation_id = $1
ON CONFLICT DO NOTHING;

-- A grant, fanned out to one edge per capability the role confers.
--
-- The capability key is resource:action and only the action survives here. A
-- grant is written at a scope, and a scope kind answers every action in the
-- namespace, so project:apollo #can_read is what lets a role carrying
-- document:read reach a document inside apollo. The resource half is already
-- expressed by which type the resource is.
--
-- Three subject shapes, and each is a node the walk already understands: one
-- customer is app_user:<id>, a group is the userset group:<id>#member_of, and
-- everyone is the subject star app_user:*.
--
-- Scope has two wildcard levels and they are not the same thing. scope_id = '*'
-- is every instance of one declared kind and needs no translation, because the
-- star already lives in a node id. all_scopes is every kind at once, and such a
-- row carries no scope_kind at all, so it fans out to the star node of every
-- kind the merchant has declared. The lateral below is what makes one statement
-- cover both.
--
-- Expiry rides on the edge, so a time-boxed grant lapses with no job running.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, expires_at, source)
SELECT DISTINCT
    'org:' || $1,
    sc.kind,
    sc.id,
    'can_' || split_part(cap, ':', 2),
    CASE g.subject_kind WHEN 'group' THEN 'group' ELSE 'app_user' END,
    CASE g.subject_kind
        WHEN 'group'    THEN g.group_id
        WHEN 'everyone' THEN '*'
        ELSE g.app_user_id
    END,
    CASE g.subject_kind WHEN 'group' THEN 'member_of' ELSE '' END,
    g.expires_at,
    'app_grants'
FROM app_grants g
JOIN app_roles r ON r.id = g.role_id
CROSS JOIN LATERAL unnest(r.effective_capabilities) AS cap
CROSS JOIN LATERAL (
    SELECT g.scope_kind AS kind, g.scope_id AS id WHERE NOT g.all_scopes
    UNION ALL
    SELECT st.kind, '*' FROM app_scope_types st
    WHERE g.all_scopes AND st.organisation_id = $1
) sc
WHERE g.organisation_id = $1
  AND NOT g.all_capabilities
  -- A self_subject grant has no edge form that can be derived here. It said
  -- "your own rows", and KYC never learned which rows exist, let alone who owns
  -- them. Ownership is now an owner edge the merchant writes when it creates
  -- the resource, so these are skipped rather than mistranslated. Translating
  -- one as an ordinary grant would hand every customer every row in the scope.
  AND COALESCE(g.constraint_kind, '') <> 'self_subject'
  AND split_part(cap, ':', 2) <> ''
ON CONFLICT DO NOTHING;

-- The capability wildcard, as one edge rather than a fan-out.
--
-- can_all is unioned into every rule, so this keeps the standing property the
-- boolean had: declare an action next quarter and every holder gains it with no
-- grant rewritten. Expanding it here into a concrete list would have quietly
-- turned a wildcard into a snapshot.
--
-- Such a row carries no role, which is why there is no join to app_roles.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, expires_at, source)
SELECT DISTINCT
    'org:' || $1,
    sc.kind,
    sc.id,
    'can_all',
    CASE g.subject_kind WHEN 'group' THEN 'group' ELSE 'app_user' END,
    CASE g.subject_kind
        WHEN 'group'    THEN g.group_id
        WHEN 'everyone' THEN '*'
        ELSE g.app_user_id
    END,
    CASE g.subject_kind WHEN 'group' THEN 'member_of' ELSE '' END,
    g.expires_at,
    'app_grants'
FROM app_grants g
CROSS JOIN LATERAL (
    SELECT g.scope_kind AS kind, g.scope_id AS id WHERE NOT g.all_scopes
    UNION ALL
    SELECT st.kind, '*' FROM app_scope_types st
    WHERE g.all_scopes AND st.organisation_id = $1
) sc
WHERE g.organisation_id = $1
  AND g.all_capabilities
  AND COALESCE(g.constraint_kind, '') <> 'self_subject'
ON CONFLICT DO NOTHING;
