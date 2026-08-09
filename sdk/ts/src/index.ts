/**
 * Transport layer for the KYC merchant Integration API.
 *
 * Everything here is derived from web/public/openapi-integration.yaml. Do not
 * hand-edit the generated module; run `make sdk` at the repo root instead.
 *
 * This package is deliberately thin. It exposes a typed fetch client and the
 * generated types, and nothing else. The ergonomic facade (organisation bound
 * at construction, cached entitlement checks) is a separate layer.
 */
import createOpenapiClient, { type ClientOptions } from "openapi-fetch";

import type { components, operations, paths } from "./generated/schema.js";

export type { components, operations, paths };

/** Response bodies. */
export type AppUser = components["schemas"]["AppUser"];
export type AttributeDefinition = components["schemas"]["AttributeDefinition"];
export type Organisation = components["schemas"]["Organisation"];
export type ProductFeature = components["schemas"]["ProductFeature"];
export type ProductPlan = components["schemas"]["ProductPlan"];
export type ProductPlanPrice = components["schemas"]["ProductPlanPrice"];
export type Subscription = components["schemas"]["Subscription"];
export type OrgEntitlements = components["schemas"]["OrgEntitlements"];
export type ErrorResponse = components["schemas"]["ErrorResponse"];

/** Request bodies. */
export type CheckEntitlementRequest = components["schemas"]["CheckEntitlementRequest"];
export type CreateAppUserRequest = components["schemas"]["CreateAppUserRequest"];
export type IngestAppUserRequest = components["schemas"]["IngestAppUserRequest"];
export type UpdateAppUserRequest = components["schemas"]["UpdateAppUserRequest"];
export type CreateAttributeDefinitionRequest =
  components["schemas"]["CreateAttributeDefinitionRequest"];
export type UpdateAttributeDefinitionRequest =
  components["schemas"]["UpdateAttributeDefinitionRequest"];
export type CreateProductFeatureRequest = components["schemas"]["CreateProductFeatureRequest"];
export type UpdateProductFeatureRequest = components["schemas"]["UpdateProductFeatureRequest"];
export type SetProductFeatureOverridesRequest =
  components["schemas"]["SetProductFeatureOverridesRequest"];
export type CreateProductPlanRequest = components["schemas"]["CreateProductPlanRequest"];
export type UpdateProductPlanRequest = components["schemas"]["UpdateProductPlanRequest"];
export type SetProductPlanFeaturesRequest = components["schemas"]["SetProductPlanFeaturesRequest"];
export type SetActiveProductPlanRequest = components["schemas"]["SetActiveProductPlanRequest"];
export type UpsertProductPlanPriceRequest =
  components["schemas"]["UpsertProductPlanPriceRequest"];
export type InboundWebhookPayload = components["schemas"]["InboundWebhookPayload"];

export interface KycClientOptions extends Omit<ClientOptions, "headers"> {
  /** Absolute origin of the KYC API, e.g. https://kyc.example.com */
  baseUrl: string;
  /** Organisation API key (kyc_…). Omit to set Authorization yourself. */
  apiKey?: string;
  headers?: Record<string, string>;
}

/**
 * Build a typed fetch client. Paths and payloads are checked against the spec.
 *
 *   const client = createClient({ baseUrl, apiKey })
 *   const { data, error } = await client.POST("/v1/entitlements/check", {
 *     body: { organisation_id: orgId, entitlement: "premium_export" },
 *   })
 */
export function createClient({ apiKey, headers, ...options }: KycClientOptions) {
  return createOpenapiClient<paths>({
    ...options,
    headers: {
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}),
      ...headers,
    },
  });
}

export default createClient;
