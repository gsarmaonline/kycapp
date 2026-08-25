-- A version for the edge graph, so a revocation is visible to a cache.
--
-- The grant store already had one. AppAccessVersion is MAX(created_at) across
-- app_grants, app_roles and group members, and a merchant caches a grant set
-- against it rather than polling. reach_edges cannot answer the same question:
-- it has created_at and no updated_at, WriteReachEdge updates expires_at and
-- source without moving it, and a delete leaves nothing behind at all.
--
-- The failure that matters is the delete. Revoking access would not move any
-- timestamp, so a cache would keep serving the *wider* permission, which is
-- staleness in the direction that costs something. Adding this before edges
-- carry grants is much cheaper than adding it after.
--
-- A counter rather than a timestamp, because the question is "has anything
-- changed since the number I hold", and a counter answers it without any
-- opinion about clocks, and without a MAX() over a table that is meant to grow
-- without limit.
--
-- One row per namespace is a write-serialisation point inside a namespace. That
-- is a known cost and it is accepted: correctness first, and caching and
-- fetching get a proper pass later.

CREATE TABLE IF NOT EXISTS reach_namespace_versions (
    namespace  TEXT PRIMARY KEY,
    version    BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every namespace that already holds edges starts at 1 rather than 0, so a
-- caller that stored 0 for "I have seen nothing" is not told it is current.
INSERT INTO reach_namespace_versions (namespace, version)
SELECT DISTINCT namespace, 1 FROM reach_edges
ON CONFLICT (namespace) DO NOTHING;
