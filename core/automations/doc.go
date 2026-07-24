// Package automations owns the merchant rule DSL (conditions, actions) and
// catalogs. Trigger IDs come from core/resources; action types implement
// ActionHandler (catalog + validate) with executors registered in internal/service.
// Persistence and HTTP live in internal/store, internal/service, and internal/http.
package automations
