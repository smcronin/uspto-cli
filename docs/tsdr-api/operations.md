# Authentication, limits, errors, and live behavior

Verified: **2026-08-06**

## Authentication

TSDR uses its own API gateway and credential. Obtain a key at the
[USPTO API Manager](https://account.uspto.gov/api-manager/) and send this exact
header on requests to `tsdrapi.uspto.gov`:

```http
USPTO-API-KEY: <TSDR key>
```

The official user guide says the header name must be entered exactly. The key
is not interchangeable with:

- ODP's `X-API-KEY` credential from the Open Data Portal;
- credentials for PatentsView, EPO OPS, or other USPTO/non-USPTO systems;
- browser cookies from `tsdr.uspto.gov` or `tmsearch.uspto.gov`.

For this CLI, prefer `USPTO_TSDR_API_KEY`; the legacy `TSDR_API_KEY` name may be
accepted for compatibility. Do not commit a key, place it in command history,
log it, include it in a URL, or echo it during diagnostics.

```bash
curl --fail-with-body \
  -H "USPTO-API-KEY: $USPTO_TSDR_API_KEY" \
  -H "Accept: application/xml" \
  "https://tsdrapi.uspto.gov/ts/cd/casestatus/sn78787878/info.xml"
```

Only attach the header to the exact configured TSDR origin. If a response
redirects to `tmng-al.uspto.gov`, `tsdr.uspto.gov`, or any other host, strip the
credential before following it.

## Published rate limits

The official guide says limits are subject to system availability and usage.
Times are stated by USPTO as EST; operational clients should conservatively
apply the peak tier whenever timezone/daylight interpretation is uncertain.

| Request class | Peak (5 a.m.–10 p.m. Eastern) | Off-peak (10 p.m.–5 a.m. Eastern, all days) |
| --- | ---: | ---: |
| All requests | 60/key/minute | 120/key/minute |
| PDF and ZIP downloads | 4/key/minute | 12/key/minute |
| Multi-case PDF and ZIP downloads | 4/key/minute | 12/key/minute |

The narrow PDF/ZIP tier is nested inside the all-request tier. A safe peak
client leaves at least 1 second between metadata requests and 15 seconds
between PDF/ZIP requests. Coordinate across processes because the quota is per
key. Do not increase concurrency merely because individual calls are slow.

## Status and error handling

| Status / failure | Interpretation | Agent behavior |
| --- | --- | --- |
| `200` | Successful response | Validate content type/body before parsing or saving |
| `204` | No public content / unregistered or empty result | Return a typed empty result; do not parse |
| `400` | Invalid identifier, selection, or parameter format | Fix request; do not retry unchanged |
| `401` | Missing/invalid TSDR authentication | Explain separate key requirement and API Manager URL |
| `403` | Credential/account rejected **or** route/method blocked by the gateway | Inspect structured `error.type`/hint; verify the key with `api-spec` or a known GET before rotating it, and prefer a working GET alias for an advertised POST |
| `404` | Case, document, asset, or route not found | Verify namespace, formatting, and alias |
| `406` | `Accept`/representation not supported | Use documented explicit suffix and matching `Accept` |
| `429` | Per-key rate limit exceeded | Honor `Retry-After`; otherwise bounded exponential backoff with jitter |
| `5xx` | Transient gateway/service failure | Retry idempotent retrievals with a cap; preserve checkpoint |
| reset/timeout | Gateway or upstream interruption | Retry conservatively; distinguish from “not found” |

All six Swagger POST operations are retrievals. The current contract models
their selectors as path/query parameters rather than bodies. If a future alias
uses a body, retry only when it can be replayed exactly. Bound retries and never
fan out a failing batch into an uncontrolled request storm.

Recommended retry envelope:

1. No retry for `400`, `401`, `403`, or ordinary `404`.
2. For `429`, honor `Retry-After` as a minimum. The CLI auto-retries only when
   that delay is at most 30 seconds; for a longer provider window it returns
   the typed rate-limit error immediately so the caller can resume after the
   full delay. JSON/NDJSON errors expose the delay as `retryAfterSeconds`.
   Apply the stricter artifact limiter before any retry.
3. For `500`, `502`, `503`, `504`, connection reset, or timeout, use exponential
   backoff with jitter, normally 3–5 attempts.
4. Checkpoint successful IDs/files so a resumed job does not redownload them.
5. Surface final failures alongside requested identifiers and safe response
   metadata.

## Content negotiation and aliases

The live Swagger models several routes as content-negotiated, while the user
guide publishes suffix-specific URLs. In practice:

- prefer `/info.xml`, `/info.json`, `/download.pdf`, and `/content.zip` when a
  public alias exists;
- set `Accept` to the same intended representation;
- tolerate XML responses with namespaces and no JSON equivalent;
- treat `406` as a representation mismatch, not as an auth error;
- inspect `Content-Type` rather than trusting old Swagger descriptions (some
  historical descriptions say PDF for ZIP routes).

## Live observations on 2026-08-06

These smoke tests used public sample identifiers from USPTO documentation:

| Request | Observation |
| --- | --- |
| `/casestatus/sn78787878/info.xml` | `200 application/xml`, approximately 29 KB; namespace-heavy status record |
| `/rawImage/78787878` | `200`, PNG image |
| `/casedocs/bundle.xml?sn=72131351` | `200 application/xml`, approximately 29 KB; `DocumentList` metadata |
| `/last-update/info.xml?sn=78787878` | Connection reset during the verification window; do not equate this with a missing case |
| `/ts/swagger.json` without auth | `401`; the specification itself requires a TSDR key |

The old `developer.uspto.gov` Swagger/catalog links redirected away from their
historical content. The authenticated live Swagger contained 30 paths and 36
operations, whereas public archived copies contained only 24–25 paths.

## Logging and file safety

- Redact `USPTO-API-KEY`, URL query values that may identify private work
  lists, and personal data extracted from correspondence.
- Log request method, route template, status, duration, rate-limit decision,
  requested/returned counts, content type, and response size.
- Never log raw headers or complete office-action/correspondence bodies by
  default.
- Write downloads atomically, use sanitized filenames, and keep a checksum.
  The CLI computes SHA-256 during streaming and emits it as `sha256` in every
  download result and case-bundle manifest entry.
- A complete trademark record can contain public addresses and other personal
  information. Public availability does not make indiscriminate redistribution
  appropriate.

For API help, the official catalog and bulk-data page direct users to
`APIhelp@uspto.gov`; TSDR record/document questions are also routed through the
TSDR support channels listed in the [FAQ](https://tsdr.uspto.gov/faqview).
