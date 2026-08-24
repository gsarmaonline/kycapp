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
