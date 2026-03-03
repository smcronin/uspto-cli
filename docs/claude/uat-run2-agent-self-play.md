# USPTO CLI UAT Run 2 - Agent Self-Play Findings

Date: 2026-03-01  
Tester mode: Black-box first-time agent user (CLI only; no implementation inspection)

## Scope

This run pressure-tested real USPTO API workflows across:

- `search` (GET and POST-style queries)
- `app` (`get`, `meta`, `docs`, `download`, `download-all`, `transactions`, `assignments`, `continuity`, `attorney`, `associated-docs`, `adjustment`, `foreign-priority`, XML extraction commands)
- `summary`
- `family`
- `status`
- `ptab` (trials, decisions, documents, appeals, interferences)
- `petition` (`search`, `get`)
- `bulk` (`search`, `get`, `files`, `download`)

Representative IDs used:

- Apps: `14643719`, `19378371`, `30032256`
- Patent: `10000000`
- PTAB trial/doc/decision: `IPR2026-00276`, `171300186`, `171299842`
- PTAB appeal/doc: `2026000539`, `650ad83e-56f0-4d7d-843a-5f238d16951f`
- PTAB interference number/doc: `106130`, `bbc1cc8275574c3d41d022f2e2040550ccad434ae6fbf07294895a04`
- Petition record: `0d5f5afa-d456-52b4-81e2-d4e51d7c801b`
- Bulk product/file: `TTABTDXF`, `tt260227.zip`

## Critical Bugs (Fix First)

1. `--dry-run` is not reliably honored across commands.
- Severity: High
- Repro:
  - `uspto app get 14643719 --dry-run`
  - `uspto ptab decisions --trial IPR2024-01362 --dry-run`
- Actual: Executes live requests and returns full data tables instead of only request preview.
- Expected: Never execute network call; print method/endpoint/body only.

2. `search --facets` path fails with response decode error.
- Severity: High
- Repro:
  - `uspto search --filter "applicationTypeLabelName=Utility" --facets "applicationTypeCategory" -f json --minify --quiet`
- Actual: `json: cannot unmarshal object into Go struct field ... facets of type []types.FacetValue`
- Expected: Parse returned facets shape correctly (or handle both object/array variants).

3. `petition search --decision` returns 404 for valid documented values.
- Severity: High
- Repro:
  - `uspto petition search --decision DENIED --limit 2 -f json --minify --quiet`
- Actual: `404 NOT_FOUND`
- Expected: Filtered results for `DENIED`/`GRANTED`/`DISMISSED` as help text promises.

## Major Bugs

1. `--quiet` does not suppress non-data output in `bulk download`.
- Repro:
  - `uspto bulk download TTABTDXF tt260227.zip -o downloads/run2_ttabtdxf_tt260227.zip -q`
- Actual: prints table summary.
- Expected: no non-data output when `-q` is set.

2. Invalid date input lacks local validation and surfaces generic server-style error.
- Repro:
  - `uspto app docs 14643719 --from 2026-99-99 -f json --minify --quiet`
- Actual: `500 SERVER_ERROR` with generic message.
- Expected: local validation error (`YYYY-MM-DD` invalid month/day) before request.

3. Conflicting filters are accepted silently.
- Repro:
  - `uspto search --pending --granted --limit 1 -f json --minify --quiet`
- Actual: succeeds, returns records; semantics are ambiguous.
- Expected: explicit conflict error or documented precedence.

4. Timeout semantics are unclear/inconsistent at low values.
- Repro:
  - `uspto app fulltext 14643719 --timeout 1 -f json --minify --quiet`
- Actual: succeeds after roughly >1s wall-clock.
- Expected: documented behavior (per HTTP request vs full command) and strict handling.

## Medium Bugs / UX Gaps

1. Some valid queries return generic 400 without actionable hinting.
- Repro:
  - `uspto search --title "artificial intelligence" --inventor "Rodriguez" --sort "filingDate:desc" --limit 2 -f json --minify --quiet`
- Actual: `400 GENERAL_ERROR Bad Request`
- Expected: include likely cause and valid field/value alternatives.

2. PTAB interferences discovery path often has `decisionDocumentData: null`.
- Repro:
  - `uspto ptab interferences --limit 50 -f json --minify --quiet`
  - `uspto ptab interferences-for 106130 -f json --minify --quiet`
- Actual: document IDs are commonly unavailable from listing paths.
- Expected: easier path to obtain doc IDs required by `ptab interference <documentId>`.

3. Output shape inconsistency (`results` array vs object) across subcommands.
- Example:
  - `ptab get` => object in `results`
  - many other commands => array in `results`
- Impact: extra conditional parsing complexity for agents.

## Feature Requests (Agent-Centric)

1. Add `--explain-error` with typed hints.
- Include: valid values, format examples, field names, and likely fix suggestions.

2. Add `--trace` machine-readable diagnostics.
- Include method, URL path, status, request id, latency, retryability in JSON.

3. Add ID-chaining helpers.
- Example: `ptab chain --trial IPR2026-00276` returns proceeding + docs + decisions IDs in one normalized object.

4. Add output projection to all commands.
- Example: `--select trialNumber,documentData.documentIdentifier` for concise agent payloads.

5. Add deterministic pagination envelope everywhere.
- Standardize `pagination` across all list/search endpoints.

## Improvements

1. Improve default table behavior for wide objects.
- Auto-truncate large text fields and expose `--expand` when needed.

2. Enforce local input validation uniformly.
- Dates, UUIDs, trial numbers, and enum values should fail fast client-side.

3. Normalize schema naming across endpoints.
- Prefer stable canonical keys over source-specific naming drift.

4. Improve command discoverability for first-time agents.
- Add `uspto examples` with copy/paste workflows by task.

5. Strengthen dry-run contract.
- A strict guarantee: zero network calls in dry-run mode.

## Endpoint Coverage Snapshot

Covered with live execution:

- `search`: yes (including POST-like filter/facets path, though facets path currently errors)
- `app` data endpoints: yes
- `app` downloads: yes (single + filtered bulk app docs)
- `app` XML extractors: yes (`abstract`, `claims`, `citations`, `description`, `fulltext`)
- `summary`: yes
- `family`: yes
- `status`: yes
- `ptab` trials/docs/decisions: yes
- `ptab` appeals: yes
- `ptab` interferences (number and doc-id endpoint): yes
- `petition` search/get: yes (decision-filter bug found)
- `bulk` search/get/files/download: yes

## Bottom Line

The CLI already enables broad end-to-end USPTO workflows for agents, including real document download and deep application/PTAB traversal. The biggest blockers for reliable autonomous use are: dry-run contract violations, schema mismatch on search facets, and weak recovery hints on common failures.


