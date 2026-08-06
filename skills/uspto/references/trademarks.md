# Trademark Search and TSDR

Use this reference whenever the user asks about a trademark, service mark,
wordmark, owner, goods/services identification, international class, design
code, trademark prosecution history, registration maintenance, or TSDR file.

## Choose the correct service

| Need | CLI surface | Credential |
| --- | --- | --- |
| Discover marks by wording, owner, goods/services, class, attorney, design code, date, or live/dead status | `uspto trademark search` | None |
| Retrieve an identified official record, prosecution history, document, mark image, maintenance data, or update time | `uspto trademark ...` other than `search` | Separate TSDR key |
| Analyze the whole trademark corpus | `uspto bulk ...` and a local index | ODP key/product access |

Trademark Search is the public backend used by the official USPTO web UI. It
is keyless but is not published as a stable developer API. The CLI discovers
its current versioned URL dynamically. Treat its results as candidates, label
the source, and hydrate important cases through TSDR.

TSDR is identifier-driven retrieval at `tsdrapi.uspto.gov`; it is not a
wordmark/owner/class search API. It sends the exact header `USPTO-API-KEY`.
Never send the patent ODP key (`X-API-KEY`) to TSDR.

## TSDR key setup

Get the separate key from https://account.uspto.gov/api-manager/.

```bash
uspto config set-tsdr-api-key your-tsdr-key
uspto config set-tsdr-api-key --from-env
uspto config set-tsdr-api-key --from-dotenv .env
uspto config show
```

Canonical environment variable: `USPTO_TSDR_API_KEY`. The compatibility alias
`TSDR_API_KEY` is also accepted. `--tsdr-api-key` is available for a one-off
call, but avoid putting secrets in shell history.

When a TSDR command has no key, the CLI exits 3 and explains where to get one.
Search must remain usable without a key. If a TSDR request returns 404 for a
known case, verify the credential because the gateway can use 404 for an
invalid key:

```bash
uspto trademark api-spec -f json -q
```

## Identifier rules

Prefer explicit prefixes, especially when numbers are ambiguous.

| Type | Form | Example |
| --- | --- | --- |
| Serial | `sn:` plus exactly 8 digits | `sn:97054561` |
| US registration | `rn:` plus 5–7 nonzero digits | `rn:3500038` |
| International registration | `ir:` plus 6–10 digits or trailing letter form | `ir:1234567` |
| US reference | `ref:` plus one letter and 7 digits | `ref:Z1231384` |
| Expungement/reexamination proceeding | `pn:` plus 10 digits and optional E/R | `pn:2022100137E` |

Bare 8-digit values default to serial; bare 5-digit values default to US
registration. Bare 6–7 digit values are rejected as ambiguous between US and
international registrations. Add `--id-type` or a prefix. TSDR batch status
supports `sn`, `rn`, `ir`, and `ref`, but not `pn`.

## Programmatic output

Always add `-f json -q` for agent use. Successful JSON includes:

```json
{
  "ok": true,
  "command": "status",
  "commandPath": "uspto trademark case status",
  "provider": "tsdr",
  "results": [],
  "version": "..."
}
```

`provider` is `tmsearch` for search and `tsdr` for keyed retrieval. Full case
JSON preserves a schema-tolerant ST.96 element tree under `results.raw`. It is
not lexically lossless; export XML when namespaces, comments, processing
instructions, or mixed-content order matter. Human table views are curated.

## Search

```bash
# Plain text is a combined-mark query (CM)
uspto trademark search "OPENAI" --status live -f json -q

# Friendly filters combine with AND
uspto trademark search --wordmark APPLE --class 009 --status live -f json -q
uspto trademark search --owner "OpenAI OpCo" --goods software -f json -q
uspto trademark search --attorney Smith --filed-from 2025-01-01 -f json -q

# Official field-tag syntax passes through unchanged
uspto trademark search --query 'CM:"APPLE" AND IC:009 AND LD:true' -f json -q
uspto trademark search --query 'ON:"NIKE" OR ON:"Nike Innovate"' --all --max-results 5000 -f json -q

# Count, pagination, projections, exact sorts, and facets
uspto trademark search --owner NIKE --count-only -f json -q
uspto trademark search --goods battery --limit 100 --offset 200 -f json -q
uspto trademark search APPLE --fields id,wordmark,ownerName,alive -f json -q
uspto trademark search APPLE --sort wordmarkExact:asc,id:asc -f json -q
uspto trademark search --owner NIKE --facets classes=internationalClassExact:25 -f json -q

# Full-power request escape hatch
uspto trademark search --raw-body request.json --raw-response -f json -q
cat request.json | uspto trademark search --raw-body - -f json -q
```

With `--raw-body`, put paging, `_source`, sorts, and aggregations/facets inside
the JSON. Do not combine it with typed `--limit`, `--offset`, `--max-results`,
`--fields`, `--sort`, or `--facets`; the CLI rejects those ambiguous mixes.

Common official tags: `CM` combined mark, `ON` owner, `AT` attorney, `SN`
serial, `RN` registration, `GS` goods/services, `IC` international class,
`CC` coordinated/US class, `DC` design code, `FD` filing date, `RD`
registration date, and `LD` live/dead.

Search response fields commonly include `id` (serial), `wordmark`,
`wordmarkPseudoText`, `ownerName`, `goodsAndServices`, `internationalClass`,
`registrationId`, filing/registration dates, `alive`, mark/drawing type,
design-code descriptions, basis, and attorney.

## Case status and analysis

```bash
uspto trademark case status sn:97054561 -f json -q
uspto trademark case status sn:97054561 rn:3500038 -f json -q
uspto trademark case status --ids-file identifiers.txt -f json -q

# Parsed stable fields plus schema-tolerant ST.96 element tree
uspto trademark case get sn:97054561 -f json -q

# Official raw JSON or XML representations
uspto trademark case get sn:97054561 --representation json -f json -q
uspto trademark case get sn:97054561 --representation xml -f json -q
uspto trademark case get sn:97054561 --representation legacy-xml -f json -q

# Focused views
uspto trademark case goods sn:97054561 -f json -q
uspto trademark case parties sn:97054561 -f json -q
uspto trademark case events sn:97054561 --latest 20 -f json -q
uspto trademark case events sn:97054561 --code NOA --from 2024-01-01 -f json -q
uspto trademark case assignments sn:97054561 -f json -q
uspto trademark case designs sn:97054561 -f json -q
uspto trademark case maintenance rn:3500038 -f json -q
```

Use `trademark batch` for many same-type IDs. The CLI enforces the live server
maximum of 25 per request and chunks larger lists:

```bash
uspto trademark batch serial 97054561 78787878 -f json -q
uspto trademark batch registration --ids-file registrations.txt -f json -q
uspto trademark batch international --ids-file - --allow-dupes -f json -q
uspto trademark batch serial --ids-file serials.txt --from OPAQUE_START --to OPAQUE_END -f json -q
```

Inspect `transactions`, `missedElements`, and `oversized`; do not silently
discard partial failures.

## Documents and downloads

List metadata first. Each result includes a stable case-scoped 1-based `index`,
its preserved per-case TSDR `selectionIndex`, derived `documentId`,
document/category codes, dates, page count, and native page URLs. In a
multi-case list, pair `serialNumber` with `index`; duplicate indices across
different cases are expected. Repeat the same filters/sort for follow-up use.
The default rich list is intentionally used for any index-based workflow. Add
`--fast` only for metadata triage; it omits IDs/URLs and its local indices must
not be reused by `info`, `download`, `page`, or `fetch`.

```bash
uspto trademark docs list sn:72131351 -f json -q
uspto trademark docs list sn:72131351 --type SPE --from 2020-01-01 -f json -q
uspto trademark docs list sn:72131351 --type SPE --fast
uspto trademark docs info sn:72131351 UNC20081028103437 -f json -q

uspto trademark docs download sn:72131351 UNC20081028103437 --asset pdf -o document.pdf
uspto trademark docs download sn:72131351 UNC20081028103437 --asset zip -o originals.zip
uspto trademark docs page sn:72131351 UNC20081028103437 1 -o page-1.bin

# Numeric list indices are accepted. Modern CMS documents without a legacy
# document ID are retrieved from their public USPTO URL without forwarding
# the TSDR key.
uspto trademark docs info sn:97238896 2 --sort date:D -f json -q
uspto trademark docs fetch sn:97238896 1 --sort date:D -o native-document.xml

uspto trademark docs bundle sn:72131351 --asset xml -o metadata.xml
uspto trademark docs bundle sn:72131351 --type SPE --asset pdf -o specimens.pdf
uspto trademark docs bundle --ids-file marks.txt --asset zip -o portfolio.zip

uspto trademark docs selected sn:72131351 --case --docs 1,3 --history 2 --asset pdf -o selected.pdf
uspto trademark docs download-all sn:72131351 --type SPE --asset pdf -o specimens/
```

PDF/ZIP calls are much more limited than metadata; avoid `download-all` until
the metadata proves the files are needed.

`docs selected --docs` accepts indices from the current rich list and maps
them back to preserved server ordinals even after local filtering/sorting.
`--assignments` and `--history` are raw TSDR server selection values; do not
substitute row numbers from a locally filtered view.

## Images, exports, bundles, updates, and low-level routes

```bash
uspto trademark image 97054561 -o mark.png

uspto trademark case export sn:97054561 --asset json -o status.json
uspto trademark case export sn:97054561 --asset xml -o status.xml
uspto trademark case export sn:97054561 --asset html -o status.html
uspto trademark case export sn:97054561 --asset pdf -o status.pdf
uspto trademark case export sn:97054561 --asset status-zip -o status.zip

# Local research bundle: parsed/raw status, XML, docs metadata, image, manifest
uspto trademark case bundle sn:97054561 -o ./case/
# Add rate-limited PDFs/ZIPs only when genuinely needed
uspto trademark case bundle sn:97054561 -o ./case/ --include-heavy

uspto trademark last-update sn:97054561 --response-format json -f json -q
uspto trademark multimedia info sn:12345678 -f json -q
```

The safe retrieval escape hatch reaches all live Swagger GET/POST operations
and future aliases. It rejects absolute/protocol-relative URLs and strips credentials
on cross-origin redirects.

```bash
uspto trademark request /ts/cd/maintenance/rn3500038/info.json -f json -q
uspto trademark request /ts/cd/casedocs/sn72131351/mega-bundle --method POST --param case=false --param docs=4 --param assignments= --param prosecutionHistory= --output selected.pdf --expected pdf --dry-run
uspto trademark casemap OPAQUE_TSDR_IDTOKEN -f json -q
uspto trademark request /ts/cd/example/file.pdf --output file.pdf --expected pdf
uspto trademark api-spec -o tsdr-swagger.json
```

Use repeated `--param key=value`; repeated and explicitly empty values are
preserved. Current Swagger POST variants use query parameters. `--body` and
`--form` are bounded, replayable forward-compatibility options. Live selected-
bundle POST returned gateway 403 with a valid GET-capable key, so prefer GET
aliases for real work and treat POST 403 as possible method/gateway behavior,
not automatically a bad key. For opaque/binary responses, always provide
`--output`; add `--expected pdf|zip|png|json|xml|html` to validate magic bytes.
Swagger also exposes `trademark casemap <opaque-idtoken>` for tokens produced
by internal TSDR workflows. It is experimental and is not an identifier mapper;
pass `sn:`, `rn:`, `ir:`, `ref:`, and `pn:` identifiers directly to case
commands.

## Rate limits and recovery

Published peak limits: 60 total requests/key/minute, but only 4 PDF/ZIP and 4
multi-case PDF/ZIP requests/key/minute. Published off-peak (10 p.m.–5 a.m.
Eastern): 120 total and 12 PDF/ZIP requests/key/minute. The CLI conservatively
coordinates 1-second metadata and 15-second binary gaps across local processes.
It honors `Retry-After` as a minimum: waits of at most 30 seconds are retried
automatically, while longer windows return immediately for caller scheduling;
structured errors expose the minimum as `error.retryAfterSeconds`. It also
retries transient 500/502/503/504 gateway failures.

- Exit 3 + `AUTH_FAILURE`: missing/rejected TSDR key. Never substitute the ODP key.
- Exit 3 + `ACCESS_FORBIDDEN`: verify with `api-spec`/known GET first; a valid
  key can still encounter a gateway-blocked Swagger POST, so prefer its GET alias.
- Exit 4: verify ID prefix and case existence; then verify key via `api-spec`.
- Exit 5: automatic retries exhausted; wait before resuming.
- Exit 6: transient USPTO backend problem; retry/checkpoint rather than fan out.
- `204`: valid request with no applicable content, common for maintenance.
- Multimedia is documented but has produced intermittent live 500 errors.
- The older `/last-update` aliases can reset connections; the CLI uses the
  current `/ts/cd/caseupdate` route.
- New documents may not appear in TSDR the same day they are filed.

Use `--dry-run` on every friendly command to inspect paths/parameters without
calling USPTO. Downloads use temporary files, validate content when a type is
known, emit a streaming SHA-256 in `sha256`, and require `--overwrite` before
replacing an existing artifact. Binary raw requests require `--output` before
any network call.
