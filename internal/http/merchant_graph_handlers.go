package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

// The merchant graph: edges in, decisions out.
//
// These are the routes that make KYC the evaluator rather than the exporter. A
// merchant writes what they own and asks whether someone may act on it, instead
// of caching a description of authority and interpreting it themselves.

type edgeBody struct {
	ObjectType      string `json:"object_type"`
	ObjectID        string `json:"object_id"`
	Relation        string `json:"relation"`
	SubjectType     string `json:"subject_type"`
	SubjectID       string `json:"subject_id"`
	SubjectRelation string `json:"subject_relation"`
	ExpiresAt       string `json:"expires_at"`
}

func (b edgeBody) toEdge() (service.MerchantEdge, error) {
	e := service.MerchantEdge{
		ObjectType:      b.ObjectType,
		ObjectID:        b.ObjectID,
		Relation:        b.Relation,
		SubjectType:     b.SubjectType,
		SubjectID:       b.SubjectID,
		SubjectRelation: b.SubjectRelation,
	}
	if b.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, b.ExpiresAt)
		if err != nil {
			return service.MerchantEdge{}, apperr.Validation("expires_at must be RFC3339")
		}
		e.ExpiresAt = &t
	}
	return e, nil
}

// handleWriteMerchantEdges records facts about a merchant's own resources.
//
// A batch rather than one at a time, because this runs in their write path: a
// resource created with an owner and a parent is two edges, and two round trips
// per create is a cost they would feel.
func (s *Server) handleWriteMerchantEdges(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Edges []edgeBody `json:"edges"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	edges := make([]service.MerchantEdge, 0, len(body.Edges))
	for _, raw := range body.Edges {
		e, err := raw.toEdge()
		if err != nil {
			writeError(w, err)
			return
		}
		edges = append(edges, e)
	}
	written, err := s.svc.WriteMerchantEdges(r.Context(), orgID, edges)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": written})
}

func (s *Server) handleDeleteMerchantEdge(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body edgeBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	e, err := body.toEdge()
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteMerchantEdge(r.Context(), orgID, e); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCheckMerchant is the call that replaces an authorisation layer.
func (s *Server) handleCheckMerchant(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		SubjectType  string `json:"subject_type"`
		SubjectID    string `json:"subject_id"`
		Action       string `json:"action"`
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	// app_user is the overwhelmingly common subject, so it is the default
	// rather than something every caller repeats.
	if body.SubjectType == "" {
		body.SubjectType = "app_user"
	}
	d, err := s.svc.CheckMerchant(r.Context(), orgID,
		body.SubjectType, body.SubjectID, body.Action, body.ResourceType, body.ResourceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// handleListMerchantObjects answers "what can this customer see?", which is
// every listing page in a merchant's product. The alternative is a check per
// row, and a page of ten thousand cannot be rendered that way at all.
func (s *Server) handleListMerchantObjects(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		SubjectType  string `json:"subject_type"`
		SubjectID    string `json:"subject_id"`
		Action       string `json:"action"`
		ResourceType string `json:"resource_type"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if body.SubjectType == "" {
		body.SubjectType = "app_user"
	}
	out, err := s.svc.ListMerchantObjects(r.Context(), orgID,
		body.SubjectType, body.SubjectID, body.Action, body.ResourceType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleListMerchantSubjects answers "who can see this?": the share dialog, and
// the question an audit asks.
func (s *Server) handleListMerchantSubjects(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Action       string `json:"action"`
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	out, err := s.svc.ListMerchantSubjects(r.Context(), orgID,
		body.Action, body.ResourceType, body.ResourceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMerchantSchema returns the schema a merchant's own vocabulary resolves
// as, derived from their rows rather than stored. It is what the customer
// access map draws, and what makes the model inspectable rather than implied.
func (s *Server) handleMerchantSchema(w http.ResponseWriter, r *http.Request) {
	schema, err := s.svc.MerchantSchema(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, schema.Graph())
}

// handleMerchantInstances returns what exists in a merchant's namespace, per
// type, capped.
//
// A separate route from the schema on purpose. Schema.Graph() lives in
// core/reach and draws a schema by design, and folding rows into it would make
// the portable renderer depend on one application's tables. The map fetches
// both and overlays them, which keeps the boundary where diagram.go put it.
func (s *Server) handleMerchantInstances(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.MerchantInstances(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
