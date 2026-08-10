# TODO

## Larger requests
1. Observability
2. Robust automations
    - More triggers
    - Richer rules
    - Email visual builder
3. Usage metering
4. Compliance
5. Customer support
6. Org settings and branding


## Integration with merchants
1. SDK
2. Docs/Journeys
3. Reading cusotmer DB via streaming/logs/wal

## Access control follow-ups

Both are consequences of decisions already made, not open questions. Design is
settled in [authentication.md](authentication.md) and [authorisation.md](authorisation.md).

1. **API key transfer.** A key's capabilities come from its owner, so revoking
   someone's membership stops their keys. That is intended, but there is no way
   to move a key to another owner first, which means offboarding can break a
   merchant's production integration. Needs a transfer endpoint and a view of
   keys whose owner has lost their membership.
2. **Time-boxed staff access.** `memberships.expires_at` is honoured by grant
   assembly, but nothing issues a short-lived membership, so just-in-time access
   is possible rather than the default. Needs an issuing API and UI. Without
   one-click issuing, staff will ask for standing access instead, which defeats
   the point.

## Tech debt/improvements
- Revisit billing


