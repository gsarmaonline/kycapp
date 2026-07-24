// Package resources is a generic catalog of domain resources and the
// automation trigger IDs derived from them.
//
// Trigger ID shapes:
//
//	{resource}.{lifecycle}           e.g. app_user.created
//	{resource}.attribute.{key}       e.g. app_user.attribute.country
//
// Attribute triggers are expanded from org-scoped keys (or any caller-supplied
// key list). Register new resources here; automations and enqueue sites consume
// the same helpers so the editor and runtime stay aligned.
package resources
