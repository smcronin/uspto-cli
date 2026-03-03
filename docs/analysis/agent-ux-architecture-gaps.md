# USPTO CLI: Agent UX & Architecture Gap Analysis

Comprehensive analysis of the USPTO CLI from an AI agent's perspective, covering output optimization, pagination, error handling, composability, missing commands, query building, performance, and output formats.

---

## 1. Agent Output Optimization

### Current State

The `-f json` flag dumps the raw API response verbatim via `JSON.stringify(data, null, 2)`. There is no standardized envelope, no metadata about the request itself, and no field selection at the output layer.

The `--fields` flag exists on `search` but it controls which fields the *API* returns, not which fields the CLI extracts from the response. There is no `--compact` flag despite being listed in the help text (`"Output format: table, json, compact"`). The `compact` format falls through to the default case in `formatOutput()` and produces identical output to `json`.

### Problems for Agents

1. **No response envelope.** When an agent calls `uspto search --title sensor -f json`, it gets back the raw API shape `{ count, patentFileWrapperDataBag }`. But when it calls `uspto ptab search -f json`, the shape is `{ count, patentTrialProceedingDataBag }`. Every command returns a differently-keyed top-level array. An agent must know the exact key name for each endpoint.

2. **No pagination metadata in JSON output.** The `count` is present, but there is no indication of the current `offset`, `limit`, or whether there are more pages. An agent cannot determine if it needs to paginate without comparing `count` against `offset + limit` using values it must track externally.

3. **Deeply nested data requires deep JSON path knowledge.** To get an invention title from search results, the agent must traverse `patentFileWrapperDataBag[n].applicationMetaData.inventionTitle`. This is a 3-level deep path that must be memorized per endpoint.

4. **`compact` format is dead code.** It is listed as an option but does nothing different from `json`.

### Recommendations (Priority: HIGH)

**R1.1 - Standardized JSON envelope.** Wrap all JSON output in a consistent structure:

```json
{
  "ok": true,
  "command": "search",
  "query": "applicationMetaData.inventionTitle:sensor",
  "pagination": {
    "offset": 0,
    "limit": 25,
    "total": 1432,
    "hasMore": true
  },
  "resultCount": 25,
  "results": [ ... ],
  "requestId": "abc-123"
}
```

This gives every command the same top-level shape. `results` always contains the array regardless of whether the API calls it `patentFileWrapperDataBag` or `petitionDecisionDataBag`. The `pagination` block lets agents auto-paginate without arithmetic.

**R1.2 - Implement `--compact` for real.** Make it output one JSON object per line (NDJSON), or a flattened summary per result. This is significantly easier for agents to parse line-by-line.

**R1.3 - Add `--pick <fields>` for output-layer field selection.** Distinct from `--fields` (which controls the API request), `--pick` would extract only the specified dot-paths from each result:

```bash
uspto search --title sensor -f json --pick applicationNumberText,applicationMetaData.inventionTitle,applicationMetaData.patentNumber
```

This would produce:

```json
[
  {"applicationNumberText": "18045436", "inventionTitle": "...", "patentNumber": "12000000"},
  ...
]
```

This eliminates the need for `jq` in agent pipelines and reduces token consumption when an LLM is parsing the output.

**R1.4 - Add `--quiet` / `-q` flag.** Suppress human-readable decorative output (`\n1432 results found\n`) that pollutes machine-parseable output. Currently, table format mixes count text with the table, which is hard to parse.

---

## 2. Pagination

### Current State

The CLI exposes `--limit` (default 25) and `--offset` (default 0). The API has a hard ceiling of 10,000 results. There is no `--all` flag or auto-pagination mechanism. The `count` field in the response tells the total, but the CLI does not surface offset or limit in its output.

The rate limiter enforces sequential requests with a 100ms minimum gap, which is correct given the burst limit of 1.

### Problems for Agents

1. **Manual pagination loop required.** An agent must run `search --offset 0`, parse `count`, then run `search --offset 25`, `--offset 50`, etc. This is error-prone and verbose.

2. **No signal for "last page."** The agent must compute `offset + limit >= count` itself, but `offset` and `limit` are not in the output.

3. **10,000 result ceiling is not communicated.** If `count` is 15,000, the agent may try to paginate beyond 10,000 and get errors or empty results with no warning.

4. **No cursor-based pagination.** The API uses offset/limit, which is fine, but the CLI could abstract it.

### Recommendations (Priority: HIGH)

**R2.1 - Add `--all` flag for auto-pagination.** Automatically fetches all pages up to the 10,000 ceiling:

```bash
uspto search --title sensor --all -f json
```

Internally, this would loop with increasing offsets, respecting the rate limiter, concatenating results, and streaming them to stdout. Should print progress to stderr: `[page 2/40, 50/1000 results...]`.

**R2.2 - Add `--page <n>` as sugar.** `--page 3` with default `--limit 25` translates to `--offset 50`. More intuitive for agents than raw offset math.

**R2.3 - Include pagination metadata in JSON output.** (Covered in R1.1.) The envelope should contain `offset`, `limit`, `total`, `hasMore`, and `nextOffset`.

**R2.4 - Warn when total exceeds 10,000.** Print to stderr: `Warning: 15,432 total results but API maximum is 10,000. Refine your query.`

**R2.5 - Support `--all` with streaming NDJSON.** For large result sets, `--all -f ndjson` would emit one JSON object per line as it fetches, rather than buffering everything in memory.

---

## 3. Error Messages & Exit Codes

### Current State

The global error handler in `index.ts` catches `UsptoApiError` and prints human-readable messages to stderr, then exits with code 1. Commander errors (missing args, unknown commands) also exit with code 1. All errors go through `console.error()` as plain text.

The `UsptoApiError` class captures `statusCode` and `errorBody`, but the error output to the user flattens this into a string.

### Problems for Agents

1. **All errors share exit code 1.** An agent cannot distinguish between "rate limited" (retryable), "bad query syntax" (fixable), "not found" (skip), and "auth failure" (stop everything). Differentiated exit codes are essential for automated retry logic.

2. **JSON mode does not output errors as JSON.** If an agent runs `uspto search --title x -f json` and gets a 429, it receives plain text on stderr and no JSON on stdout. The agent's JSON parser will get nothing or throw, losing the error context.

3. **No structured error schema.** Error messages are ad-hoc strings like `"API Error (429): ..."` that require regex parsing.

4. **Download failures in `download-all` use `console.error` inline.** The final summary is also plain text, not structured.

### Recommendations (Priority: HIGH)

**R3.1 - Differentiated exit codes:**

| Code | Meaning | Agent Action |
|------|---------|-------------|
| 0 | Success | Process output |
| 1 | General/unknown error | Log and stop |
| 2 | Invalid arguments / bad query syntax | Fix query |
| 3 | Authentication failure (403) | Check API key |
| 4 | Not found (404) | Skip this item |
| 5 | Rate limited (429) | Wait and retry |
| 6 | Server error (5xx) | Retry with backoff |

**R3.2 - JSON error output in JSON mode.** When `-f json` is active, errors should be emitted as JSON to stdout:

```json
{
  "ok": false,
  "error": {
    "code": 429,
    "type": "RATE_LIMITED",
    "message": "Rate limit exceeded",
    "retryAfterMs": 5000,
    "requestId": "abc-123"
  }
}
```

**R3.3 - Add `--retry <n>` flag.** Auto-retry on 429 and 5xx errors up to `n` times with exponential backoff. The rate limiter already handles 429 backoff internally, but this would cover it at the command level too. Currently, a 429 throws and terminates the process; the 5-second backoff in `RateLimiter.markRateLimited()` only helps if there is a *subsequent* request, not a retry of the failed one.

**R3.4 - Retry logic in the client.** The `request()` method in `client.ts` should retry 429s automatically (at least once) before throwing. The current code marks the rate limiter but then throws immediately, so the backoff is wasted.

---

## 4. Composability

### Current State

Commands are independent. The only pipeline pattern documented is piping to `jq`. There are no commands designed to accept piped input, no batch-processing modes, and no cross-command composition helpers.

The `download-all` command is the only "compound" command, combining document listing and download in a loop.

### Problems for Agents

1. **Multi-step workflows require multiple invocations with manual glue.** The common workflow "search, then get details for each result, then download docs for each" requires the agent to:
   - Run `search -f json`
   - Parse the JSON to extract application numbers
   - Run `app get <n> -f json` for each (N sequential commands)
   - Run `app docs <n> -f json` for each
   - Run `app dl <n> <idx>` for each document

   This is O(N) separate process invocations with startup overhead per call.

2. **No stdin input support.** An agent cannot pipe application numbers into a command:
   ```bash
   # This does not work:
   echo "16123456\n17654321" | uspto app get --stdin -f json
   ```

3. **No batch operations.** There is no way to get details for multiple applications in one command.

### Recommendations (Priority: MEDIUM-HIGH)

**R4.1 - Add `--stdin` flag for batch input.** Accept newline-delimited input on stdin for commands that take a single identifier:

```bash
# Get details for all applications from a search
uspto search --title sensor -f json | jq -r '.results[].applicationNumberText' | uspto app get --stdin -f json
```

**R4.2 - Add `--app-numbers <file>` flag.** Read application numbers from a file (one per line) and process them sequentially.

**R4.3 - Add a `batch` meta-command.** Execute a command for multiple identifiers in one invocation:

```bash
uspto batch app get 16123456 17654321 18999999 -f json
```

This would output NDJSON (one result per line) and handle rate limiting internally.

**R4.4 - Add `--output-dir` to more commands.** Currently only `download-all` has output directory support. Adding it to search and app commands would let agents write results to files automatically:

```bash
uspto search --title sensor --all -f json --output-dir ./results/
# Creates ./results/page-001.json, ./results/page-002.json, etc.
```

**R4.5 - Add an `extract` utility command.** Pull specific fields from JSON input:

```bash
uspto search --title sensor -f json | uspto extract applicationNumberText applicationMetaData.patentNumber
```

This would replace the need for `jq` in simple cases.

---

## 5. Missing Convenience Commands

### Current State

The CLI maps 1:1 to API endpoints. There are no compound commands that combine multiple API calls into a single high-level operation. The only compound behavior is `download-all`.

### Recommendations (Priority: HIGH)

**R5.1 - `uspto summary <appNumber>` -- One-shot complete summary.**

Combines metadata, continuity, assignments, and recent transactions into a single structured output. This is the most common agent workflow: "tell me everything about this application."

```bash
uspto summary 16123456 -f json
```

Would make 4-5 API calls internally (metadata, continuity, assignment, transactions, documents) and return:

```json
{
  "application": { ... },
  "continuity": { "parents": [...], "children": [...] },
  "assignments": [...],
  "recentTransactions": [...],
  "documentCount": 47,
  "latestDocument": { ... }
}
```

This saves agents 4-5 separate invocations and eliminates the need to understand which sub-commands to call.

**R5.2 - `uspto family <appNumber>` -- Recursive patent family tree.**

Follows continuity chains (parents and children) recursively to build a complete family tree. This is a common patent research workflow that currently requires the agent to recursively call `app cont` for every discovered application number.

```bash
uspto family 16123456 -f json --depth 3
```

Output:

```json
{
  "root": "16123456",
  "tree": {
    "applicationNumberText": "16123456",
    "patentNumber": "12000000",
    "parents": [
      {
        "applicationNumberText": "15111111",
        "relationship": "CON",
        "parents": [...],
        "children": [...]
      }
    ],
    "children": [...]
  },
  "allApplicationNumbers": ["16123456", "15111111", "17222222", ...]
}
```

The `--depth` flag controls recursion depth (default 2). The `allApplicationNumbers` flat list makes it easy for agents to iterate.

**R5.3 - `uspto watch <query> [--since <date>]` -- New filings monitor.**

Runs a search filtered to filings since a given date (defaulting to 7 days ago) and returns only new results:

```bash
# Check for new filings matching a query in the last 7 days
uspto watch --title "autonomous vehicle" --since 2026-02-21 -f json
```

This is syntactic sugar for a date-filtered search, but it communicates intent clearly and could eventually support persistent state (tracking what has been seen before).

**R5.4 - `uspto export <query> --all --format csv` -- Full export.**

Combines auto-pagination with format conversion. Uses the API's own `/search/download` endpoint (which the client already wraps as `downloadPatents()` but is never exposed in any CLI command):

```bash
# Export all matching results as CSV
uspto export --title sensor --type UTL --format csv --output sensors.csv
```

The `downloadPatents()` method in `client.ts` already exists but has zero CLI surface area. This is a missed opportunity.

**R5.5 - `uspto prior-art <appNumber>` -- Prior art landscape.**

Given an application, extracts its CPC classes and runs searches for other applications in the same CPC classes filed before its effective filing date. Useful for competitive landscape analysis.

**R5.6 - `uspto timeline <appNumber>` -- Prosecution timeline.**

Combines transactions and documents into a chronological prosecution timeline. Agents frequently need to understand prosecution history, and combining events with documents into one timeline is more useful than separate lists.

---

## 6. Query Builder Improvements

### Current State

The search command supports shorthand flags (`--title`, `--applicant`, `--inventor`, `--patent`, `--cpc`, `--status`, `--type`, `--filed-after`, `--filed-before`) that get assembled into query string syntax with `AND` joining. Raw query syntax is also accepted as a positional argument.

The API supports both simplified syntax (query string) and advanced syntax (POST with JSON body including filters, rangeFilters, sort, facets, pagination). The CLI only uses the GET simplified syntax for search.

### Problems for Agents

1. **No OR support.** All shorthand flags are ANDed together. An agent cannot express `--title "machine learning" OR --title "deep learning"` using flags.

2. **No negation.** There is no `--not-status` or `NOT` shorthand.

3. **Advanced POST syntax is unused.** The `searchPatentsPost()` method exists in the client but has no CLI surface. The POST syntax supports structured filters, range filters, and multi-value filters that are awkward or impossible to express in query string syntax.

4. **Date range handling is split across two flags.** `--filed-after` and `--filed-before` must both be provided to create a range; there is no `--filed-within <period>` shorthand.

5. **CPC classes require exact codes.** An agent must know that `H04L` is the right code. There is no lookup or autocomplete.

6. **No saved/named queries.** An agent running the same search repeatedly must reconstruct the full query each time.

### Recommendations (Priority: MEDIUM)

**R6.1 - Add `--or` modifier.** Allow OR logic:

```bash
uspto search --title "machine learning" --or --title "deep learning"
```

Or support comma-separated values in shorthand flags:

```bash
uspto search --title "machine learning,deep learning" --match any
```

**R6.2 - Add `--not` prefix flags.** `--not-status 150` to exclude patented cases.

**R6.3 - Expose the POST search endpoint.** Add `--advanced` flag or a separate `search-post` command that accepts a JSON body from stdin or a file:

```bash
# Read advanced query from file
uspto search --query-file ./queries/my-search.json -f json

# Or from stdin
cat query.json | uspto search --query-stdin -f json
```

**R6.4 - Add relative date shorthands:**

```bash
--filed-within 90d        # Filed in the last 90 days
--filed-within 6m         # Filed in the last 6 months
--filed-within 2y         # Filed in the last 2 years
--filed-year 2024         # Filed anytime in 2024
--granted-after 2023-01-01  # Grant date filter (currently missing entirely)
```

**R6.5 - Add `--granted-after` / `--granted-before` flags.** Grant date filtering is a common need but not covered by any shorthand. Currently an agent must use raw query syntax: `applicationMetaData.grantDate:[2023-01-01 TO 2024-01-01]`.

**R6.6 - Add `--publication-number` shorthand.** Searching by publication number (e.g., US20230366018A1) is a very common pattern with no shorthand.

---

## 7. Performance

### Current State

- The rate limiter enforces a 100ms minimum gap between requests, which is conservative but safe given the burst limit of 1.
- No caching exists at any layer.
- Each CLI invocation creates a new client, new rate limiter state, and makes independent HTTP requests.
- `download-all` downloads files sequentially (correct given burst limit).
- No connection pooling or keep-alive management.

### Problems

1. **Rate limiter state is lost between invocations.** If an agent runs 10 sequential `uspto` commands rapidly, each one resets the rate limiter. The 100ms gap only applies within a single process. This could cause 429s when an agent calls the CLI in a tight loop.

2. **No caching of any kind.** Repeatedly fetching the same application data (common in family tree exploration) makes redundant API calls.

3. **The `summary` and `family` compound commands (if added) would benefit from parallel requests.** But the burst limit of 1 means they must be sequential regardless.

4. **No request deduplication.** If `family` recurses and discovers the same application number from multiple paths, it would fetch it multiple times.

5. **Startup overhead.** Each `bun run index.ts` invocation has runtime startup cost. For batch workflows, this adds up.

### Recommendations (Priority: MEDIUM)

**R7.1 - File-based rate limiter state.** Write last-request timestamps to a temp file (e.g., `/tmp/uspto-ratelimit`). Read it at startup to maintain rate limiting across invocations. This is critical for agent workflows that spawn many sequential processes.

**R7.2 - Local response cache.** Cache API responses to a local directory (`~/.cache/uspto/`) keyed by URL + params with a configurable TTL (default: 1 hour for metadata, 24 hours for static data like status codes).

```bash
# Skip cache
uspto app get 16123456 --no-cache

# Custom TTL
uspto app get 16123456 --cache-ttl 3600

# Clear cache
uspto cache clear
```

This dramatically reduces API calls in recursive operations (family tree) and repeated queries.

**R7.3 - Request deduplication in compound commands.** The `family` command should maintain a visited set and skip already-fetched application numbers.

**R7.4 - Consider a daemon/server mode.** For heavy agent usage, a long-running process that accepts requests over stdin or a local socket would eliminate startup overhead and maintain rate limiter state:

```bash
# Start daemon
uspto serve --port 8765

# Agent sends requests via HTTP or stdin
curl localhost:8765/search?title=sensor
```

This is lower priority but would significantly improve throughput for agents making hundreds of calls.

---

## 8. Output Formats

### Current State

Two real formats: `table` (cli-table3 with chalk colors) and `json` (pretty-printed). `compact` is listed but is a no-op alias for `json`.

### Problems

1. **No CSV/TSV output.** These are standard data interchange formats. Agents often need to write results to spreadsheets or databases.

2. **No NDJSON.** Newline-delimited JSON (one object per line) is the standard for streaming and piping JSON data. It is significantly easier for agents to parse incrementally.

3. **Table format includes ANSI color codes.** When piped to a file or another program, the output contains escape sequences that must be stripped. There is no `--no-color` flag (chalk may auto-detect, but it is not guaranteed in all agent environments).

4. **JSON is always pretty-printed.** `JSON.stringify(data, null, 2)` wastes bandwidth and tokens. Agents do not need indentation.

### Recommendations (Priority: MEDIUM)

**R8.1 - Add `ndjson` format.** One JSON object per line, ideal for streaming and piping:

```bash
uspto search --title sensor --all -f ndjson | while read -r line; do
  echo "$line" | jq .applicationNumberText
done
```

**R8.2 - Add `csv` format.** Use a predefined set of columns (or columns from `--pick`):

```bash
uspto search --title sensor -f csv --pick applicationNumberText,applicationMetaData.inventionTitle
```

Output:
```csv
applicationNumberText,inventionTitle
18045436,"LABELED NUCLEOTIDE ANALOGS..."
```

**R8.3 - Add `tsv` format.** Same as CSV but tab-delimited, easier for shell processing with `cut` and `awk`.

**R8.4 - Add `--no-color` global flag.** Force disable ANSI escape codes. Also respect the `NO_COLOR` environment variable (standard convention).

**R8.5 - Add `--minify` flag for JSON.** Output `JSON.stringify(data)` without indentation. Saves significant bytes and tokens for LLM consumption.

**R8.6 - Expose the download endpoint.** The API has a `/search/download` endpoint that returns CSV/JSON natively. The client wraps it (`downloadPatents()`) but no CLI command uses it. Wire it up:

```bash
uspto search --title sensor --download --format csv > results.csv
```

---

## 9. Additional Architectural Observations

### 9.1 - Missing `--version` in output envelope

When debugging agent issues, knowing which CLI version produced the output is valuable. The JSON envelope (R1.1) should include `"cliVersion": "0.1.0"`.

### 9.2 - No `--dry-run` flag

An agent building complex queries cannot preview what API call will be made without executing it. A `--dry-run` flag that prints the URL and body (for POST) without making the request would aid debugging:

```bash
uspto search --title sensor --inventor "Smith" --filed-after 2023-01-01 --dry-run
# GET https://api.uspto.gov/api/v1/patent/applications/search?q=applicationMetaData.inventionTitle:sensor AND ...
```

### 9.3 - Debug output goes to stderr (good)

The `[DEBUG]` output correctly uses `console.error()`, keeping stdout clean for data. This is correct.

### 9.4 - `any` types in the client

Several client methods return `Promise<any>` (`getMetadata`, `getAdjustment`, `getAssignment`, `getAttorney`, `getContinuity`, `getForeignPriority`, `getTransactions`). These should have proper return types for better tooling and documentation.

### 9.5 - No `--help` examples

Commander's help output lists flags but not usage examples. Adding `.addHelpText('after', ...)` with examples would help agents that read help text to understand flag combinations.

### 9.6 - No input validation on application numbers

Application numbers should be validated (8-digit numeric) before making API calls. Currently, invalid inputs result in 404 or 400 errors that consume rate limit quota.

### 9.7 - Missing `--timeout` flag

The client has no request timeout. A hung request will block indefinitely. Add a configurable timeout (default: 30s, with 600s for downloads per the API docs' recommendation).

---

## Priority Summary

### Tier 1 -- Do First (highest impact for agents)

| ID | Recommendation | Effort |
|----|----------------|--------|
| R1.1 | Standardized JSON envelope with pagination metadata | Medium |
| R2.1 | `--all` auto-pagination flag | Medium |
| R3.1 | Differentiated exit codes | Low |
| R3.2 | JSON error output in JSON mode | Low |
| R3.4 | Auto-retry on 429 in the client | Low |
| R5.1 | `uspto summary` compound command | Medium |
| R5.4 | `uspto export` using existing download endpoint | Low |

### Tier 2 -- Do Next (significant quality of life)

| ID | Recommendation | Effort |
|----|----------------|--------|
| R1.3 | `--pick` output field selection | Medium |
| R2.2 | `--page` convenience flag | Low |
| R4.1 | `--stdin` batch input | Medium |
| R5.2 | `uspto family` recursive tree | High |
| R6.4 | Relative date shorthands (`--filed-within`) | Low |
| R6.5 | Grant date filter flags | Low |
| R7.1 | File-based rate limiter state | Low |
| R8.1 | NDJSON output format | Low |
| R8.5 | `--minify` for JSON output | Low |

### Tier 3 -- Polish (nice-to-have)

| ID | Recommendation | Effort |
|----|----------------|--------|
| R1.2 | Real `compact` format implementation | Low |
| R1.4 | `--quiet` flag | Low |
| R4.3 | `batch` meta-command | Medium |
| R5.3 | `uspto watch` new filings monitor | Medium |
| R5.6 | `uspto timeline` prosecution timeline | Medium |
| R6.1 | OR logic in shorthand flags | Medium |
| R6.3 | POST search via `--query-file` | Medium |
| R7.2 | Local response cache | High |
| R8.2 | CSV output format | Medium |
| R8.4 | `--no-color` and `NO_COLOR` env support | Low |
| R9.2 | `--dry-run` flag | Low |
| R9.6 | Input validation on application numbers | Low |
| R9.7 | `--timeout` flag | Low |

---

## Key Takeaway

The CLI has strong API coverage (53 endpoints) and a solid foundation. The single biggest gap for agent usability is the lack of a **standardized output envelope** (R1.1) combined with **auto-pagination** (R2.1) and **JSON-mode error output** (R3.2). These three changes alone would transform the CLI from "agent-usable with workarounds" to "agent-native." The compound commands (`summary`, `family`, `export`) are the next tier -- they eliminate the most common multi-call workflows that force agents to orchestrate complex sequences externally.

