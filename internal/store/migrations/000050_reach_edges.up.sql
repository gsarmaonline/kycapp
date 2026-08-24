-- The edge table for the relation-graph authorisation engine.
--
-- See docs/access-by-reachability.md. Nothing reads this table on a request
-- path yet: it exists so the two engines can be run side by side on the same
-- requests and their disagreements logged. The current tables stay
-- authoritative until that comparison is clean.
--
-- One table holds the entire data model. There is no capability table, no
-- scope table and no grant table, because a group, a role, a container, a tag
-- and an action are all nodes, and every fact about them is an edge.

CREATE TABLE IF NOT EXISTS reach_edges (
    -- The tenancy boundary. KYC's own model is 'kyc'; a merchant's will be
    -- 'org:<id>', which is what keeps their open vocabulary out of this one.
    namespace        TEXT NOT NULL,

    -- The object end. object_id may be '*', the star node standing for every
    -- object of that type, present and future. That is how global reach is
    -- written: one row rather than one per tenant.
    object_type      TEXT NOT NULL,
    object_id        TEXT NOT NULL,

    relation         TEXT NOT NULL,

    -- The subject end. A non-empty subject_relation makes this a userset:
    -- role:acme_ops#holder means "whoever holds that role", resolved by the
    -- walk rather than enumerated here.
    subject_type     TEXT NOT NULL,
    subject_id       TEXT NOT NULL,
    subject_relation TEXT NOT NULL DEFAULT '',

    -- Expiry lives on the edge, which is what makes time-boxed access the
    -- cheap option rather than the diligent one. No job has to run on time.
    expires_at       TIMESTAMPTZ,

    -- Where this row came from. Evaluation ignores it; audit and the
    -- projection's own verification read it.
    source           TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation),

    -- Reach over every type at once is the environment-derived root of trust.
    -- It stays outside the data, where it can be counted.
    CONSTRAINT reach_edges_no_star_type CHECK (object_type <> '*' AND subject_type <> '*'),
    -- A userset on a star node would name nobody in particular and fan the walk
    -- out over a whole type for no expressible reason.
    CONSTRAINT reach_edges_no_star_userset CHECK (NOT (subject_id = '*' AND subject_relation <> ''))
);

-- The resolver's only query is an exact prefix of the primary key, so it needs
-- no index of its own.

-- The reverse direction: every edge naming one subject. This is what a sweep
-- uses when a person leaves, and what "who can reach this?" will build on.
CREATE INDEX IF NOT EXISTS reach_edges_subject_idx
    ON reach_edges (namespace, subject_type, subject_id);

CREATE INDEX IF NOT EXISTS reach_edges_expiry_idx
    ON reach_edges (expires_at) WHERE expires_at IS NOT NULL;

-- The projection of the current model into this table is deliberately not here.
-- A data migration buried in a schema change cannot be re-run and cannot be
-- tested, and this one has to be both: it runs again after the cutover, and its
-- correctness is the whole basis for trusting the new engine. It lives in
-- internal/accessmodel/projection.sql, applied by accessmodel.Project.
