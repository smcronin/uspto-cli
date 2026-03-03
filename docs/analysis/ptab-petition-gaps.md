# PTAB & Petition Decision Gap Analysis

Comparison of USPTO Open Data Portal API documentation against the current CLI implementation.

**Files analyzed:**
- API docs: `docs/raw/ptab-trials-*.txt`, `docs/raw/ptab-appeals-*.txt`, `docs/raw/petition-*.txt`
- CLI code: `src/commands/ptab.ts`, `src/commands/petition.ts`
- API client: `src/api/client.ts`
- Types: `src/types/api.ts`
- Formatters: `src/utils/format.ts`

---

## 1. PTAB Trials -- Proceedings

### 1A. Missing search filter flags on `ptab search`

The current CLI exposes `--type`, `--patent`, `--owner`, `--petitioner`. The API supports all fields as query params. Missing agent-friendly shorthand flags:

| API Field Path | Suggested Flag | Purpose |
|---|---|---|
| `trialMetaData.trialStatusCategory` | `--status <status>` | Filter by status (Instituted, Terminated, FWD Entered, etc.) |
| `patentOwnerData.applicationNumberText` | `--app <number>` | Filter by application number |
| `patentOwnerData.inventorName` | `--inventor <name>` | Filter by inventor on challenged patent |
| `patentOwnerData.counselName` | `--po-counsel <name>` | Filter by patent owner counsel |
| `regularPetitionerData.counselName` | `--pet-counsel <name>` | Filter by petitioner counsel |
| `patentOwnerData.technologyCenterNumber` | `--tc <number>` | Filter by technology center |
| `patentOwnerData.groupArtUnitNumber` | `--art-unit <number>` | Filter by art unit |
| `trialMetaData.petitionFilingDate` | `--filed-after <date>` / `--filed-before <date>` | Date range on petition filing |
| `trialMetaData.institutionDecisionDate` | `--instituted-after <date>` | Filter by institution decision date |
| `trialMetaData.terminationDate` | `--terminated-after <date>` | Filter by termination date |

**Code-level recommendation for `src/commands/ptab.ts` -- `search` command:**

```typescript
// Add these options after existing ones (around line 22):
.option("--status <status>", "Trial status: Instituted, Terminated, etc.")
.option("--app <number>", "Application number")
.option("--inventor <name>", "Inventor name")
.option("--po-counsel <name>", "Patent owner counsel name")
.option("--pet-counsel <name>", "Petitioner counsel name")
.option("--tc <number>", "Technology center number")
.option("--art-unit <number>", "Art unit number")
.option("--filed-after <date>", "Petition filed after date (YYYY-MM-DD)")
.option("--filed-before <date>", "Petition filed before date (YYYY-MM-DD)")

// Add these query builders in the action handler (around line 28-33):
if (opts.status) parts.push(`trialMetaData.trialStatusCategory:"${opts.status}"`);
if (opts.app) parts.push(`patentOwnerData.applicationNumberText:${opts.app}`);
if (opts.inventor) parts.push(`patentOwnerData.inventorName:${opts.inventor.includes(" ") ? `"${opts.inventor}"` : opts.inventor}`);
if (opts.poCounsel) parts.push(`patentOwnerData.counselName:${opts.poCounsel.includes(" ") ? `"${opts.poCounsel}"` : opts.poCounsel}`);
if (opts.petCounsel) parts.push(`regularPetitionerData.counselName:${opts.petCounsel.includes(" ") ? `"${opts.petCounsel}"` : opts.petCounsel}`);
if (opts.tc) parts.push(`patentOwnerData.technologyCenterNumber:${opts.tc}`);
if (opts.artUnit) parts.push(`patentOwnerData.groupArtUnitNumber:${opts.artUnit}`);
if (opts.filedAfter) parts.push(`trialMetaData.petitionFilingDate:[${opts.filedAfter} TO *]`);
if (opts.filedBefore) parts.push(`trialMetaData.petitionFilingDate:[* TO ${opts.filedBefore}]`);
```

### 1B. Missing response fields not displayed in table format

`formatProceedingTable` in `src/utils/format.ts` (line 190) shows: Trial #, Type, Status, Patent #, Owner, Petitioner. The API response also provides these fields that are not displayed:

- `trialMetaData.petitionFilingDate` -- when the petition was filed
- `trialMetaData.accordedFilingDate` -- official accorded filing date
- `trialMetaData.institutionDecisionDate` -- institution decision date
- `trialMetaData.latestDecisionDate` -- most recent decision date
- `trialMetaData.terminationDate` -- termination date
- `trialRecordIdentifier` -- record identifier
- `patentOwnerData.inventorName` -- inventor name
- `patentOwnerData.counselName` -- patent owner counsel
- `patentOwnerData.technologyCenterNumber` -- technology center
- `patentOwnerData.groupArtUnitNumber` -- art unit
- `patentOwnerData.grantDate` -- grant date of challenged patent
- `regularPetitionerData.counselName` -- petitioner counsel
- `respondentData.*` -- entire respondent data block
- `derivationPetitionerData.*` -- entire derivation petitioner data block
- `fileDownloadURI` -- document download link

**Code-level recommendation:** Add a `--verbose` / `-v` flag to `ptab search` that uses an expanded table or detail-per-record output:

```typescript
// In src/utils/format.ts, add:
export function formatProceedingDetail(p: ProceedingData): string {
  const m = p.trialMetaData;
  const po = p.patentOwnerData;
  const pet = p.regularPetitionerData;
  const lines = [
    "",
    chalk.bold.white(`  ${p.trialNumber}`),
    "",
    `  ${chalk.gray("Type:")}              ${m?.trialTypeCode || "-"}`,
    `  ${chalk.gray("Status:")}            ${m?.trialStatusCategory || "-"}`,
    `  ${chalk.gray("Petition Filed:")}    ${m?.petitionFilingDate || "-"}`,
    `  ${chalk.gray("Accorded Filed:")}    ${m?.accordedFilingDate || "-"}`,
    `  ${chalk.gray("Institution Date:")}  ${m?.institutionDecisionDate || "-"}`,
    `  ${chalk.gray("Latest Decision:")}   ${m?.latestDecisionDate || "-"}`,
    `  ${chalk.gray("Termination Date:")}  ${m?.terminationDate || "-"}`,
    "",
    chalk.bold("  Patent Owner:"),
    `    ${chalk.gray("Patent #:")}    ${po?.patentNumber || "-"}`,
    `    ${chalk.gray("App #:")}       ${po?.applicationNumberText || "-"}`,
    `    ${chalk.gray("Owner:")}       ${po?.patentOwnerName || "-"}`,
    `    ${chalk.gray("Inventor:")}    ${po?.inventorName || "-"}`,
    `    ${chalk.gray("Counsel:")}     ${po?.counselName || "-"}`,
    `    ${chalk.gray("Grant Date:")} ${po?.grantDate || "-"}`,
    `    ${chalk.gray("TC/AU:")}       ${po?.technologyCenterNumber || "-"}/${po?.groupArtUnitNumber || "-"}`,
    `    ${chalk.gray("RPI:")}         ${po?.realPartyInInterestName || "-"}`,
    "",
    chalk.bold("  Petitioner:"),
    `    ${chalk.gray("RPI:")}         ${pet?.realPartyInInterestName || "-"}`,
    `    ${chalk.gray("Counsel:")}     ${pet?.counselName || "-"}`,
    "",
  ];
  return lines.join("\n");
}
```

### 1C. Missing `trialRecordIdentifier` and `fileDownloadURI` in ProceedingData type

In `src/types/api.ts`, the `ProceedingData` interface (line 279) is missing:

```typescript
export interface ProceedingData {
  trialNumber: string;
  trialRecordIdentifier: string;       // MISSING -- add this
  lastModifiedDateTime: string;
  trialMetaData: TrialMetaData;
  patentOwnerData: PartyData;
  regularPetitionerData: PartyData;     // WRONG TYPE -- should be full PartyData, not {counselName; realPartyInInterestName}
  respondentData: PartyData;
  derivationPetitionerData: PartyData;
}
```

**Bug:** The `regularPetitionerData` type on line 284 is typed as `{ counselName: string; realPartyInInterestName: string }` but the API returns a **full** `PartyData` object with `applicationNumberText`, `grantDate`, `groupArtUnitNumber`, `inventorName`, `patentNumber`, `patentOwnerName`, `technologyCenterNumber`, plus `realPartyInInterestName` and `counselName`. The sample response confirms all these fields are present for `regularPetitionerData`. This truncated type silently discards data.

**Fix in `src/types/api.ts` line 284:**
```typescript
// BEFORE (wrong):
regularPetitionerData: { counselName: string; realPartyInInterestName: string };

// AFTER (correct):
regularPetitionerData: PartyData;
```

### 1D. Missing `fileDownloadURI` in `TrialMetaData`

The API response sample shows `fileDownloadURI` at the `trialMetaData` level in some responses. Add to `TrialMetaData` interface:

```typescript
export interface TrialMetaData {
  // ... existing fields ...
  fileDownloadURI?: string;  // MISSING -- add this
}
```

---

## 2. PTAB Trials -- Download Proceedings Search Results

### 2A. Completely missing download endpoint

**API endpoint:** `GET /api/v1/patent/trials/proceedings/search/download`

This endpoint returns search results as a downloadable CSV or JSON stream. It accepts the same query params as the search endpoint, plus a `format` parameter. The CLI has no command to call it.

**Missing client method in `src/api/client.ts`:**

```typescript
async downloadProceedings(
  query?: string,
  format: "json" | "csv" = "json",
  opts: { limit?: number; offset?: number; sort?: string } = {}
): Promise<any> {
  const params: Record<string, string> = { format };
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;
  return this.request("GET", "/api/v1/patent/trials/proceedings/search/download", { params });
}
```

**Missing CLI subcommand in `src/commands/ptab.ts`:**

```typescript
ptab
  .command("download-proceedings")
  .alias("dl-proc")
  .description("Download PTAB proceedings search results as CSV or JSON")
  .argument("[query]", "Search query")
  .option("--csv", "Download as CSV (default: JSON)")
  .option("-l, --limit <n>", "Max results", "100")
  .option("-o, --output <path>", "Output file path")
  .action(async (query, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const format = opts.csv ? "csv" : "json";
    const result = await client.downloadProceedings(query, format, {
      limit: parseInt(opts.limit),
    });
    if (opts.output) {
      const { writeFile } = await import("fs/promises");
      await writeFile(opts.output, typeof result === "string" ? result : JSON.stringify(result, null, 2));
      console.log(`Written to ${opts.output}`);
    } else {
      console.log(typeof result === "string" ? result : JSON.stringify(result, null, 2));
    }
  });
```

---

## 3. PTAB Trials -- Decisions

### 3A. Missing search filter flags on `ptab decisions`

The `decisions` command (line 62-79 in `ptab.ts`) only has `--trial` and `--limit`. The API supports these searchable fields that should have shorthand flags:

| API Field Path | Suggested Flag | Purpose |
|---|---|---|
| `decisionData.trialOutcomeCategory` | `--outcome <outcome>` | e.g., "Denied", "Institution Granted", "Final Written Decision" |
| `decisionData.decisionTypeCategory` | `--decision-type <type>` | e.g., "Final Written Decision", "Decision" |
| `decisionData.appealOutcomeCategory` | `--appeal-outcome <outcome>` | e.g., "Affirmed", "Reversed" |
| `trialTypeCode` | `--type <code>` | IPR, PGR, CBM, DER |
| `trialMetaData.trialStatusCategory` | `--status <status>` | Current trial status |
| `documentData.filingPartyCategory` | `--filing-party <party>` | Who filed the document |
| `documentData.documentTypeDescriptionText` | `--doc-type <type>` | Document type description |

**Code-level recommendation for `src/commands/ptab.ts` -- `decisions` command:**

```typescript
ptab
  .command("decisions")
  .description("Search or get trial decisions")
  .argument("[query]", "Search query or trial number")
  .option("-l, --limit <n>", "Max results", "25")
  .option("-o, --offset <n>", "Starting offset", "0")
  .option("--trial <number>", "Get decisions for a specific trial")
  .option("--type <code>", "Trial type: IPR, PGR, CBM, DER")
  .option("--outcome <outcome>", "Decision outcome (Denied, Institution Granted, etc.)")
  .option("--decision-type <type>", "Decision type category")
  .option("--status <status>", "Trial status")
  .option("--patent <number>", "Patent number")
  .option("-s, --sort <field>", "Sort field and order")
  .option("-f, --format <fmt>", "Output format: table, json", "json")
  .action(async (query, opts) => {
    // ...build query from opts...
  });
```

### 3B. Missing `--offset` and `--sort` on decisions search

The client method `searchTrialDecisions` (line 252 in `client.ts`) accepts `offset` but the CLI command does not expose `-o, --offset` or `-s, --sort`. Add those options.

### 3C. Missing decision-specific table formatter

There is no `formatDecisionTable` function. Decisions are always output as raw JSON. A table formatter should display:

- Trial number, trial type, status
- Decision type category, outcome, issue date
- Document name, filing date, filing party
- Statute/rule bag

```typescript
// In src/utils/format.ts, add:
export function formatDecisionTable(decisions: TrialDocument[]): string {
  if (!decisions?.length) return chalk.yellow("No decisions found.");

  const table = new Table({
    head: [
      chalk.cyan("Trial #"),
      chalk.cyan("Type"),
      chalk.cyan("Outcome"),
      chalk.cyan("Decision Type"),
      chalk.cyan("Issue Date"),
      chalk.cyan("Document"),
    ],
    colWidths: [18, 6, 22, 24, 13, 25],
    wordWrap: true,
  });

  for (const d of decisions) {
    table.push([
      d.trialNumber || "",
      d.trialTypeCode || "",
      d.decisionData?.trialOutcomeCategory || "-",
      d.decisionData?.decisionTypeCategory || "-",
      d.decisionData?.decisionIssueDate || "-",
      (d.documentData?.documentName || "").substring(0, 30),
    ]);
  }

  return table.toString();
}
```

### 3D. Missing download decisions endpoint

**API endpoint:** `GET /api/v1/patent/trials/decisions/search/download`

Same pattern as proceedings download. Not implemented.

**Missing client method:**

```typescript
async downloadTrialDecisions(
  query?: string,
  format: "json" | "csv" = "json",
  opts: { limit?: number; offset?: number; sort?: string } = {}
): Promise<any> {
  const params: Record<string, string> = { format };
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;
  return this.request("GET", "/api/v1/patent/trials/decisions/search/download", { params });
}
```

### 3E. Missing decisions-by-document-identifier endpoint

**API endpoint:** `GET /api/v1/patent/trials/decisions/{documentIdentifier}`

The client has `getTrialDecision(documentId)` on line 260 which calls this correctly. However, the CLI `decisions` command has no way to invoke it. The `--trial` flag calls `getTrialDecisions(trialNumber)` which goes to `/trials/{trialNumber}/decisions`. There is no `--doc-id` flag to look up a specific decision by its document identifier.

**Add to `ptab decisions` command:**

```typescript
.option("--doc-id <id>", "Get a specific decision by document identifier")
// In action:
if (opts.docId) {
  const result = await client.getTrialDecision(opts.docId);
  console.log(formatOutput(result, opts.format));
} else if (opts.trial) {
  // ...existing
}
```

### 3F. DecisionData type is incomplete

In `src/types/api.ts` line 311, `DecisionData` is missing the `appealOutcomeCategory` field that the API returns:

```typescript
export interface DecisionData {
  statuteAndRuleBag: string | string[];   // Can be string OR array -- API shows both
  decisionIssueDate: string;
  decisionTypeCategory: string;
  issueTypeBag: string | string[];        // Can be string OR array -- API shows both
  trialOutcomeCategory: string;
  appealOutcomeCategory?: string;         // MISSING -- add this
}
```

Also, `statuteAndRuleBag` and `issueTypeBag` should be typed as `string | string[]` because the API sometimes returns a single string and sometimes an array. Currently typed as just `string`.

### 3G. Facets not exposed

The decisions search response includes a `facets` array with `trialTypeCategory`, `trialStatusCategory`, and `statuteAndRuleBag` aggregations. The `TrialDocumentResponse` type (line 333) does model `facets` but it is typed as `FacetValue[]` which is wrong -- the actual shape is:

```typescript
export interface TrialDecisionFacets {
  trialTypeCategory?: { name: string; quantity: number }[];
  trialStatusCategory?: { name: string; quantity: number }[];
  statuteAndRuleBag?: { name: string; quantity: number }[];
}

// Update TrialDocumentResponse:
export interface TrialDocumentResponse {
  count: number;
  facets?: TrialDecisionFacets[];  // Array of facet groups
  patentTrialDocumentDataBag?: TrialDocument[];
  patentTrialDecisionDataBag?: TrialDocument[];
}
```

Add a `--facets` flag to show faceted counts in CLI output.

---

## 4. PTAB Appeals -- MAJOR GAPS

### 4A. Missing search filter flags on `ptab appeals`

The `appeals` command (line 104-121) only has `--appeal` and `--limit`. The API supports many searchable fields:

| API Field Path | Suggested Flag | Purpose |
|---|---|---|
| `decisionData.appealOutcomeCategory` | `--outcome <outcome>` | "Affirmed", "Reversed", "Affirmed-in-Part" |
| `decisionData.decisionTypeCategory` | `--decision-type <type>` | "Decision", "Rehearing" |
| `appellantData.applicationNumberText` | `--app <number>` | Application number |
| `appellantData.patentNumber` | `--patent <number>` | Patent number |
| `appellantData.inventorName` | `--inventor <name>` | Inventor name |
| `appellantData.technologyCenterNumber` | `--tc <number>` | Technology center |
| `appellantData.groupArtUnitNumber` | `--art-unit <number>` | Art unit |
| `appellantData.realPartyInInterestName` | `--rpi <name>` | Real party in interest |
| `appellantData.counselName` | `--counsel <name>` | Counsel name |
| `appealMetaData.appealFilingDate` | `--filed-after <date>` | Appeal filing date range |
| `decisionData.decisionIssueDate` | `--decided-after <date>` | Decision date range |
| `thirdPartyRequesterData.thirdPartyName` | `--third-party <name>` | Third party requester (reexam appeals) |
| `decisionData.issueTypeBag` | `--issue-type <type>` | Statutory section (e.g., 101, 102, 103, 112) |
| `decisionData.statuteAndRuleBag` | `--statute <statute>` | Statute/rule filter |

**Code-level recommendation:**

```typescript
ptab
  .command("appeals")
  .description("Search or get appeal decisions")
  .argument("[query]", "Search query")
  .option("-l, --limit <n>", "Max results", "25")
  .option("-o, --offset <n>", "Starting offset", "0")
  .option("-s, --sort <field>", "Sort field and order")
  .option("--appeal <number>", "Get decisions for a specific appeal number")
  .option("--doc-id <id>", "Get a specific appeal decision by document identifier")
  .option("--outcome <outcome>", "Appeal outcome: Affirmed, Reversed, Affirmed-in-Part")
  .option("--decision-type <type>", "Decision type: Decision, Rehearing")
  .option("--app <number>", "Application number")
  .option("--patent <number>", "Patent number")
  .option("--inventor <name>", "Inventor name")
  .option("--tc <number>", "Technology center number")
  .option("--art-unit <number>", "Art unit number")
  .option("--rpi <name>", "Real party in interest name")
  .option("--counsel <name>", "Counsel name")
  .option("--filed-after <date>", "Appeal filed after date (YYYY-MM-DD)")
  .option("--decided-after <date>", "Decision issued after date (YYYY-MM-DD)")
  .option("--issue-type <type>", "Statutory issue type (101, 102, 103, 112)")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
  .action(async (query, opts) => {
    const client = createClient({ debug: program.opts().debug });

    if (opts.docId) {
      const result = await client.getAppealDecision(opts.docId);
      console.log(formatOutput(result, opts.format));
      return;
    }

    if (opts.appeal) {
      const result = await client.getAppealDecisions(opts.appeal);
      console.log(formatOutput(result, opts.format));
      return;
    }

    const parts: string[] = [];
    if (query) parts.push(query);
    if (opts.outcome) parts.push(`decisionData.appealOutcomeCategory:"${opts.outcome}"`);
    if (opts.decisionType) parts.push(`decisionData.decisionTypeCategory:"${opts.decisionType}"`);
    if (opts.app) parts.push(`appellantData.applicationNumberText:${opts.app}`);
    if (opts.patent) parts.push(`appellantData.patentNumber:${opts.patent}`);
    if (opts.inventor) parts.push(`appellantData.inventorName:${opts.inventor.includes(" ") ? `"${opts.inventor}"` : opts.inventor}`);
    if (opts.tc) parts.push(`appellantData.technologyCenterNumber:${opts.tc}`);
    if (opts.artUnit) parts.push(`appellantData.groupArtUnitNumber:${opts.artUnit}`);
    if (opts.rpi) parts.push(`appellantData.realPartyInInterestName:${opts.rpi.includes(" ") ? `"${opts.rpi}"` : opts.rpi}`);
    if (opts.counsel) parts.push(`appellantData.counselName:${opts.counsel.includes(" ") ? `"${opts.counsel}"` : opts.counsel}`);
    if (opts.filedAfter) parts.push(`appealMetaData.appealFilingDate:[${opts.filedAfter} TO *]`);
    if (opts.decidedAfter) parts.push(`decisionData.decisionIssueDate:[${opts.decidedAfter} TO *]`);
    if (opts.issueType) parts.push(`decisionData.issueTypeBag:${opts.issueType}`);

    const q = parts.join(" AND ") || undefined;
    const result = await client.searchAppealDecisions(q, {
      limit: parseInt(opts.limit),
      offset: parseInt(opts.offset),
    });

    if (opts.format === "json") {
      console.log(formatOutput(result, "json"));
    } else {
      console.log(`\n${result.count} appeal decisions found\n`);
      console.log(formatAppealTable(result.patentAppealDataBag));
    }
  });
```

### 4B. Missing appeals download endpoint

**API endpoint:** `GET /api/v1/patent/appeals/decisions/search/download`

Not implemented in client or CLI. Supports CSV and JSON download of search results.

**Missing client method:**

```typescript
async downloadAppealDecisions(
  query?: string,
  format: "json" | "csv" = "json",
  opts: { limit?: number; offset?: number; sort?: string } = {}
): Promise<any> {
  const params: Record<string, string> = { format };
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;
  return this.request("GET", "/api/v1/patent/appeals/decisions/search/download", { params });
}
```

### 4C. Missing appeals table formatter

No `formatAppealTable` function exists. Appeals results always fall through to raw JSON.

```typescript
// In src/utils/format.ts, add:
export function formatAppealTable(appeals: AppealData[]): string {
  if (!appeals?.length) return chalk.yellow("No appeal decisions found.");

  const table = new Table({
    head: [
      chalk.cyan("Appeal #"),
      chalk.cyan("Outcome"),
      chalk.cyan("Decision Type"),
      chalk.cyan("Issue Date"),
      chalk.cyan("App #"),
      chalk.cyan("Inventor"),
    ],
    colWidths: [14, 20, 16, 13, 12, 25],
    wordWrap: true,
  });

  for (const a of appeals) {
    table.push([
      a.appealNumber || "",
      a.decisionData?.appealOutcomeCategory || "-",
      a.decisionData?.decisionTypeCategory || "-",
      a.decisionData?.decisionIssueDate || "-",
      a.appellantData?.applicationNumberText || "",
      (a.appellantData?.inventorName || "").substring(0, 30),
    ]);
  }

  return table.toString();
}
```

### 4D. AppealData type inconsistencies and missing fields

The `AppealData` interface in `src/types/api.ts` (line 340) has several problems:

1. **`appelantData` vs `appellantData`**: The API docs show BOTH spellings depending on the endpoint. The search endpoint uses `appellantData` (correct English), but the by-number and download endpoints use `appelantData` (typo in API). The current type uses `appelantData` which is the misspelled version. The code must handle both:

```typescript
export interface AppealData {
  appealNumber: string;
  appealDocumentCategory: string;
  lastModifiedDateTime: string;
  appealMetaData: {
    docketNoticeMailedDate: string;
    appealFilingDate: string;
    appealLastModifiedDate: string;
    appealLastModifiedDateTime?: string;  // MISSING -- present in some responses
    applicationTypeCategory: string;
    fileDownloadURI: string;
  };
  // API uses BOTH spellings depending on the endpoint
  appellantData?: AppellantData;
  appelantData?: AppellantData;    // Misspelled variant from some endpoints
  decisionData?: AppealDecisionData;
  documentData?: AppealDocumentData;
  thirdPartyRequesterData?: { thirdPartyName: string | null };
  requestorData?: { thirdPartyName: string | null };  // Alternate key name in some responses
}
```

2. **Missing fields in appellant data**: The current type is missing several fields from the API:

```typescript
export interface AppellantData {
  applicationNumberText: string;
  counselName: string;
  groupArtUnitNumber: string;
  inventorName: string;
  realPartyInInterestName: string;  // Sometimes 'realPartyName' in API responses
  patentNumber: string;
  patentOwnerName: string;
  publicationDate: string;          // MISSING from current type
  publicationNumber: string;        // MISSING from current type
  technologyCenterNumber: string;   // Sometimes 'techCenterNumber' in API responses
  grantDate?: string;               // MISSING -- present in search endpoint response
}
```

3. **Missing `decisionData` block**: The current `AppealData` type does not include the `decisionData` object, which contains:

```typescript
export interface AppealDecisionData {
  appealOutcomeCategory: string;
  decisionIssueDate: string;
  decisionTypeCategory: string;
  issueTypeBag: string[];
  statuteAndRuleBag: string[];
}
```

This is a critical omission -- the decision data is arguably the most important part of an appeal response.

4. **`documentData` type reuse**: The current type reuses `TrialDocumentData` for appeals, but the appeal document schema differs (uses `documentTypeCategory` vs `documentTypeDescriptionText`, and `downloadURI` vs `fileDownloadURI` in some response variants).

### 4E. Missing `--offset` and `--sort` on appeals search client method

`searchAppealDecisions` in `client.ts` line 288 accepts `offset` but not `sort`. The API supports sorting. Add:

```typescript
async searchAppealDecisions(query?: string, opts: { limit?: number; offset?: number; sort?: string } = {}): Promise<AppealDecisionResponse> {
  const params: Record<string, string> = {};
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;  // MISSING -- add this
  return this.request<AppealDecisionResponse>("GET", "/api/v1/patent/appeals/decisions/search", { params });
}
```

---

## 5. PTAB Interferences -- ENTIRELY MISSING API DOCUMENTATION

### 5A. No raw API documentation captured

There are no `docs/raw/ptab-interference-*.txt` files. The PTAB Interferences API has 4 endpoints based on the sidebar navigation in the existing docs:

1. `GET /api/v1/patent/interferences/decisions/search` -- Search Interferences
2. `GET /api/v1/patent/interferences/decisions/search/download` -- Download Search Results
3. `GET /api/v1/patent/interferences/decisions/{documentIdentifier}` -- Search by Document Identifier
4. `GET /api/v1/patent/interferences/{interferenceNumber}/decisions` -- Search by Interference Number

The CLI already has stubs for endpoints 1, 3, and 4 in `client.ts` (lines 306-320) and a basic `interferences` subcommand in `ptab.ts` (lines 125-143). However:

### 5B. Missing interference download endpoint

**API endpoint:** `GET /api/v1/patent/interferences/decisions/search/download`

Not implemented in client or CLI.

```typescript
async downloadInterferenceDecisions(
  query?: string,
  format: "json" | "csv" = "json",
  opts: { limit?: number; offset?: number; sort?: string } = {}
): Promise<any> {
  const params: Record<string, string> = { format };
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;
  return this.request("GET", "/api/v1/patent/interferences/decisions/search/download", { params });
}
```

### 5C. Missing search filter flags on `ptab interferences`

The CLI only has `--interference` and `--limit`. Should add:

```typescript
.option("--doc-id <id>", "Get by document identifier")
.option("-o, --offset <n>", "Starting offset", "0")
.option("-s, --sort <field>", "Sort field and order")
.option("-f, --format <fmt>", "Output format: table, json", "table")
```

### 5D. InterferenceData type needs verification

The `InterferenceData` interface has `decisionDocumentData: any` which is un-typed. This should be properly typed once the interference API documentation is captured. The expected shape is likely similar to the trial/appeal decision data.

### 5E. Missing interference table formatter

No table formatting for interference search results.

---

## 6. Petition Decisions

### 6A. Missing search filter flags on `petition search`

The current `petition search` command (line 12-60 in `petition.ts`) exposes `--office` and `--decision` only. The API has many more searchable fields:

| API Field Path | Suggested Flag | Purpose |
|---|---|---|
| `applicationNumberText` | `--app <number>` | Application number |
| `patentNumber` | `--patent <number>` | Patent number |
| `firstApplicantName` | `--applicant <name>` | Applicant name |
| `inventionTitle` | `--title <title>` | Invention title |
| `decisionPetitionTypeCode` | `--petition-type <code>` | 3-digit petition type code |
| `decisionPetitionTypeCodeDescriptionText` | `--petition-desc <text>` | Petition type description text |
| `technologyCenter` | `--tc <number>` | Technology center |
| `groupArtUnitNumber` | `--art-unit <number>` | Art unit |
| `businessEntityStatusCategory` | `--entity <status>` | Entity status (Small, Micro, Regular Undiscounted) |
| `prosecutionStatusCodeDescriptionText` | `--prosecution-status <text>` | Prosecution status |
| `courtActionIndicator` | `--court-action` | Filter for court actions |
| `ruleBag` | `--rule <rule>` | CFR rule (e.g., "37 CFR 1.137") |
| `statuteBag` | `--statute <statute>` | Statute (e.g., "35 USC 111") |
| `decisionDate` | `--decided-after <date>` / `--decided-before <date>` | Decision date range |
| `petitionMailDate` | `--mailed-after <date>` | Petition mail date |
| `petitionIssueConsideredTextBag` | `--issue <text>` | Issue considered |

**Code-level recommendation for `src/commands/petition.ts`:**

```typescript
petition
  .command("search")
  .description("Search petition decisions")
  .argument("[query]", "Search query")
  .option("-l, --limit <n>", "Max results", "25")
  .option("-o, --offset <n>", "Starting offset", "0")
  .option("-s, --sort <field>", "Sort field and order")
  .option("--office <name>", "Deciding office filter")
  .option("--decision <type>", "Decision type: GRANTED, DENIED, DISMISSED")
  .option("--app <number>", "Application number")
  .option("--patent <number>", "Patent number")
  .option("--applicant <name>", "First applicant name")
  .option("--title <title>", "Invention title keyword")
  .option("--petition-type <code>", "Petition type code (3-digit)")
  .option("--tc <number>", "Technology center")
  .option("--art-unit <number>", "Art unit")
  .option("--entity <status>", "Entity status: Small, Micro, Regular Undiscounted")
  .option("--rule <rule>", "CFR rule filter")
  .option("--statute <statute>", "Statute filter")
  .option("--decided-after <date>", "Decision date after (YYYY-MM-DD)")
  .option("--decided-before <date>", "Decision date before (YYYY-MM-DD)")
  .option("--court-action", "Only show decisions with court action")
  .option("--issue <text>", "Issue considered text")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
```

### 6B. Missing petition decision response fields not displayed

The table formatter in `petition.ts` (inline, lines 42-58) only shows: App #, Patent #, Decision, Type, Date, Applicant. Many fields are discarded:

- `inventionTitle` -- title of the invention
- `technologyCenter` -- technology center
- `groupArtUnitNumber` -- art unit
- `businessEntityStatusCategory` -- entity status
- `prosecutionStatusCodeDescriptionText` -- prosecution status
- `petitionIssueConsideredTextBag` -- issues considered (array)
- `ruleBag` -- applicable rules (array)
- `statuteBag` -- applicable statutes (array)
- `courtActionIndicator` -- whether court action taken
- `actionTakenByCourtName` -- court name
- `inventorBag` -- inventors (array)
- `customerNumber` -- customer number
- `firstInventorToFileIndicator` -- FITF flag
- `decisionPetitionTypeCode` -- numeric petition type code
- `prosecutionStatusCode` -- numeric prosecution status code

**Recommendation:** Move the inline table formatter to `src/utils/format.ts` as `formatPetitionTable` and add a `formatPetitionDetail` function for detailed single-record view:

```typescript
// In src/utils/format.ts:
export function formatPetitionTable(decisions: PetitionDecision[]): string {
  if (!decisions?.length) return chalk.yellow("No petition decisions found.");

  const table = new Table({
    head: [
      chalk.cyan("App #"),
      chalk.cyan("Patent #"),
      chalk.cyan("Decision"),
      chalk.cyan("Type"),
      chalk.cyan("Date"),
      chalk.cyan("Applicant"),
      chalk.cyan("Issue"),
    ],
    colWidths: [14, 12, 10, 28, 13, 22, 25],
    wordWrap: true,
  });

  for (const d of decisions) {
    table.push([
      d.applicationNumberText || "",
      d.patentNumber || "-",
      d.decisionTypeCode || "",
      (d.decisionPetitionTypeCodeDescriptionText || "").substring(0, 35),
      d.decisionDate || d.petitionMailDate || "",
      (d.firstApplicantName || "").substring(0, 25),
      (d.petitionIssueConsideredTextBag?.[0] || "").substring(0, 30),
    ]);
  }

  return table.toString();
}

export function formatPetitionDetail(d: PetitionDecision): string {
  const lines = [
    "",
    chalk.bold.white(`  Petition Decision: ${d.applicationNumberText}`),
    "",
    `  ${chalk.gray("Application #:")}    ${d.applicationNumberText || "-"}`,
    `  ${chalk.gray("Patent #:")}         ${d.patentNumber || "-"}`,
    `  ${chalk.gray("Invention Title:")}  ${d.inventionTitle || "-"}`,
    `  ${chalk.gray("Applicant:")}        ${d.firstApplicantName || "-"}`,
    `  ${chalk.gray("Inventors:")}        ${(d.inventorBag || []).join(", ") || "-"}`,
    `  ${chalk.gray("Entity Status:")}    ${d.businessEntityStatusCategory || "-"}`,
    "",
    `  ${chalk.gray("Decision:")}         ${d.decisionTypeCodeDescriptionText || d.decisionTypeCode || "-"}`,
    `  ${chalk.gray("Petition Type:")}    ${d.decisionPetitionTypeCodeDescriptionText || "-"} (${d.decisionPetitionTypeCode || "-"})`,
    `  ${chalk.gray("Decision Date:")}    ${d.decisionDate || "-"}`,
    `  ${chalk.gray("Mail Date:")}        ${d.petitionMailDate || "-"}`,
    `  ${chalk.gray("Deciding Office:")}  ${d.finalDecidingOfficeName || "-"}`,
    "",
    `  ${chalk.gray("TC / Art Unit:")}    ${d.technologyCenter || "-"} / ${d.groupArtUnitNumber || "-"}`,
    `  ${chalk.gray("Prosecution:")}      ${d.prosecutionStatusCodeDescriptionText || "-"}`,
    `  ${chalk.gray("AIA/FITF:")}         ${d.firstInventorToFileIndicator ?? "-"}`,
    "",
    `  ${chalk.gray("Issues:")}           ${(d.petitionIssueConsideredTextBag || []).join("; ") || "-"}`,
    `  ${chalk.gray("Rules:")}            ${(d.ruleBag || []).join(", ") || "-"}`,
    `  ${chalk.gray("Statutes:")}         ${(d.statuteBag || []).join(", ") || "-"}`,
    "",
    `  ${chalk.gray("Court Action:")}     ${d.courtActionIndicator ? `Yes - ${d.actionTakenByCourtName}` : "No"}`,
    `  ${chalk.gray("Record ID:")}        ${d.petitionDecisionRecordIdentifier || "-"}`,
    "",
  ];
  return lines.join("\n");
}
```

### 6C. PetitionDecision type missing fields

In `src/types/api.ts` line 394, the `PetitionDecision` interface is missing several fields from the API:

```typescript
export interface PetitionDecision {
  petitionDecisionRecordIdentifier: string;
  applicationNumberText: string;
  businessEntityStatusCategory: string;
  customerNumber: number;
  decisionDate: string;
  decisionPetitionTypeCode: number;
  decisionTypeCode: string;
  decisionPetitionTypeCodeDescriptionText: string;
  decisionTypeCodeDescriptionText: string;  // MISSING -- the API returns this (e.g., "DENIED")
  finalDecidingOfficeName: string;
  firstApplicantName: string;
  firstInventorToFileIndicator: boolean;
  groupArtUnitNumber: string;
  technologyCenter: string;
  inventionTitle: string;
  inventorBag: string[];
  courtActionIndicator: boolean;
  actionTakenByCourtName: string;           // MISSING -- add this
  patentNumber: string;
  petitionMailDate: string;
  prosecutionStatusCode: string;            // MISSING -- add this
  prosecutionStatusCodeDescriptionText: string;
  petitionIssueConsideredTextBag: string[];  // MISSING -- add this
  ruleBag: string[];
  statuteBag: string[];
  lastIngestionDateTime: string;            // MISSING -- add this
}
```

### 6D. Petition Decision Data Download (`petition get`) incomplete

The `petition get` command calls `getPetitionDecision(recordId, includeDocuments)` which goes to:
`GET /api/v1/petition/decisions/{petitionDecisionRecordIdentifier}?includeDocuments=true`

The API response includes a `documentBag` array when `includeDocuments=true`, with rich document metadata:

```typescript
export interface PetitionDocument {
  applicationNumberText: string;
  officialDate: string;
  documentIdentifier: string;
  documentCode: string;
  documentCodeDescriptionText: string;
  directionCategory: string;              // "INCOMING" or "OUTGOING"
  downloadOptionBag: PetitionDownloadOption[];
}

export interface PetitionDownloadOption {
  mimeTypeIdentifier: string;             // "PDF", "MS_WORD", "XML", "PNG"
  downloadUrl: string;
  pageTotalQuantity?: number;
}
```

**Issues with current implementation:**
1. The `getPetitionDecision` return type is `Promise<any>` (line 333 in `client.ts`) -- should be properly typed.
2. The CLI command outputs raw JSON with no formatting for the document data.
3. No ability to actually download the petition decision documents themselves.
4. The download URLs in the response use `api.test.uspto.gov` in the sample but should work with the production base URL.

**Add download support:**

```typescript
// In petition.ts, add a download subcommand:
petition
  .command("download")
  .description("Download petition decision document")
  .argument("<recordId>", "Petition decision record identifier (UUID)")
  .option("--format <fmt>", "Document format: PDF, MS_WORD, XML", "PDF")
  .option("-o, --output <path>", "Output file path")
  .action(async (recordId, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.getPetitionDecision(recordId, true);
    const decision = result.petitionDecisionDataBag?.[0];
    if (!decision?.documentBag?.length) {
      console.error("No documents found for this petition decision.");
      return;
    }
    for (const doc of decision.documentBag) {
      const option = doc.downloadOptionBag?.find(
        (o: any) => o.mimeTypeIdentifier === opts.format
      );
      if (option) {
        const filename = opts.output || `${decision.applicationNumberText}_${doc.documentCode}.${opts.format.toLowerCase() === "ms_word" ? "docx" : opts.format.toLowerCase()}`;
        await client.downloadDocument(option.downloadUrl, filename);
        console.log(`Downloaded: ${filename}`);
      }
    }
  });
```

---

## 7. Cross-Cutting Download Architecture Gap

### 7A. Five download endpoints exist but none are implemented

The API provides these download/export endpoints that produce streamable CSV or JSON:

| # | Endpoint | Status |
|---|---|---|
| 1 | `GET /api/v1/patent/trials/proceedings/search/download` | NOT IMPLEMENTED |
| 2 | `GET /api/v1/patent/trials/decisions/search/download` | NOT IMPLEMENTED |
| 3 | `GET /api/v1/patent/trials/documents/search/download` | NOT IMPLEMENTED (also not documented but follows pattern) |
| 4 | `GET /api/v1/patent/appeals/decisions/search/download` | NOT IMPLEMENTED |
| 5 | `GET /api/v1/patent/interferences/decisions/search/download` | NOT IMPLEMENTED |

All share the same query parameter interface as their corresponding search endpoints, with an additional `format` parameter (`csv` or `json`).

**Recommended architecture:** Add a generic download method pattern to the client:

```typescript
// In src/api/client.ts, add a generic download-search method:

private async downloadSearchResults(
  basePath: string,
  query?: string,
  format: "json" | "csv" = "json",
  opts: { limit?: number; offset?: number; sort?: string } = {}
): Promise<string | any> {
  const params: Record<string, string> = { format };
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;

  // For CSV, we need raw text response
  if (format === "csv") {
    await this.rateLimiter.waitForSlot();
    const url = `${this.config.baseUrl}${basePath}/download?${new URLSearchParams(params)}`;
    const response = await fetch(url, { headers: this.headers });
    this.rateLimiter.markRequestComplete();
    if (!response.ok) throw new Error(`Download failed: HTTP ${response.status}`);
    return response.text();
  }

  return this.request("GET", `${basePath}/download`, { params });
}

// Then expose specific methods:
async downloadProceedings(q?: string, fmt?: "json" | "csv", opts?: any) {
  return this.downloadSearchResults("/api/v1/patent/trials/proceedings/search", q, fmt, opts);
}
async downloadTrialDecisions(q?: string, fmt?: "json" | "csv", opts?: any) {
  return this.downloadSearchResults("/api/v1/patent/trials/decisions/search", q, fmt, opts);
}
async downloadTrialDocuments(q?: string, fmt?: "json" | "csv", opts?: any) {
  return this.downloadSearchResults("/api/v1/patent/trials/documents/search", q, fmt, opts);
}
async downloadAppealDecisions(q?: string, fmt?: "json" | "csv", opts?: any) {
  return this.downloadSearchResults("/api/v1/patent/appeals/decisions/search", q, fmt, opts);
}
async downloadInterferenceDecisions(q?: string, fmt?: "json" | "csv", opts?: any) {
  return this.downloadSearchResults("/api/v1/patent/interferences/decisions/search", q, fmt, opts);
}
```

### 7B. CLI `--download` flag pattern

Instead of separate subcommands, consider adding a `--download [path]` flag to each search command:

```typescript
.option("--download [path]", "Download results to file (default: stdout)")
.option("--csv", "Download as CSV (default: JSON)")
```

---

## 8. Document Download URIs

### 8A. PTAB fileDownloadURI fields exist but are not used

Multiple response objects contain `fileDownloadURI` or `downloadURI` fields that point to actual document files (PDFs, ZIPs). These are present in:

- `trialMetaData.fileDownloadURI` -- ZIP of all trial documents
- `appealMetaData.fileDownloadURI` -- ZIP of all appeal documents (e.g., `.zip` file)
- `documentData.fileDownloadURI` -- individual document download
- `documentData.downloadURI` -- alternate key in some responses
- Petition `downloadOptionBag[].downloadUrl` -- per-format download URLs

The CLI never uses these for actual file downloads. The `downloadDocument` method exists in `client.ts` (line 341) but no command invokes it for PTAB/appeal documents.

**Add a universal document download command:**

```typescript
// In ptab.ts:
ptab
  .command("download")
  .alias("dl")
  .description("Download a PTAB document by URL or trial/appeal number")
  .argument("<source>", "Document URL, trial number, or appeal number")
  .option("-o, --output <path>", "Output directory", ".")
  .option("--type <type>", "Source type: trial, appeal, interference", "trial")
  .action(async (source, opts) => {
    const client = createClient({ debug: program.opts().debug });

    if (source.startsWith("http")) {
      // Direct URL download
      const filename = path.join(opts.output, path.basename(source));
      await client.downloadDocument(source, filename);
      console.log(`Downloaded: ${filename}`);
    } else {
      // Look up the trial/appeal and get the fileDownloadURI
      // Then download it
    }
  });
```

---

## 9. POST Search Endpoint Support

### 9A. POST not used for PTAB/Appeal/Interference/Petition searches

The API documentation mentions POST support for at least proceedings search and petition search (both say "POST: See Swagger documentation"). POST allows structured `SearchRequest` bodies with filters, range filters, sort arrays, field selection, and facets -- which is far more powerful than the GET query string approach.

The client has `searchPatentsPost` for patent search but no POST equivalents for:
- PTAB proceedings search
- PTAB decisions search
- PTAB documents search
- Appeals search
- Interferences search
- Petition decisions search

**Add POST search methods:**

```typescript
async searchProceedingsPost(body: SearchRequest): Promise<ProceedingDataResponse> {
  return this.request<ProceedingDataResponse>("POST", "/api/v1/patent/trials/proceedings/search", { body });
}

async searchTrialDecisionsPost(body: SearchRequest): Promise<TrialDocumentResponse> {
  return this.request<TrialDocumentResponse>("POST", "/api/v1/patent/trials/decisions/search", { body });
}

async searchAppealDecisionsPost(body: SearchRequest): Promise<AppealDecisionResponse> {
  return this.request<AppealDecisionResponse>("POST", "/api/v1/patent/appeals/decisions/search", { body });
}

async searchPetitionDecisionsPost(body: SearchRequest): Promise<PetitionDecisionResponse> {
  return this.request<PetitionDecisionResponse>("POST", "/api/v1/petition/decisions/search", { body });
}
```

---

## 10. Summary: Endpoint Coverage Matrix

| API Endpoint | Client Method | CLI Command | Table Formatter | Status |
|---|---|---|---|---|
| **PTAB Trials - Proceedings** | | | | |
| `GET .../proceedings/search` | `searchProceedings` | `ptab search` | `formatProceedingTable` | PARTIAL -- missing filters/fields |
| `GET .../proceedings/search/download` | MISSING | MISSING | N/A | NOT IMPLEMENTED |
| `GET .../proceedings/{trialNumber}` | `getProceeding` | `ptab get` | None (JSON only) | PARTIAL |
| **PTAB Trials - Decisions** | | | | |
| `GET .../decisions/search` | `searchTrialDecisions` | `ptab decisions` | MISSING | PARTIAL -- no table, few filters |
| `GET .../decisions/search/download` | MISSING | MISSING | N/A | NOT IMPLEMENTED |
| `GET .../decisions/{documentIdentifier}` | `getTrialDecision` | NO CLI FLAG | N/A | CLI INACCESSIBLE |
| `GET .../{trialNumber}/decisions` | `getTrialDecisions` | `ptab decisions --trial` | None (JSON only) | OK |
| **PTAB Trials - Documents** | | | | |
| `GET .../documents/search` | `searchTrialDocuments` | `ptab docs` | MISSING | PARTIAL |
| `GET .../documents/search/download` | MISSING | MISSING | N/A | NOT IMPLEMENTED |
| `GET .../documents/{documentIdentifier}` | `getTrialDocument` | NO CLI FLAG | N/A | CLI INACCESSIBLE |
| `GET .../{trialNumber}/documents` | `getTrialDocuments` | `ptab docs --trial` | None (JSON only) | OK |
| **PTAB Appeals** | | | | |
| `GET .../appeals/decisions/search` | `searchAppealDecisions` | `ptab appeals` | MISSING | PARTIAL -- no filters |
| `GET .../appeals/decisions/search/download` | MISSING | MISSING | N/A | NOT IMPLEMENTED |
| `GET .../appeals/decisions/{documentIdentifier}` | `getAppealDecision` | NO CLI FLAG | N/A | CLI INACCESSIBLE |
| `GET .../appeals/{appealNumber}/decisions` | `getAppealDecisions` | `ptab appeals --appeal` | None (JSON only) | OK |
| **PTAB Interferences** | | | | |
| `GET .../interferences/decisions/search` | `searchInterferenceDecisions` | `ptab interferences` | MISSING | PARTIAL |
| `GET .../interferences/decisions/search/download` | MISSING | MISSING | N/A | NOT IMPLEMENTED |
| `GET .../interferences/decisions/{documentIdentifier}` | `getInterferenceDecision` | NO CLI FLAG | N/A | CLI INACCESSIBLE |
| `GET .../interferences/{interferenceNumber}/decisions` | `getInterferenceDecisions` | `ptab interferences --interference` | None (JSON only) | OK |
| **Petition Decisions** | | | | |
| `GET .../petition/decisions/search` | `searchPetitionDecisions` | `petition search` | Inline (not in format.ts) | PARTIAL -- few filters |
| `GET .../petition/decisions/{id}?includeDocuments=true` | `getPetitionDecision` | `petition get` | None (JSON only) | PARTIAL -- untyped response |
| Petition document download | MISSING | MISSING | N/A | NOT IMPLEMENTED |

---

## 11. Priority Action Items

### P0 -- Type/Data Correctness Bugs (fix immediately)

1. **`regularPetitionerData` type is wrong** in `ProceedingData` (api.ts:284). Typed as `{counselName; realPartyInInterestName}` but API returns full `PartyData`. Silently loses data.
2. **`DecisionData.appealOutcomeCategory`** missing from type (api.ts:311). Appeals outcome data is silently dropped.
3. **`DecisionData.statuteAndRuleBag` and `issueTypeBag`** typed as `string` but API returns `string[]`. Will cause display bugs.
4. **`AppealData` missing `decisionData` block entirely** (api.ts:340). The most important part of an appeal response is not typed.
5. **`PetitionDecision` missing `decisionTypeCodeDescriptionText`** (api.ts:394). The human-readable decision description is lost.

### P1 -- Missing Table Formatters (high impact for agent usability)

6. Add `formatDecisionTable` for PTAB trial decisions.
7. Add `formatAppealTable` for PTAB appeals.
8. Add `formatInterferenceTable` for PTAB interferences.
9. Move inline petition table to `formatPetitionTable` in format.ts.
10. Add detail formatters (`formatProceedingDetail`, `formatPetitionDetail`) for single-record views.

### P2 -- Missing Shorthand Flags (high impact for agents)

11. Add `--status`, `--app`, `--inventor`, `--tc`, `--art-unit`, date range flags to `ptab search`.
12. Add `--outcome`, `--type`, `--decision-type`, `--patent`, `--doc-id` flags to `ptab decisions`.
13. Add `--outcome`, `--app`, `--patent`, `--inventor`, `--tc`, `--art-unit`, `--rpi`, `--counsel`, `--filed-after`, `--decided-after`, `--issue-type`, `--doc-id` flags to `ptab appeals`.
14. Add `--app`, `--patent`, `--applicant`, `--tc`, `--art-unit`, `--entity`, `--rule`, `--statute`, `--petition-type`, date range, `--court-action`, `--issue` flags to `petition search`.
15. Add `--offset` and `--sort` to all commands that are missing them.

### P3 -- Missing Download/Export Endpoints

16. Implement all 5 download-search-results endpoints in client.ts.
17. Add `--download [path]` and `--csv` flags to search commands.
18. Add petition document download command.
19. Add PTAB document download command using `fileDownloadURI`.

### P4 -- Missing POST Search Support

20. Add POST search methods for PTAB, appeals, interferences, petitions.
21. Add `--filters-file <path>` flag to allow JSON filter input for advanced queries.

### P5 -- Documentation & Raw Data Capture

22. Capture raw API docs for PTAB Interferences (4 endpoints) -- no docs/raw files exist.
23. Capture PTAB Trials Documents endpoints (search, download, by-trial, by-doc-id) raw docs.
24. Capture PTAB Trials Decisions by Document Identifier raw docs.

