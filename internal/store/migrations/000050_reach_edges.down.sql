-- The projection is derived, so dropping the table loses nothing that cannot be
-- rebuilt by running the up migration again.
DROP INDEX IF EXISTS reach_edges_expiry_idx;
DROP INDEX IF EXISTS reach_edges_subject_idx;
DROP TABLE IF EXISTS reach_edges;
