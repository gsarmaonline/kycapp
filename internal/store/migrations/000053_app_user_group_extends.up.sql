-- Groups nest, the same way roles already do.
--
-- A role and a group are one mechanism wearing two names: a named set that
-- confers something through membership. The only thing separating them was an
-- accident of what got built. app_role_extends exists, so a merchant can say
-- "owner extends admin". Nothing equivalent existed for groups, so "enterprise
-- customers are also beta customers" could only be written by adding every
-- member to both, and keeping them in step by hand for ever.
--
-- Reachability only ever adds, so a diamond resolves the same way whatever
-- order it is walked, which is why multiple parents are allowed and why no
-- ordering field appears here.

CREATE TABLE IF NOT EXISTS app_user_group_extends (
    group_id  TEXT NOT NULL REFERENCES app_user_groups (id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL REFERENCES app_user_groups (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, parent_id),
    -- A group extending itself would be inert rather than wrong, but it reads
    -- as though it says something. Depth beyond this is caught by ExpandSets;
    -- this catches the case a constraint can.
    CHECK (group_id <> parent_id)
);

CREATE INDEX IF NOT EXISTS app_user_group_extends_parent_idx
    ON app_user_group_extends (parent_id);

-- Membership is expanded at write time and stored, for the same reason a role's
-- effective_capabilities is: a merchant's backend reads the assembled answer in
-- its own process, without this graph, so the flattening has to have happened
-- before it asks.
--
-- Direct membership stays in app_user_group_members untouched. This column is
-- derived from it and the table above, so the source of truth is still one
-- place and a bad expansion can be recomputed rather than reconciled.
ALTER TABLE app_user_groups
    ADD COLUMN IF NOT EXISTS effective_parent_ids TEXT[] NOT NULL DEFAULT '{}';

-- Seed the identity case: before any nesting exists, a group's effective set is
-- itself. Without this every existing group would read as reaching nothing
-- until the next write touched it.
UPDATE app_user_groups SET effective_parent_ids = ARRAY[id]
WHERE cardinality(effective_parent_ids) = 0;
