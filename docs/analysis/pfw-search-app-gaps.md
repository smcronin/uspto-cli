# Patent File Wrapper: Search & Application Data -- Gap Analysis

> Generated: 2026-02-28
> Scope: Comparison of USPTO ODP API documentation vs current CLI implementation
> Files analyzed:
> - API docs: `docs/raw/pfw-search.txt`, `docs/raw/pfw-application-data.txt`, `docs/raw/api-syntax-examples.txt`
> - CLI code: `src/commands/search.ts`, `src/commands/app.ts`, `src/api/client.ts`, `src/types/api.ts`, `src/utils/format.ts`

---

## Executive Summary

The CLI has a solid foundation but is currently a thin wrapper around the GET endpoint's simplified query syntax. The API's most powerful capabilities -- POST-based structured search with typed filters, range filters, facets, field projection, and CSV download -- are defined in types but not wired into the CLI commands. Several shorthand flags have wrong or inconsistent field paths. The formatter ignores 15+ response fields that are critical for agent workflows. Pagination is entirely manual with no auto-paging support despite the API's hard 10,000 result cap.

**Severity tally:** 6 Critical, 12 Major, 9 Minor, 8 Enhancements

---

## 1. CRITICAL: Search Command Uses GET Only -- POST Body Is Never Constructed

### What the API supports (from `api-syntax-examples.txt`)

The POST endpoint accepts a structured JSON body with five distinct sections:

```json
{
  "q": "free-text query",
  "filters": [{ "name": "field.path", "value": ["val1", "val2"] }],
  "rangeFilters": [{ "field": "field.path", "valueFrom": "...", "valueTo": "..." }],
  "pagination": { "offset": 0, "limit": 25 },
  "sort": [{ "field": "field.path", "order": "Desc" }],
  "fields": ["applicationNumberText", "applicationMetaData"],
  "facets": ["applicationMetaData.applicationTypeLabelName"]
}
```

### What the CLI does (`src/commands/search.ts`, lines 49-56)

The CLI **always** uses `client.searchPatents()` which is the GET method (line 150 of `client.ts`). The GET endpoint passes everything as query string parameters, which:
- Cannot express multi-value filters (e.g., status IN ["Patented Case", "Non Final Action Mailed"])
- Cannot express multiple sort fields
- Cannot express structured range filters separately from the `q` parameter
- Cannot express field projection as an array

The `searchPatentsPost()` method exists in the client (line 153) but is **never called** from any command.

### Recommendation

Add a `--post` flag or auto-detect when POST is needed (when filters, rangeFilters, facets, or multi-value options are present). Build the `SearchRequest` body from CLI options:

```typescript
// search.ts -- new logic after option parsing
if (needsPost) {
  const body: SearchRequest = {
    q: q || undefined,
    filters: parsedFilters,        // from --filter flag
    rangeFilters: parsedRanges,    // from --filed-after/--filed-before
    pagination: { offset, limit },
    sort: parsedSort,              // from --sort
    fields: parsedFields,          // from --fields
    facets: parsedFacets,          // from --facets
  };
  result = await client.searchPatentsPost(body);
}
```

---

## 2. CRITICAL: Date Range Handling -- Filed-After/Filed-Before Broken on GET, Not Using rangeFilters on POST

### Current behavior (`src/commands/search.ts`, lines 41-45)

```typescript
if (opts.filedAfter || opts.filedBefore) {
  const from = opts.filedAfter || "2001-01-01";
  const to = opts.filedBefore || "2099-12-31";
  parts.push(`applicationMetaData.filingDate:[${from} TO ${to}]`);
}
```

Problems:
1. The option is defined as `--filed-after` (line 23) which Commander converts to `opts.filedAfter`, but it uses the simplified query syntax `[FROM TO TO]` embedded in the `q` parameter. This works on GET but is fragile.
2. On POST, date ranges should use the `rangeFilters` array, not be embedded in `q`.
3. The API also supports `effectiveFilingDate`, `grantDate`, `applicationStatusDate`, `earliestPublicationDate`, and `pctPublicationDate` -- none of these have shorthand date range flags.

### Recommendation

Add date range flags for all documented date fields:
```
--filed-after <date>       filingDate range start
--filed-before <date>      filingDate range end
--granted-after <date>     grantDate range start
--granted-before <date>    grantDate range end
--published-after <date>   earliestPublicationDate range start
--published-before <date>  earliestPublicationDate range end
```

When using POST, construct `rangeFilters`:
```typescript
rangeFilters.push({
  field: "applicationMetaData.filingDate",
  valueFrom: opts.filedAfter,
  valueTo: opts.filedBefore,
});
```

---

## 3. CRITICAL: --filters and --facets Are Passed as Raw Strings, Not Parsed into Structured Objects

### Current behavior (`src/commands/search.ts`, lines 14-15; `src/api/client.ts`, lines 148-149)

```typescript
// search.ts
.option("--filters <filters>", "Field-value filters")
.option("--facets <facets>", "Comma-separated facet fields")

// client.ts
if (opts.filters) params.filters = opts.filters;
if (opts.facets) params.facets = opts.facets;
```

The `--filters` value is passed as a raw string query parameter. The GET endpoint may or may not accept this (the Swagger docs for GET show `q` as the primary mechanism). But for POST, filters must be structured:

```json
"filters": [
  { "name": "applicationMetaData.applicationTypeLabelName", "value": ["Utility"] },
  { "name": "applicationMetaData.publicationCategoryBag", "value": ["Granted/Issued"] }
]
```

### Recommendation

A) For GET: Remove `--filters` since it has unclear behavior on the GET endpoint. Instead, expand shorthands.

B) For POST: Parse `--filter` (repeatable) into `Filter[]`:
```
--filter "applicationTypeLabelName=Utility"
--filter "publicationCategoryBag=Granted/Issued,Pre-Grant Publications - PGPub"
```

Parse logic:
```typescript
// Allow repeatable --filter flags
.option("--filter <expr...>", "Field=value filter (repeatable, comma-separated values)")
```

For facets, parse comma-separated string into `string[]`:
```typescript
const facetFields = opts.facets?.split(",").map(f =>
  f.includes(".") ? f : `applicationMetaData.${f}`
);
```

---

## 4. CRITICAL: --fields Flag Is Passed as a String, Not an Array; Field Projection Not Fully Leveraged

### Current behavior (`src/api/client.ts`, line 149)

```typescript
if (opts.fields) params.fields = opts.fields;
```

This passes `fields` as a single string query parameter. The POST body expects `"fields": ["applicationNumberText", "applicationMetaData"]`.

### What the API supports (`api-syntax-examples.txt`, Example 3, line within query_template)

```python
"fields": ["applicationNumberText","applicationMetaData"]
```

Field projection is extremely valuable for agent workflows because it reduces response payload size dramatically. A search returning only `applicationNumberText` and `applicationMetaData.inventionTitle` vs the full response (which includes `assignmentBag`, `eventDataBag`, `patentTermAdjustmentData`, etc.) can be 50-100x smaller.

### Recommendation

1. Parse `--fields` into an array: `opts.fields.split(",")`.
2. Add shorthand presets for agent use:
```
--fields-preset brief     -> applicationNumberText,applicationMetaData.inventionTitle,applicationMetaData.filingDate,applicationMetaData.patentNumber
--fields-preset status    -> applicationNumberText,applicationMetaData.applicationStatusDescriptionText,applicationMetaData.applicationStatusDate
--fields-preset full      -> (no field restriction)
```

---

## 5. CRITICAL: No Auto-Pagination / Pagination Helpers

### What the API supports (`pfw-search.txt`, line 15)

> "The Search API has a maximum of 10,000 results."

The API returns `count` in the response indicating total matches, and pagination is via `offset` + `limit`.

### What the CLI provides

Manual `--offset` and `--limit` only. An agent needing all 3,000 results from a search must manually call:
```
search --offset 0 --limit 100
search --offset 100 --limit 100
search --offset 200 --limit 100
... (30 times)
```

### Recommendation

Add `--all` flag that auto-pages:
```typescript
.option("--all", "Fetch all results (auto-paginate up to 10,000)")
.option("--max <n>", "Max total results when using --all", "10000")
```

Implementation:
```typescript
if (opts.all) {
  const allResults: PatentFileWrapper[] = [];
  let offset = 0;
  const limit = 100; // optimal page size
  const max = parseInt(opts.max);

  do {
    const page = await client.searchPatentsPost({ ...body, pagination: { offset, limit } });
    allResults.push(...page.patentFileWrapperDataBag);
    offset += limit;
    if (offset >= page.count || offset >= max) break;
  } while (true);

  // Output all results
}
```

Also add `--count-only` to just return the total count without fetching data:
```
search "wireless" --count-only
# Output: 4,327 results found
```

---

## 6. CRITICAL: No CSV/Download Export from Search

### What the API supports (`src/api/client.ts`, lines 157-164)

The client has `downloadPatents()` which hits `/api/v1/patent/applications/search/download` with a `format` parameter accepting `"json"` or `"csv"`. This method exists but is **never exposed** in any CLI command.

### Recommendation

Add `--download` and `--csv` flags to the search command:
```
search "wireless" --csv --output results.csv
search "wireless" --download json --output results.json
```

Or add a dedicated subcommand:
```
search download "wireless" --format csv --output results.csv
```

---

## 7. MAJOR: --sort Flag Parsing Is Wrong

### Current behavior (`src/commands/search.ts`, line 12; `src/api/client.ts`, line 148)

```typescript
.option("-s, --sort <field>", "Sort field and order (e.g., filingDate desc)")
// client.ts
if (opts.sort) params.sort = opts.sort;
```

The sort value is passed as a raw string `"filingDate desc"`. For the GET endpoint, the expected format based on OpenSearch conventions is likely `applicationMetaData.filingDate:desc` or similar. For POST, it needs to be:

```json
"sort": [{ "field": "applicationMetaData.filingDate", "order": "Desc" }]
```

Problems:
1. The field name likely needs the full path `applicationMetaData.filingDate`, not just `filingDate`.
2. The space-separated format `"filingDate desc"` is ambiguous and may not match what the GET endpoint expects.
3. POST sort order values are `"Asc"` / `"Desc"` (capitalized), per the API examples.

### Recommendation

1. Accept format `--sort filingDate:desc` and auto-prefix with `applicationMetaData.` if not already prefixed.
2. For POST, parse into `SortField[]`:
```typescript
const [field, order] = opts.sort.split(":");
sort.push({
  field: field.includes(".") ? field : `applicationMetaData.${field}`,
  order: (order || "desc").charAt(0).toUpperCase() + (order || "desc").slice(1) as "Asc" | "Desc"
});
```

---

## 8. MAJOR: Shorthand Field Paths May Be Wrong or Incomplete

### Current shorthands (`src/commands/search.ts`, lines 31-38)

| Shorthand | Field Path Used | Correct? |
|-----------|----------------|----------|
| `--title` | `applicationMetaData.inventionTitle` | YES |
| `--applicant` | `applicationMetaData.firstApplicantName` | PARTIAL - only searches first applicant, not `applicantBag.applicantNameText` |
| `--inventor` | `applicationMetaData.inventorBag.inventorNameText` | YES for nested search |
| `--patent` | `applicationMetaData.patentNumber` | YES |
| `--cpc` | `applicationMetaData.cpcClassificationBag` | YES |
| `--status` | `applicationMetaData.applicationStatusCode` | YES but numeric; users will want to use `applicationStatusDescriptionText` |
| `--type` | `applicationMetaData.applicationTypeCode` | YES |

### Missing shorthands that agents need

From the API response fields (`pfw-search.txt`):

| Suggested Flag | Field Path | Use Case |
|---------------|-----------|----------|
| `--examiner <name>` | `applicationMetaData.examinerNameText` | Find all apps by an examiner |
| `--art-unit <num>` | `applicationMetaData.groupArtUnitNumber` | Find apps in an art unit |
| `--assignee <name>` | `assignmentBag.assigneeBag.assigneeNameText` | Find by current owner |
| `--docket <num>` | `applicationMetaData.docketNumber` | Find by docket number |
| `--pub-number <num>` | `applicationMetaData.earliestPublicationNumber` | Find by publication number |
| `--app-category <cat>` | `applicationMetaData.applicationTypeCategory` | REGULAR, CONTINUATION, etc. |
| `--entity <status>` | `applicationMetaData.entityStatusData.businessEntityStatusCategory` | Small entity filtering |
| `--status-text <text>` | `applicationMetaData.applicationStatusDescriptionText` | Search by status description text |
| `--customer-number <num>` | `applicationMetaData.customerNumber` | Find all apps for a law firm |
| `--confirmation <num>` | `applicationMetaData.applicationConfirmationNumber` | Verify app identity |

---

## 9. MAJOR: `app get` vs `app meta` -- Redundant and Confused

### Current behavior (`src/commands/app.ts`)

- `app get` (line 17-31): Calls `client.getApplication(appNumber)` which hits `/api/v1/patent/applications/{appNumber}` (full data)
- `app meta` (line 33-47): Calls `client.getMetadata(appNumber)` which hits `/api/v1/patent/applications/{appNumber}/meta-data` (metadata only)

Both use `formatPatentDetail()` for output. But `app get` returns the **full** `PatentFileWrapper` (with `assignmentBag`, `eventDataBag`, `continuityBag`, `patentTermAdjustmentData`, etc.) while `app meta` returns only `applicationMetaData`.

### Problem

`formatPatentDetail()` (line 46 of `format.ts`) only reads from `applicationMetaData`, so `app get` and `app meta` produce **identical output** despite fetching vastly different amounts of data.

### Recommendation

1. Make `app get` show a comprehensive view including summaries of assignments, continuity, recent transactions, and PTA data.
2. Or rename: `app get` -> `app full` and add flags like `--include assignments,continuity,pta`.
3. Add field selection: `app get 16123456 --sections meta,assignments,continuity`.

---

## 10. MAJOR: Response Fields Ignored in Formatters

### `formatPatentTable()` (`src/utils/format.ts`, lines 15-44)

Shows 6 columns. Fields from the API response that are available but **not displayed**:

| Available Field | Agent Value |
|----------------|------------|
| `applicationMetaData.grantDate` | Critical for determining patent status |
| `applicationMetaData.applicationTypeCode` / `applicationTypeLabelName` | Utility vs Design vs Plant |
| `applicationMetaData.groupArtUnitNumber` | Art unit for examiner context |
| `applicationMetaData.examinerNameText` | Examiner name |
| `applicationMetaData.cpcClassificationBag` | CPC codes |
| `applicationMetaData.earliestPublicationNumber` | Publication ID |
| `applicationMetaData.entityStatusData.businessEntityStatusCategory` | Entity status |
| `applicationMetaData.docketNumber` | Docket tracking |
| `applicationMetaData.applicationTypeCategory` | REGULAR, CONTINUATION, etc. |
| `lastIngestionDateTime` | Data freshness |

### `formatPatentDetail()` (`src/utils/format.ts`, lines 46-72)

Shows 18 fields. Missing fields that the API returns:

| Missing Field | Source |
|---------------|--------|
| `applicationMetaData.pctPublicationNumber` | PCT data |
| `applicationMetaData.pctPublicationDate` | PCT data |
| `applicationMetaData.nationalStageIndicator` | PCT national stage |
| `applicationMetaData.applicationTypeCategory` | REGULAR vs CONTINUATION etc. |
| `applicationMetaData.publicationCategoryBag` | Publication categories |
| `applicationMetaData.publicationDateBag` | All publication dates |
| `applicationMetaData.internationalRegistrationNumber` | Design international reg |
| `applicationMetaData.internationalRegistrationPublicationDate` | Design international reg |
| `grantDocumentMetaData.fileLocationURI` | Direct link to grant XML |
| `pgpubDocumentMetaData.fileLocationURI` | Direct link to pgpub XML |
| `lastIngestionDateTime` | When data was last updated |

### Recommendation

1. Add `--verbose` / `-v` flag to show all available fields.
2. Add `--columns <col1,col2,...>` flag for the table format to let agents pick exactly which columns they want.
3. Always show `lastIngestionDateTime` in detail views since it tells agents how fresh the data is.

---

## 11. MAJOR: `compact` Output Format Is Declared But Not Implemented

### Current behavior (`src/commands/search.ts`, line 16)

```typescript
.option("-f, --format <fmt>", "Output format: table, json, compact", "table")
```

The `compact` format is listed as an option but never handled:

```typescript
// Lines 58-63
if (opts.format === "json") {
  console.log(formatOutput(result, "json"));
} else {
  // "compact" falls through to "table"
  console.log(`\n${result.count} results found\n`);
  console.log(formatPatentTable(result.patentFileWrapperDataBag));
}
```

### Recommendation

Implement `compact` as a single-line-per-result format ideal for agent piping:
```
16123456|12000000|WIRELESS DEVICE|2022-01-15|Patented Case|Acme Corp
16123457|-|BATTERY SYSTEM|2023-03-20|Non Final Action Mailed|Battery Inc
```

Also consider adding:
- `--format tsv` -- tab-separated values
- `--format csv` -- comma-separated values
- `--format ids` -- just application numbers, one per line (for piping into other commands)

---

## 12. MAJOR: No `--raw` Flag to Bypass Formatting

For agent workflows, the most common need is raw JSON output with no formatting overhead. While `--format json` exists, it should be aliased to a shorter flag.

### Recommendation

Add `-j` / `--json` as a shorthand for `--format json` across all commands. Several `app` subcommands (attorney, adjustment, foreign-priority, associated-docs) already output JSON-only with no table formatter, which is inconsistent.

---

## 13. MAJOR: `applicantBag` and `inventorBag` Typed as `any[]` in Types

### Current types (`src/types/api.ts`, lines 97-98)

```typescript
applicantBag: any[];
inventorBag: any[];
```

### What the API returns (from `pfw-search.txt` and `pfw-application-data.txt`)

These have well-defined structures:

```typescript
interface Applicant {
  applicantNameText: string;
  firstName?: string;
  middleName?: string;
  lastName?: string;
  preferredName?: string;
  namePrefix?: string;
  nameSuffix?: string;
  countryCode?: string;
  correspondenceAddressBag?: CorrespondenceAddress[];
}

interface Inventor {
  firstName?: string;
  middleName?: string;
  lastName?: string;
  preferredName?: string;
  namePrefix?: string;
  nameSuffix?: string;
  countryCode?: string;
  inventorNameText?: string;
  correspondenceAddressBag?: CorrespondenceAddress[];
}
```

### Impact

Without typed bags, the formatter at `format.ts` line 39 can only access `firstApplicantName` rather than listing all applicants. The `formatAssignmentTable` at line 177 has to guess property names with fallbacks (`x.name || x.assignorName || x.assigneeName`).

### Recommendation

Define proper interfaces for `Applicant`, `Inventor`, `Assignor`, `Assignee`, `CorrespondenceAddress`, and `AttorneyData` in `types/api.ts`. Replace all `any[]` with specific types.

---

## 14. MAJOR: `getMetadata()` Returns `Promise<any>` -- No Type Safety

### Current behavior (`src/api/client.ts`, line 170)

```typescript
async getMetadata(appNumber: string): Promise<any> {
```

Several other endpoint methods also return `Promise<any>`:
- `getMetadata` (line 170)
- `getAdjustment` (line 174)
- `getAssignment` (line 178)
- `getAttorney` (line 182)
- `getContinuity` (line 186)
- `getForeignPriority` (line 190)
- `getTransactions` (line 194)
- `getAssociatedDocuments` (line 206)

### Recommendation

All of these should return `Promise<PatentDataResponse>` since the API wraps everything in `{ count, patentFileWrapperDataBag: [...] }`. This is confirmed by the JSON response samples in both API doc files.

---

## 15. MAJOR: No 429 Retry Logic in Main Request Path

### Current behavior (`src/api/client.ts`, lines 119-134)

```typescript
if (response.status === 429) {
  this.rateLimiter.markRateLimited();
}
// ... throws UsptoApiError
```

When a 429 is received, the error is marked but the request **still throws**. The caller gets an exception. The API docs (`api-syntax-examples.txt`, Example 3) show explicit retry logic:

```python
elif response.status_code == 429:
    if retry < HTTP_RETRY:
        time.sleep(SLEEP_AFTER_429)
        retry += 1
        return make_search_request(offset, retry)
```

### Recommendation

Add automatic retry with backoff inside the `request()` method:

```typescript
private async request<T>(method, path, options, retries = 3): Promise<T> {
  // ... existing logic ...
  if (response.status === 429 && retries > 0) {
    this.rateLimiter.markRateLimited();
    await this.rateLimiter.waitForSlot();
    return this.request(method, path, options, retries - 1);
  }
}
```

---

## 16. MAJOR: Download Endpoints Missing Redirect Handling Warning

### API documentation (`api-syntax-examples.txt`, Example 4)

> "ODP uses an HTTP redirect 301 to provide a direct link to download files. Most software libraries will follow redirect automatically or by adding an extra parameter."

### Current behavior (`src/api/client.ts`, line 344)

```typescript
const response = await fetch(url, { headers: this.headers, redirect: "follow" });
```

The `redirect: "follow"` is correctly set. However, the API key header may not be forwarded on the redirect. Some fetch implementations strip headers on cross-origin redirects. The API docs specifically warn about this.

### Recommendation

Add explicit redirect handling that re-applies the API key:
```typescript
const response = await fetch(url, { headers: this.headers, redirect: "manual" });
if (response.status === 301 || response.status === 302) {
  const redirectUrl = response.headers.get("location");
  return fetch(redirectUrl, { headers: this.headers });
}
```

Also add the recommended 600-second timeout for large files as the docs state:
> "we recommend setting your maximum time-out to start generating content at 600 seconds"

---

## 17. MAJOR: No `--examiner` Shorthand Despite Being in API Response

### What the API returns (`pfw-application-data.txt`, line 55; `pfw-search.txt`, JSON sample line 339)

```json
"examinerNameText": "RILEY, JEZIA"
```

This field is returned in both search results and application metadata. It is displayed in `formatPatentDetail()` (line 60 of `format.ts`) but there is no `--examiner` search shorthand.

### Recommendation

Add to `search.ts`:
```typescript
.option("--examiner <name>", "Search by examiner name (shorthand)")
// ...
if (opts.examiner) parts.push(`applicationMetaData.examinerNameText:${opts.examiner.includes(" ") ? `"${opts.examiner}"` : opts.examiner}`);
```

---

## 18. MINOR: Search Query Quoting Logic Is Inconsistent

### Current behavior (`src/commands/search.ts`, lines 32-38)

```typescript
if (opts.title) parts.push(`applicationMetaData.inventionTitle:${opts.title.includes(" ") ? `"${opts.title}"` : opts.title}`);
```

This only wraps in quotes if the value contains a space. But what about values with special characters like `*`, `?`, `:`, `[`, `]` which have meaning in the OpenSearch query syntax?

### Recommendation

Create a helper function:
```typescript
function quoteIfNeeded(value: string): string {
  if (/[\s:*?\[\]{}()"]/.test(value)) {
    return `"${value.replace(/"/g, '\\"')}"`;
  }
  return value;
}
```

Also, the API supports wildcards. Add a note or flag:
```
--title "wireless*"   # wildcard search
--title "\"exact phrase\""   # exact match
```

---

## 19. MINOR: `--type` Flag Values Not Validated or Documented

### Current behavior (`src/commands/search.ts`, line 25)

```typescript
.option("--type <code>", "Application type: UTL, PLT, DSN, REI")
```

The valid codes are listed in the help text but not validated. An agent passing `--type utility` would get zero results instead of an error.

### Recommendation

Add a choices validator:
```typescript
.option("--type <code>", "Application type")
.choices(["UTL", "PLT", "DSN", "REI", "PPA", "PCT"])  // Commander supports .choices()
```

Also, the API JSON sample shows additional values beyond the four listed:
- `applicationTypeCategory`: "REGULAR", "CONTINUATION", etc.
- `applicationTypeLabelName`: "Utility", "Design", "Plant", "Reissue"

Consider accepting human-friendly names and mapping them:
```
--type Utility  ->  applicationTypeCode:UTL
--type Design   ->  applicationTypeCode:DSN
```

---

## 20. MINOR: `assigneeBag` Formatter Uses Wrong Property Names

### Current behavior (`src/utils/format.ts`, line 177)

```typescript
const assignees = (a.assigneeBag || []).map((x: any) => x.name || x.assigneeName || "").join(", ");
```

### What the API returns (`pfw-search.txt`, line 132)

The field is `assigneeNameText`, not `assigneeName`:

```
assigneeNameText | A person or entity that has the property rights... | String
```

### Recommendation

Fix to:
```typescript
const assignees = (a.assigneeBag || []).map((x: any) => x.assigneeNameText || x.name || "").join(", ");
```

---

## 21. MINOR: `assignorBag` Formatter Uses Wrong Property Names

### Current behavior (`src/utils/format.ts`, line 176)

```typescript
const assignors = (a.assignorBag || []).map((x: any) => x.name || x.assignorName || "").join(", ");
```

### What the API returns (`pfw-search.txt`, line 129)

The field is `assignorName` (correct as second fallback), but the order should prefer `assignorName` first since that is the documented field:

```
assignorName | A party that transfers its interest... | String
```

### Recommendation

```typescript
const assignors = (a.assignorBag || []).map((x: any) => x.assignorName || x.name || "").join(", ");
```

---

## 22. MINOR: `PatentTermAdjustmentData` Missing From Detail Formatter

### What the API returns (`pfw-search.txt`, lines 218-233)

PTA data includes `adjustmentTotalQuantity`, `aDelayQuantity`, `bDelayQuantity`, `cDelayQuantity`, `applicantDayDelayQuantity`, `overlappingDayQuantity`, `nonOverlappingDayQuantity`, and a full history bag.

### Current behavior

`formatPatentDetail()` does not show any PTA data. The `app pta` command exists but only outputs raw JSON.

### Recommendation

Add PTA summary to `formatPatentDetail()`:
```typescript
if (p.patentTermAdjustmentData?.adjustmentTotalQuantity !== undefined) {
  lines.push(`  ${chalk.gray("PTA Total:")}     ${p.patentTermAdjustmentData.adjustmentTotalQuantity} days`);
  lines.push(`  ${chalk.gray("PTA A/B/C:")}     ${p.patentTermAdjustmentData.aDelayQuantity}/${p.patentTermAdjustmentData.bDelayQuantity}/${p.patentTermAdjustmentData.cDelayQuantity}`);
}
```

---

## 23. MINOR: `grantDocumentMetaData` and `pgpubDocumentMetaData` Not Shown

### What the API returns (`pfw-search.txt`, lines 238-249)

```json
"grantDocumentMetaData": {
  "fileLocationURI": "https://api.uspto.gov/api/v1/datasets/products/files/PTGRXML-SPLT/2024/ipg240604/18045436_12000000.xml"
}
```

These provide direct URLs to the grant and pre-grant publication XML documents.

### Current behavior

Never displayed in any formatter. The `app associated-docs` / `app xml` command exists but only dumps raw JSON.

### Recommendation

Add to `formatPatentDetail()`:
```typescript
if (p.grantDocumentMetaData?.fileLocationURI) {
  lines.push(`  ${chalk.gray("Grant XML:")}     ${p.grantDocumentMetaData.fileLocationURI}`);
}
if (p.pgpubDocumentMetaData?.fileLocationURI) {
  lines.push(`  ${chalk.gray("PGPub XML:")}     ${p.pgpubDocumentMetaData.fileLocationURI}`);
}
```

---

## 24. MINOR: `lastIngestionDateTime` Not Surfaced

### What the API returns (`pfw-search.txt`, line 250)

```
lastIngestionDateTime | Date time when application was last modified. | Date
```

Sample: `"lastIngestionDateTime": "2025-01-29T23:32:15"`

### Current behavior

This field exists in the `PatentFileWrapper` type (line 187 of `api.ts`) but is never displayed in any formatter.

### Recommendation

Always show this in detail views:
```typescript
lines.push(`  ${chalk.gray("Last Updated:")}  ${p.lastIngestionDateTime || "-"}`);
```

This is critical for agents to assess data freshness.

---

## 25. MINOR: `--status` Flag Expects Numeric Code But Users Think in Text

### Current behavior (`src/commands/search.ts`, line 37)

```typescript
if (opts.status) parts.push(`applicationMetaData.applicationStatusCode:${opts.status}`);
```

The status code is numeric (e.g., 150 = "Patented Case"). Users and agents rarely know numeric codes.

### Recommendation

1. Add `--status-text <text>` that searches `applicationStatusDescriptionText` instead.
2. Or better: detect if the value is numeric vs text and route to the correct field:
```typescript
if (opts.status) {
  if (/^\d+$/.test(opts.status)) {
    parts.push(`applicationMetaData.applicationStatusCode:${opts.status}`);
  } else {
    parts.push(`applicationMetaData.applicationStatusDescriptionText:"${opts.status}"`);
  }
}
```

---

## 26. ENHANCEMENT: Add `search count` Subcommand

For agent workflows that need to gauge result volume before fetching data:

```
$ uspat search count "wireless AND applicationMetaData.filingDate:[2024-01-01 TO 2024-12-31]"
4,327
```

This would make a search request with `limit=0` (or `limit=1`) and only return the `count` field.

---

## 27. ENHANCEMENT: Add Pipe-Friendly Application Number Output

Agents frequently need to pipe search results into per-application commands:

```bash
uspat search --title "wireless" --format ids | xargs -I{} uspat app get {}
```

Add `--format ids` that outputs one application number per line:
```
16123456
16123457
16123458
```

---

## 28. ENHANCEMENT: Add `--granted` and `--pending` Convenience Filters

These are the most common agent filter patterns:

```
--granted    ->  filter: publicationCategoryBag = "Granted/Issued"
--pending    ->  filter: publicationCategoryBag = "Pre-Grant Publications - PGPub"
```

These map to the `publicationCategoryBag` values shown in the API sample at line 340 of `pfw-search.txt`.

---

## 29. ENHANCEMENT: Facet Results Not Formatted

### What the API supports

The POST body accepts `facets`:
```json
"facets": ["applicationMetaData.applicationTypeLabelName", "applicationMetaData.entityStatusData.businessEntityStatusCategory"]
```

And the response includes a `facets` field (defined at `api.ts` line 193: `facets?: any[]`).

### Current behavior

The `--facets` option exists but:
1. Facets are not displayed in table output
2. The `facets` response field is typed as `any[]`
3. No formatter exists for facets

### Recommendation

Add facet display after search results:
```
4,327 results found

Facets:
  applicationTypeLabelName:
    Utility: 3,891
    Design: 436

  businessEntityStatusCategory:
    Regular Undiscounted: 2,100
    Small Entity: 1,500
    Micro Entity: 727
```

---

## 30. ENHANCEMENT: Add `app summary` Command for Agent-Optimized Single-App Overview

Agents frequently need a quick, structured overview of a patent application. Create a command that fetches metadata + a few key details and outputs a concise, parseable summary:

```
$ uspat app summary 16123456 --json
{
  "applicationNumber": "16123456",
  "patentNumber": "12000000",
  "title": "WIRELESS DEVICE FOR...",
  "status": "Patented Case",
  "filingDate": "2022-01-15",
  "grantDate": "2024-06-04",
  "applicant": "Acme Corp",
  "examiner": "SMITH, JOHN",
  "artUnit": "2612",
  "cpc": ["H04W4/00", "H04B1/00"],
  "ptaDays": 45,
  "lastUpdated": "2025-01-29T23:32:15"
}
```

This flattens the nested structure into a single-level object optimized for agent consumption.

---

## 31. ENHANCEMENT: Support Batch Application Lookups

Agents often need data for multiple applications. Add:

```bash
uspat app get 16123456 16123457 16123458
# or from a file
uspat app get --from-file app_numbers.txt
```

The search endpoint can handle this via a query like:
```
applicationNumberText:(16123456 OR 16123457 OR 16123458)
```

---

## 32. ENHANCEMENT: Add `--quiet` / `-q` Flag for Scriptable Output

Several commands output headers, progress messages, and chrome (colors, box-drawing chars) that are hard to parse in scripts. Add `--quiet` to suppress everything except the data.

---

## 33. ENHANCEMENT: `app` Subcommands Missing Format Flags

### Current behavior (`src/commands/app.ts`)

These subcommands only output raw JSON with no format option:
- `app attorney` (line 126-133)
- `app adjustment` / `app pta` (line 135-144)
- `app foreign-priority` / `app fp` (line 146-155)
- `app associated-docs` / `app xml` (line 157-166)

All other subcommands offer `-f, --format` with table rendering.

### Recommendation

Add table formatters for all subcommands. At minimum:
- `formatAttorneyTable()` showing attorney names, registration numbers, and active status
- `formatPTATable()` showing delay breakdown
- `formatForeignPriorityTable()` showing country, filing date, application number

---

## Summary: Priority Implementation Order

### Phase 1 -- Critical (Unblock Agent Workflows)
1. Wire up POST search body construction (Gap #1)
2. Add auto-pagination with `--all` (Gap #5)
3. Expose CSV/download export (Gap #6)
4. Fix sort parsing (Gap #7)
5. Fix filter/facet parsing for POST (Gap #3)
6. Parse fields as array, add presets (Gap #4)

### Phase 2 -- Major (Improve Agent Efficiency)
7. Add missing shorthands: `--examiner`, `--art-unit`, `--assignee`, `--docket`, `--pub-number`, `--customer-number` (Gap #8, #17)
8. Implement `compact`, `ids`, `tsv`, `csv` output formats (Gap #11, #27)
9. Fix assignee/assignor property names in formatter (Gap #20, #21)
10. Add 429 retry logic in client (Gap #15)
11. Type all `any` returns and `any[]` bags (Gap #13, #14)
12. Add date range flags for grant/publication dates (Gap #2)
13. Fix `app get` vs `app meta` redundancy (Gap #9)
14. Surface all response fields in formatters (Gap #10)

### Phase 3 -- Minor + Enhancements
15. Add `--count-only` (Gap #26)
16. Add `--granted` / `--pending` convenience filters (Gap #28)
17. Add `--verbose` / `-v` for full field display (Gap #10)
18. Format facet results (Gap #29)
19. Add `app summary` flattened output (Gap #30)
20. Support batch lookups (Gap #31)
21. Add PTA and document metadata to detail formatter (Gap #22, #23, #24)
22. Add formatters for attorney, PTA, foreign priority (Gap #33)
23. Add `--quiet` / `-q` (Gap #32)
24. Smart `--status` handling (numeric vs text) (Gap #25)
25. Validate `--type` choices (Gap #19)
26. Fix query value quoting (Gap #18)
27. Handle download redirects with re-applied headers (Gap #16)

