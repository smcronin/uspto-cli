# Patent File Wrapper Detail Endpoints - Gap Analysis

**Scope**: Continuity, Assignments, Foreign Priority, Associated Documents, Patent Term Adjustment, Patent Term Extension, Address/Attorney, Status Codes

**Date**: 2026-02-28

**Methodology**: Compared raw USPTO Open Data Portal API documentation against the current CLI implementation across `src/commands/app.ts`, `src/api/client.ts`, `src/types/api.ts`, `src/utils/format.ts`, and `src/commands/status.ts`.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Missing Endpoint: Patent Term Extension](#2-missing-endpoint-patent-term-extension)
3. [Continuity Endpoint Gaps](#3-continuity-endpoint-gaps)
4. [Assignments Endpoint Gaps](#4-assignments-endpoint-gaps)
5. [Foreign Priority Endpoint Gaps](#5-foreign-priority-endpoint-gaps)
6. [Associated Documents Endpoint Gaps](#6-associated-documents-endpoint-gaps)
7. [Patent Term Adjustment Gaps](#7-patent-term-adjustment-gaps)
8. [Address/Attorney Endpoint Gaps](#8-addressattorney-endpoint-gaps)
9. [Status Codes Endpoint Gaps](#9-status-codes-endpoint-gaps)
10. [Cross-Cutting Issues](#10-cross-cutting-issues)

---

## 1. Executive Summary

| Category | Severity | Description |
|----------|----------|-------------|
| Missing Endpoint | **CRITICAL** | Patent Term Extension (`/extension`) has no client method, no command, no types |
| Missing Formatters | **HIGH** | 5 of 8 subcommands dump raw JSON instead of structured tables: `attorney`, `adjustment`, `foreign-priority`, `associated-docs`, and the missing `extension` |
| Missing Format Flag | **HIGH** | `attorney`, `adjustment`, `foreign-priority`, `associated-docs` lack `--format` option |
| Incomplete Types | **HIGH** | `PatentTermExtensionData`, `ForeignPriorityData`, `AttorneyData`, `AssociatedDocumentData` types are completely absent |
| Missing Response Fields | **MEDIUM** | Assignment assignee address fields, domestic representative, correspondence address details not typed |
| Missing Continuity Fields | **MEDIUM** | `childApplicationNumberText` in parent bag and `parentApplicationNumberText` in child bag not rendered in table |
| Missing Download Support | **MEDIUM** | Assignment document download URI (`assignmentDocumentLocationURI`) not exposed as downloadable |
| Missing POST Support | **LOW** | Status codes POST search not exposed |

**Total gaps identified**: 47

---

## 2. Missing Endpoint: Patent Term Extension

### API Documentation

```
GET /api/v1/patent/applications/{applicationNumberText}/extension
```

Returns patent term extension data with the same structure as Patent Term Adjustment but with extension-specific field names.

### Current State

- **client.ts**: No `getExtension()` method exists
- **app.ts**: No `extension` or `pte` subcommand exists
- **types/api.ts**: No `PatentTermExtensionData` interface exists
- **format.ts**: No `formatExtensionTable()` exists

### Response Fields (from API docs)

| Field | Type | Description |
|-------|------|-------------|
| `applicantDayDelayQuantity` | Number | Applicant delay days |
| `overlappingDayQuantity` | Number | Overlapping delay days |
| `ipOfficeExtensionDelayQuantity` | Number | IP office extension delay summation |
| `cDelayQuantity` | Number | C delay (interference, secrecy, appellate) |
| `extensionTotalQuantity` | Number | Total PTE calculation |
| `bDelayQuantity` | Number | B delay (3-year issue window) |
| `aDelayQuantity` | Number | A delay (USPTO processing delays) |
| `nonOverlappingDayQuantity` | Number | Non-overlapping days summation |
| `patentTermExtensionHistoryDataBag[]` | Array | History events |
| `patentTermExtensionHistoryDataBag[].eventDescriptionText` | String | Event description |
| `patentTermExtensionHistoryDataBag[].eventSequenceNumber` | Number | Sequence number |
| `patentTermExtensionHistoryDataBag[].originatingEventSequenceNumber` | Number | Originating sequence |
| `patentTermExtensionHistoryDataBag[].ptaPTECode` | String | PTA or PTE code |
| `patentTermExtensionHistoryDataBag[].eventDate` | Date | Event date |

### Required Implementation

#### 2a. Add type to `src/types/api.ts`

```typescript
export interface PatentTermExtensionHistoryEntry {
  eventDescriptionText: string;
  eventSequenceNumber: number;
  originatingEventSequenceNumber: number;
  ptaPTECode: string;
  eventDate: string;
}

export interface PatentTermExtensionData {
  applicantDayDelayQuantity: number;
  overlappingDayQuantity: number;
  ipOfficeExtensionDelayQuantity: number;
  cDelayQuantity: number;
  extensionTotalQuantity: number;
  bDelayQuantity: number;
  aDelayQuantity: number;
  nonOverlappingDayQuantity: number;
  patentTermExtensionHistoryDataBag: PatentTermExtensionHistoryEntry[];
}
```

#### 2b. Add client method to `src/api/client.ts`

```typescript
async getExtension(appNumber: string): Promise<any> {
  return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/extension`);
}
```

#### 2c. Add subcommand to `src/commands/app.ts`

```typescript
app
  .command("extension")
  .alias("pte")
  .description("Get patent term extension data")
  .argument("<appNumber>", "Application number")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
  .action(async (appNumber, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.getExtension(appNumber);
    if (opts.format === "json") {
      console.log(formatOutput(result, "json"));
    } else {
      const data = result.patentFileWrapperDataBag?.[0]?.patentTermExtensionData;
      console.log(formatExtensionTable(data));
    }
  });
```

#### 2d. Add formatter to `src/utils/format.ts`

```typescript
export function formatExtensionTable(data: PatentTermExtensionData): string {
  if (!data) return chalk.yellow("No patent term extension data found.");

  const lines = [
    "",
    chalk.bold("  Patent Term Extension Summary:"),
    "",
    `  ${chalk.gray("A Delay (USPTO):")}           ${data.aDelayQuantity} days`,
    `  ${chalk.gray("B Delay (3-yr issue):")}       ${data.bDelayQuantity} days`,
    `  ${chalk.gray("C Delay (interf/secrecy):")}   ${data.cDelayQuantity} days`,
    `  ${chalk.gray("Overlapping:")}                ${data.overlappingDayQuantity} days`,
    `  ${chalk.gray("Non-Overlapping:")}            ${data.nonOverlappingDayQuantity} days`,
    `  ${chalk.gray("IP Office Delay:")}            ${data.ipOfficeExtensionDelayQuantity} days`,
    `  ${chalk.gray("Applicant Delay:")}            ${data.applicantDayDelayQuantity} days`,
    `  ${chalk.gray("TOTAL EXTENSION:")}            ${chalk.bold(String(data.extensionTotalQuantity))} days`,
    "",
  ];

  if (data.patentTermExtensionHistoryDataBag?.length) {
    lines.push(chalk.bold("  Extension History:"));
    const table = new Table({
      head: [chalk.cyan("Seq"), chalk.cyan("Date"), chalk.cyan("Code"), chalk.cyan("Description")],
      colWidths: [7, 13, 6, 60],
      wordWrap: true,
    });
    for (const h of data.patentTermExtensionHistoryDataBag) {
      table.push([
        h.eventSequenceNumber ?? "",
        h.eventDate || "",
        h.ptaPTECode || "",
        h.eventDescriptionText || "",
      ]);
    }
    lines.push(table.toString());
  }

  return lines.join("\n");
}
```

---

## 3. Continuity Endpoint Gaps

### Endpoint

```
GET /api/v1/patent/applications/{applicationNumberText}/continuity
```

### 3a. Missing Response Fields in Display

The API returns `childApplicationNumberText` inside each parent continuity entry (indicating which child is claiming the parent), and `parentApplicationNumberText` inside each child entry (indicating which parent the child claims). The current `formatContinuityTable()` does NOT display these cross-referencing fields.

**Parent bag has these fields returned but not displayed:**
| Field | Currently Displayed | Notes |
|-------|-------------------|-------|
| `parentApplicationStatusCode` | NO | Numeric status code (only description is shown) |
| `firstInventorToFileIndicator` | NO | AIA/FITF indicator |
| `claimParentageTypeCodeDescriptionText` | NO | Human-readable description like "is a Continuation of" |
| `childApplicationNumberText` | NO | Which child is claiming this parent |

**Child bag has these fields returned but not displayed:**
| Field | Currently Displayed | Notes |
|-------|-------------------|-------|
| `childApplicationStatusCode` | NO | Numeric status code (only description is shown) |
| `firstInventorToFileIndicator` | NO | AIA/FITF indicator |
| `claimParentageTypeCodeDescriptionText` | NO | Human-readable description |
| `parentApplicationNumberText` | NO | Which parent the child claims |

### 3b. Type Definition Gaps

In `src/types/api.ts`, `ContinuityData` is defined as a single interface trying to serve both parent and child entries. The API actually returns different field names for each bag:

**Parent bag uses**: `parentApplicationStatusCode`, `parentApplicationStatusDescriptionText`, `parentApplicationFilingDate`, `parentApplicationNumberText`, `parentPatentNumber`, `childApplicationNumberText`

**Child bag uses**: `childApplicationStatusCode`, `childApplicationStatusDescriptionText`, `childApplicationFilingDate`, `childApplicationNumberText`, `parentApplicationNumberText`

The current type has `childPatentNumber` as a field, but the API sample response for child entries does NOT include a `childPatentNumber` field. This field appears to be fabricated in the type definition.

### Recommended Changes

#### In `src/types/api.ts`, split into two interfaces:

```typescript
export interface ParentContinuityEntry {
  parentApplicationStatusCode: number;
  firstInventorToFileIndicator?: boolean;
  claimParentageTypeCode: string;
  claimParentageTypeCodeDescriptionText: string;
  parentApplicationStatusDescriptionText: string;
  parentApplicationNumberText: string;
  parentApplicationFilingDate: string;
  parentPatentNumber?: string;
  childApplicationNumberText: string;
}

export interface ChildContinuityEntry {
  childApplicationStatusCode: number | null;
  firstInventorToFileIndicator?: boolean;
  claimParentageTypeCode: string;
  claimParentageTypeCodeDescriptionText: string;
  childApplicationStatusDescriptionText: string;
  childApplicationNumberText: string;
  childApplicationFilingDate: string;
  parentApplicationNumberText: string;
}
```

#### In `src/utils/format.ts`, update `formatContinuityTable()`:

Add `claimParentageTypeCodeDescriptionText` as a separate column or as a parenthetical next to the type code, and add the cross-referencing app number column. Example improved parent table:

```
| Parent App # | Patent #   | Relationship             | Filing Date | Status       | Child App # |
|------------- |----------- |------------------------- |------------ |------------- |------------ |
| 17006669     | 11466319   | is a Continuation of     | 2020-08-28  | Patented     | 18045436    |
```

---

## 4. Assignments Endpoint Gaps

### Endpoint

```
GET /api/v1/patent/applications/{applicationNumberText}/assignment
```

### 4a. Missing Response Fields Not Displayed

The current `formatAssignmentTable()` shows only 5 columns: Reel/Frame, Recorded Date, Conveyance, Assignor names, Assignee names. The API returns significantly more data.

**Fields returned by API but NOT displayed or typed:**

| Field | Type | Currently In Types | Currently Displayed | Priority |
|-------|------|-------------------|-------------------|----------|
| `assignmentReceivedDate` | Date | YES | NO | Medium |
| `assignmentMailedDate` | Date | YES | NO | Medium |
| `pageTotalQuantity` | Number | YES | NO | Low |
| `imageAvailableStatusCode` | Boolean | YES | NO | Low |
| `assignmentDocumentLocationURI` | String | YES | NO | **HIGH** - downloadable document |
| `assignorBag[].executionDate` | Date | NO (bag is `any[]`) | NO | Medium |
| `assigneeBag[].assigneeNameText` | String | NO (bag is `any[]`) | Partially (wrong field name) | **HIGH** |
| `assigneeBag[].assigneeAddress.addressLineOneText` | String | NO | NO | Medium |
| `assigneeBag[].assigneeAddress.cityName` | String | NO | NO | Medium |
| `assigneeBag[].assigneeAddress.countryOrStateCode` | String | NO | NO | Low |
| `assigneeBag[].assigneeAddress.ictStateCode` | String | NO | NO | Low |
| `assigneeBag[].assigneeAddress.ictCountryCode` | String | NO | NO | Low |
| `assigneeBag[].assigneeAddress.geographicRegionName` | String | NO | NO | Low |
| `assigneeBag[].assigneeAddress.geographicRegionCode` | String | NO | NO | Low |
| `assigneeBag[].assigneeAddress.countryName` | String | NO | NO | Low |
| `assigneeBag[].assigneeAddress.postalCode` | String | NO | NO | Low |
| `correspondenceAddress[].correspondentNameText` | String | NO (typed as `any[]`) | NO | Medium |
| `correspondenceAddress[].addressLineOneText` | String | NO | NO | Low |
| `correspondenceAddress[].addressLineTwoText` | String | NO | NO | Low |
| `domesticRepresentative.name` | String | NOT IN TYPES | NO | Medium |
| `domesticRepresentative.addressLineOneText` | String | NOT IN TYPES | NO | Low |
| `domesticRepresentative.cityName` | String | NOT IN TYPES | NO | Low |
| `domesticRepresentative.postalCode` | String | NOT IN TYPES | NO | Low |
| `domesticRepresentative.geographicRegionName` | String | NOT IN TYPES | NO | Low |
| `domesticRepresentative.countryName` | String | NOT IN TYPES | NO | Low |
| `domesticRepresentative.emailAddress` | String | NOT IN TYPES | NO | Low |

### 4b. Bug: Assignee Name Field Mismatch

In `formatAssignmentTable()` at line 177:
```typescript
const assignees = (a.assigneeBag || []).map((x: any) => x.name || x.assigneeName || "").join(", ");
```

The API returns the field as `assigneeNameText`, NOT `assigneeName` or `name`. This means assignee names are likely rendering as empty strings.

**Fix:**
```typescript
const assignees = (a.assigneeBag || []).map((x: any) => x.assigneeNameText || x.name || "").join(", ");
```

### 4c. Missing Assignment Document Download Support

The API returns `assignmentDocumentLocationURI` which is a direct download URL (e.g., `https://assignmentcenter.uspto.gov/ipas/search/api/v2/public/download/patent/066070/0442`). This should be exposed as a download option or at least displayed in the table.

**Recommendation**: Add a `--download` flag to the assignments command or a new `app assign-download` command:

```typescript
app
  .command("assignment-download")
  .alias("assign-dl")
  .description("Download assignment document")
  .argument("<appNumber>", "Application number")
  .argument("[assignmentIndex]", "Assignment index from assignments list", "1")
  .option("-o, --output <path>", "Output file path")
  .action(async (appNumber, assignmentIndex, opts) => {
    // fetch assignments, get the URI at the given index, download
  });
```

### 4d. Recommended Type Updates

```typescript
export interface AssigneeAddress {
  addressLineOneText?: string;
  addressLineTwoText?: string;
  addressLineThreeText?: string;
  addressLineFourText?: string;
  cityName?: string;
  countryOrStateCode?: string;
  ictStateCode?: string;
  ictCountryCode?: string;
  geographicRegionName?: string;
  geographicRegionCode?: string;
  countryName?: string;
  postalCode?: string;
}

export interface Assignor {
  executionDate: string;
  assignorName: string;
}

export interface Assignee {
  assigneeNameText: string;
  assigneeAddress?: AssigneeAddress;
}

export interface CorrespondenceAddress {
  correspondentNameText?: string;
  addressLineOneText?: string;
  addressLineTwoText?: string;
  addressLineThreeText?: string;
  addressLineFourText?: string;
}

export interface DomesticRepresentative {
  name?: string;
  addressLineOneText?: string;
  addressLineTwoText?: string;
  addressLineThreeText?: string;
  addressLineFourText?: string;
  cityName?: string;
  postalCode?: string;
  geographicRegionName?: string;
  countryName?: string;
  emailAddress?: string;
}

export interface Assignment {
  reelNumber: number;            // API returns Number, not String
  frameNumber: number;           // API returns Number, not String
  reelAndFrameNumber: string;
  pageTotalQuantity: number;
  imageAvailableStatusCode: boolean;
  assignmentDocumentLocationURI: string;
  assignmentReceivedDate: string;
  assignmentRecordedDate: string;
  assignmentMailedDate: string;
  conveyanceText: string;
  assignorBag: Assignor[];
  assigneeBag: Assignee[];
  correspondenceAddress: CorrespondenceAddress[];
  domesticRepresentative?: DomesticRepresentative;
}
```

---

## 5. Foreign Priority Endpoint Gaps

### Endpoint

```
GET /api/v1/patent/applications/{applicationNumberText}/foreign-priority
```

### 5a. No Table Formatter

The `foreign-priority` command in `app.ts` (line 147-155) only outputs raw JSON via `formatOutput(result, "json")`. There is no table formatter and no `--format` flag.

### 5b. No TypeScript Type Definition

The API returns `foreignPriorityBag[]` with three fields per entry, but no corresponding type exists in `api.ts`.

### 5c. Missing Response Fields (All of Them)

| Field | Type | In Types | Displayed | Agent Value |
|-------|------|----------|-----------|-------------|
| `foreignPriorityBag[].filingDate` | Date | NO | NO (raw JSON only) | HIGH |
| `foreignPriorityBag[].applicationNumberText` | String | NO | NO (raw JSON only) | HIGH |
| `foreignPriorityBag[].ipOfficeName` | String | NO | NO (raw JSON only) | HIGH |

### Recommended Implementation

#### Add type to `src/types/api.ts`:

```typescript
export interface ForeignPriorityEntry {
  filingDate: string;
  applicationNumberText: string;
  ipOfficeName: string;
}

export interface ForeignPriorityResponse {
  count: number;
  patentFileWrapperDataBag: Array<{
    applicationNumberText: string;
    foreignPriorityBag: ForeignPriorityEntry[];
    requestIdentifier: string;
  }>;
}
```

#### Add formatter to `src/utils/format.ts`:

```typescript
export function formatForeignPriorityTable(entries: ForeignPriorityEntry[]): string {
  if (!entries?.length) return chalk.yellow("No foreign priority data found.");

  const table = new Table({
    head: [chalk.cyan("Country/Office"), chalk.cyan("Application #"), chalk.cyan("Filing Date")],
    colWidths: [25, 30, 13],
    wordWrap: true,
  });

  for (const e of entries) {
    table.push([
      e.ipOfficeName || "",
      e.applicationNumberText || "",
      e.filingDate || "",
    ]);
  }

  return table.toString();
}
```

#### Update command in `src/commands/app.ts`:

```typescript
app
  .command("foreign-priority")
  .alias("fp")
  .description("Get foreign priority data")
  .argument("<appNumber>", "Application number")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
  .action(async (appNumber, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.getForeignPriority(appNumber);
    if (opts.format === "json") {
      console.log(formatOutput(result, "json"));
    } else {
      const data = result.patentFileWrapperDataBag?.[0];
      console.log(formatForeignPriorityTable(data?.foreignPriorityBag));
    }
  });
```

---

## 6. Associated Documents Endpoint Gaps

### Endpoint

```
GET /api/v1/patent/applications/{applicationNumberText}/associated-documents
```

### 6a. No Table Formatter

The `associated-docs` command (line 157-166) only outputs raw JSON. There is no table view and no `--format` flag.

### 6b. No Dedicated Type

The types file has `FileMetaData` which partially covers the fields, but there is no `AssociatedDocumentsResponse` type.

### 6c. Missing Response Fields

| Field | Type | In Types | Displayed | Notes |
|-------|------|----------|-----------|-------|
| `grantDocumentMetaData.productIdentifier` | String | YES | NO (raw JSON) | e.g., "PTGRXML" |
| `grantDocumentMetaData.zipFileName` | String | YES | NO (raw JSON) | e.g., "ipg240604.zip" |
| `grantDocumentMetaData.fileCreateDateTime` | Date | YES | NO (raw JSON) | |
| `grantDocumentMetaData.xmlFileName` | String | YES | NO (raw JSON) | |
| `grantDocumentMetaData.fileLocationURI` | String | YES | NO (raw JSON) | **Downloadable XML** |
| `pgpubDocumentMetaData.productIdentifier` | String | YES | NO (raw JSON) | e.g., "APPXML" |
| `pgpubDocumentMetaData.zipFileName` | String | YES | NO (raw JSON) | |
| `pgpubDocumentMetaData.fileCreateDateTime` | Date | YES | NO (raw JSON) | |
| `pgpubDocumentMetaData.xmlFileName` | String | YES | NO (raw JSON) | |
| `pgpubDocumentMetaData.fileLocationURI` | String | YES | NO (raw JSON) | **Downloadable XML** |

### 6d. Missing XML Download Support

The `fileLocationURI` values are direct download URLs for XML full-text documents. These are valuable for agents that need to parse patent claims and descriptions programmatically.

### Recommended Implementation

#### Add formatter to `src/utils/format.ts`:

```typescript
export function formatAssociatedDocsTable(data: any): string {
  if (!data) return chalk.yellow("No associated documents found.");

  const lines: string[] = [""];

  if (data.grantDocumentMetaData) {
    const g = data.grantDocumentMetaData;
    lines.push(chalk.bold("  Patent Grant XML:"));
    lines.push(`  ${chalk.gray("Product:")}    ${g.productIdentifier || "-"}`);
    lines.push(`  ${chalk.gray("Zip File:")}   ${g.zipFileName || "-"}`);
    lines.push(`  ${chalk.gray("XML File:")}   ${g.xmlFileName || "-"}`);
    lines.push(`  ${chalk.gray("Created:")}    ${g.fileCreateDateTime || "-"}`);
    lines.push(`  ${chalk.gray("URI:")}        ${g.fileLocationURI || "-"}`);
    lines.push("");
  }

  if (data.pgpubDocumentMetaData) {
    const p = data.pgpubDocumentMetaData;
    lines.push(chalk.bold("  Published Application XML:"));
    lines.push(`  ${chalk.gray("Product:")}    ${p.productIdentifier || "-"}`);
    lines.push(`  ${chalk.gray("Zip File:")}   ${p.zipFileName || "-"}`);
    lines.push(`  ${chalk.gray("XML File:")}   ${p.xmlFileName || "-"}`);
    lines.push(`  ${chalk.gray("Created:")}    ${p.fileCreateDateTime || "-"}`);
    lines.push(`  ${chalk.gray("URI:")}        ${p.fileLocationURI || "-"}`);
    lines.push("");
  }

  if (lines.length <= 1) return chalk.yellow("No associated documents found.");
  return lines.join("\n");
}
```

#### Add XML download subcommand:

```typescript
app
  .command("download-xml")
  .alias("dl-xml")
  .description("Download associated XML full-text document")
  .argument("<appNumber>", "Application number")
  .option("--type <type>", "Document type: grant, pgpub", "grant")
  .option("-o, --output <path>", "Output file path")
  .action(async (appNumber, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.getAssociatedDocuments(appNumber);
    const data = result.patentFileWrapperDataBag?.[0];
    const meta = opts.type === "pgpub"
      ? data?.pgpubDocumentMetaData
      : data?.grantDocumentMetaData;
    if (!meta?.fileLocationURI) {
      console.error(`No ${opts.type} XML document available.`);
      process.exit(1);
    }
    const outPath = opts.output || meta.xmlFileName || `${appNumber}_${opts.type}.xml`;
    console.log(`Downloading: ${meta.xmlFileName}`);
    const savedPath = await client.downloadDocument(meta.fileLocationURI, outPath);
    console.log(`Saved to: ${savedPath}`);
  });
```

---

## 7. Patent Term Adjustment Gaps

### Endpoint

```
GET /api/v1/patent/applications/{applicationNumberText}/adjustment
```

### 7a. No Table Formatter

The `adjustment` command (line 136-144) only outputs raw JSON. There is no `--format` flag.

### 7b. Missing `ipOfficeAdjustmentDelayQuantity` in Types

The `PatentTermAdjustmentData` interface in `api.ts` is missing `ipOfficeAdjustmentDelayQuantity`. This field is present in the API response.

**Current type has 7 fields. API returns 8 + history bag:**

| Field | In Types | Displayed |
|-------|----------|-----------|
| `applicantDayDelayQuantity` | YES | NO (raw JSON) |
| `overlappingDayQuantity` | YES | NO (raw JSON) |
| `ipOfficeAdjustmentDelayQuantity` | **NO** | NO |
| `cDelayQuantity` | YES | NO (raw JSON) |
| `adjustmentTotalQuantity` | YES | NO (raw JSON) |
| `bDelayQuantity` | YES | NO (raw JSON) |
| `aDelayQuantity` | YES | NO (raw JSON) |
| `nonOverlappingDayQuantity` | YES | NO (raw JSON) |
| `patentTermAdjustmentHistoryDataBag` | YES (as `any[]`) | NO (raw JSON) |

### 7c. History Bag Not Typed

`patentTermAdjustmentHistoryDataBag` is typed as `any[]` but the API returns structured data with fields: `eventDescriptionText`, `eventSequenceNumber`, `originatingEventSequenceNumber`, `ptaPTECode`, `eventDate`.

### Recommended Type Update

```typescript
export interface PatentTermAdjustmentHistoryEntry {
  eventDescriptionText: string;
  eventSequenceNumber: number;
  originatingEventSequenceNumber: number;
  ptaPTECode: string;
  eventDate: string;
}

export interface PatentTermAdjustmentData {
  aDelayQuantity: number;
  bDelayQuantity: number;
  cDelayQuantity: number;
  adjustmentTotalQuantity: number;
  applicantDayDelayQuantity: number;
  nonOverlappingDayQuantity: number;
  overlappingDayQuantity: number;
  ipOfficeAdjustmentDelayQuantity: number;  // MISSING - add this
  patentTermAdjustmentHistoryDataBag: PatentTermAdjustmentHistoryEntry[];  // replace any[]
}
```

### Recommended Formatter

```typescript
export function formatAdjustmentTable(data: PatentTermAdjustmentData): string {
  if (!data) return chalk.yellow("No patent term adjustment data found.");

  const lines = [
    "",
    chalk.bold("  Patent Term Adjustment Summary:"),
    "",
    `  ${chalk.gray("A Delay (USPTO):")}           ${data.aDelayQuantity} days`,
    `  ${chalk.gray("B Delay (3-yr issue):")}       ${data.bDelayQuantity} days`,
    `  ${chalk.gray("C Delay (interf/secrecy):")}   ${data.cDelayQuantity} days`,
    `  ${chalk.gray("Overlapping:")}                ${data.overlappingDayQuantity} days`,
    `  ${chalk.gray("Non-Overlapping:")}            ${data.nonOverlappingDayQuantity} days`,
    `  ${chalk.gray("IP Office Delay:")}            ${data.ipOfficeAdjustmentDelayQuantity} days`,
    `  ${chalk.gray("Applicant Delay:")}            ${data.applicantDayDelayQuantity} days`,
    `  ${chalk.gray("TOTAL ADJUSTMENT:")}           ${chalk.bold(String(data.adjustmentTotalQuantity))} days`,
    "",
  ];

  if (data.patentTermAdjustmentHistoryDataBag?.length) {
    lines.push(chalk.bold("  Adjustment History:"));
    const table = new Table({
      head: [chalk.cyan("Seq"), chalk.cyan("Date"), chalk.cyan("Code"), chalk.cyan("Description")],
      colWidths: [7, 13, 6, 60],
      wordWrap: true,
    });
    for (const h of data.patentTermAdjustmentHistoryDataBag) {
      table.push([
        h.eventSequenceNumber ?? "",
        h.eventDate || "",
        h.ptaPTECode || "",
        h.eventDescriptionText || "",
      ]);
    }
    lines.push(table.toString());
  }

  return lines.join("\n");
}
```

---

## 8. Address/Attorney Endpoint Gaps

### Endpoint

```
GET /api/v1/patent/applications/{applicationNumberText}/attorney
```

### 8a. No Table Formatter

The `attorney` command (line 126-133) only outputs raw JSON. There is no `--format` flag.

### 8b. No TypeScript Types

The `PatentFileWrapper` interface types `recordAttorney` as `any`. The API returns a deeply nested structure with three major sections:

1. `customerNumberCorrespondenceData` - Customer number and correspondence address
2. `powerOfAttorneyBag[]` - Attorneys with power of attorney
3. `attorneyBag[]` - Additional attorneys/agents

### 8c. Complete List of Missing Types

**`customerNumberCorrespondenceData`:**
| Field | Type |
|-------|------|
| `patronIdentifier` | Number |
| `organizationStandardName` | String |
| `powerOfAttorneyAddressBag[].nameLineOneText` | String |
| `powerOfAttorneyAddressBag[].nameLineTwoText` | String |
| `powerOfAttorneyAddressBag[].addressLineOneText` | String |
| `powerOfAttorneyAddressBag[].addressLineTwoText` | String |
| `powerOfAttorneyAddressBag[].geographicRegionName` | String |
| `powerOfAttorneyAddressBag[].geographicRegionCode` | String |
| `powerOfAttorneyAddressBag[].postalCode` | String |
| `powerOfAttorneyAddressBag[].cityName` | String |
| `powerOfAttorneyAddressBag[].countryCode` | String |
| `powerOfAttorneyAddressBag[].countryName` | String |
| `telecommunicationAddressBag[].telecommunicationNumber` | String |
| `telecommunicationAddressBag[].extensionNumber` | String |
| `telecommunicationAddressBag[].telecomTypeCode` | String |

**Each entry in `powerOfAttorneyBag[]` and `attorneyBag[]`:**
| Field | Type |
|-------|------|
| `firstName` | String |
| `middleName` | String |
| `lastName` | String |
| `namePrefix` | String |
| `nameSuffix` | String |
| `preferredName` | String (POA only) |
| `countryCode` | String (POA only) |
| `registrationNumber` | String |
| `activeIndicator` | String |
| `registeredPractitionerCategory` | String (e.g., "ATTNY", "AGENT") |
| `attorneyAddressBag[]` | nested address array |
| `telecommunicationAddressBag[]` | nested telecom array |

### Recommended Types

```typescript
export interface AttorneyAddress {
  nameLineOneText?: string;
  nameLineTwoText?: string;
  addressLineOneText?: string;
  addressLineTwoText?: string;
  geographicRegionName?: string;
  geographicRegionCode?: string;
  postalCode?: string;
  cityName?: string;
  countryCode?: string;
  countryName?: string;
}

export interface TelecommunicationAddress {
  telecommunicationNumber?: string;
  extensionNumber?: string;
  telecomTypeCode?: string;
}

export interface AttorneyEntry {
  firstName?: string;
  middleName?: string;
  lastName?: string;
  namePrefix?: string;
  nameSuffix?: string;
  preferredName?: string;
  countryCode?: string;
  registrationNumber?: string;
  activeIndicator?: string;
  registeredPractitionerCategory?: string;
  attorneyAddressBag?: AttorneyAddress[];
  telecommunicationAddressBag?: TelecommunicationAddress[];
}

export interface CustomerNumberCorrespondenceData {
  patronIdentifier?: number;
  organizationStandardName?: string;
  powerOfAttorneyAddressBag?: AttorneyAddress[];
  telecommunicationAddressBag?: TelecommunicationAddress[];
}

export interface RecordAttorney {
  customerNumberCorrespondenceData?: CustomerNumberCorrespondenceData;
  powerOfAttorneyBag?: AttorneyEntry[];
  attorneyBag?: AttorneyEntry[];
}
```

### Recommended Formatter

```typescript
export function formatAttorneyTable(data: RecordAttorney): string {
  if (!data) return chalk.yellow("No attorney/agent data found.");

  const lines: string[] = [""];

  if (data.customerNumberCorrespondenceData) {
    const c = data.customerNumberCorrespondenceData;
    lines.push(chalk.bold("  Correspondence:"));
    lines.push(`  ${chalk.gray("Customer #:")}  ${c.patronIdentifier || "-"}`);
    if (c.powerOfAttorneyAddressBag?.[0]) {
      const addr = c.powerOfAttorneyAddressBag[0];
      lines.push(`  ${chalk.gray("Firm:")}        ${addr.nameLineOneText || "-"}`);
      lines.push(`  ${chalk.gray("Address:")}     ${addr.addressLineOneText || ""} ${addr.addressLineTwoText || ""}`);
      lines.push(`  ${chalk.gray("City/State:")}  ${addr.cityName || ""}, ${addr.geographicRegionCode || ""} ${addr.postalCode || ""}`);
    }
    lines.push("");
  }

  const allAttorneys = [
    ...(data.powerOfAttorneyBag || []).map(a => ({ ...a, _source: "POA" })),
    ...(data.attorneyBag || []).map(a => ({ ...a, _source: "Attorney" })),
  ];

  if (allAttorneys.length) {
    lines.push(chalk.bold("  Attorneys/Agents:"));
    const table = new Table({
      head: [
        chalk.cyan("Name"),
        chalk.cyan("Reg #"),
        chalk.cyan("Type"),
        chalk.cyan("Category"),
        chalk.cyan("Active"),
        chalk.cyan("Firm"),
      ],
      colWidths: [25, 10, 10, 10, 8, 30],
      wordWrap: true,
    });

    for (const a of allAttorneys) {
      const name = [a.firstName, a.middleName, a.lastName].filter(Boolean).join(" ");
      const firm = a.attorneyAddressBag?.[0]?.nameLineOneText || "";
      table.push([
        name,
        a.registrationNumber || "",
        (a as any)._source || "",
        a.registeredPractitionerCategory || "",
        a.activeIndicator || "",
        firm.substring(0, 35),
      ]);
    }
    lines.push(table.toString());
  }

  return lines.join("\n");
}
```

---

## 9. Status Codes Endpoint Gaps

### Endpoints

```
GET  /api/v1/patent/status-codes
POST /api/v1/patent/status-codes
```

### 9a. Missing POST Method

The API documentation states a POST method is also available for status codes search. The current client only implements GET via `searchStatusCodes()`. The POST method would allow structured search queries consistent with other POST search endpoints.

### 9b. Missing `--offset` Flag

The status command accepts `--limit` but not `--offset`, preventing pagination through the full 241 status codes.

### 9c. No `--all` Convenience Flag

Since there are only 241 status codes total, an `--all` flag that sets `limit=500` would be useful for agents that want to dump the complete lookup table.

### Recommended Changes to `src/commands/status.ts`:

```typescript
app
  .command("status-codes")
  .alias("status")
  .description("Search patent application status codes")
  .argument("[query]", "Search query (code number or description text)")
  .option("-l, --limit <n>", "Max results", "25")
  .option("-o, --offset <n>", "Results offset for pagination", "0")
  .option("--all", "Return all status codes (overrides limit)")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
  .action(async (query, opts) => {
    const limit = opts.all ? 500 : parseInt(opts.limit);
    const offset = parseInt(opts.offset);
    // ...
  });
```

---

## 10. Cross-Cutting Issues

### 10a. Inconsistent `--format` Flag Coverage

| Subcommand | Has `--format` flag | Has table formatter |
|------------|-------------------|-------------------|
| `app get` | YES | YES |
| `app meta` | YES | YES |
| `app docs` | YES | YES |
| `app transactions` | YES | YES |
| `app continuity` | YES | YES |
| `app assignments` | YES | YES |
| `app attorney` | **NO** | **NO** |
| `app adjustment` / `pta` | **NO** | **NO** |
| `app foreign-priority` / `fp` | **NO** | **NO** |
| `app associated-docs` / `xml` | **NO** | **NO** |
| `app extension` / `pte` | **MISSING** | **MISSING** |
| `status-codes` | YES | YES |

**Every subcommand should support `-f, --format <fmt>` with options `table` and `json`.**

### 10b. Client Methods Return `Promise<any>`

The following client methods return untyped `Promise<any>` instead of proper response types:

| Method | Current Return | Should Return |
|--------|---------------|---------------|
| `getMetadata()` | `Promise<any>` | `Promise<PatentDataResponse>` |
| `getAdjustment()` | `Promise<any>` | `Promise<AdjustmentResponse>` |
| `getAssignment()` | `Promise<any>` | `Promise<AssignmentResponse>` |
| `getAttorney()` | `Promise<any>` | `Promise<AttorneyResponse>` |
| `getContinuity()` | `Promise<any>` | `Promise<ContinuityResponse>` |
| `getForeignPriority()` | `Promise<any>` | `Promise<ForeignPriorityResponse>` |
| `getTransactions()` | `Promise<any>` | `Promise<TransactionsResponse>` |
| `getAssociatedDocuments()` | `Promise<any>` | `Promise<AssociatedDocumentsResponse>` |
| *(missing)* `getExtension()` | N/A | `Promise<ExtensionResponse>` |

Each of these should have a corresponding response wrapper type:

```typescript
export interface PfwSubEndpointResponse<T> {
  count: number;
  patentFileWrapperDataBag: Array<{
    applicationNumberText: string;
    requestIdentifier?: string;
  } & T>;
  requestIdentifier?: string;
}

// Example usage:
export type ContinuityResponse = PfwSubEndpointResponse<{
  parentContinuityBag: ParentContinuityEntry[];
  childContinuityBag: ChildContinuityEntry[];
}>;

export type AssignmentResponse = PfwSubEndpointResponse<{
  assignmentBag: Assignment[];
}>;

export type ForeignPriorityResponse = PfwSubEndpointResponse<{
  foreignPriorityBag: ForeignPriorityEntry[];
}>;

export type AdjustmentResponse = PfwSubEndpointResponse<{
  patentTermAdjustmentData: PatentTermAdjustmentData;
}>;

export type ExtensionResponse = PfwSubEndpointResponse<{
  patentTermExtensionData: PatentTermExtensionData;
}>;

export type AttorneyResponse = PfwSubEndpointResponse<{
  recordAttorney: RecordAttorney;
}>;

export type AssociatedDocumentsResponse = PfwSubEndpointResponse<{
  grantDocumentMetaData?: FileMetaData;
  pgpubDocumentMetaData?: FileMetaData;
}>;
```

### 10c. Missing `requestIdentifier` Handling

Every API response includes a `requestIdentifier` field. This is useful for debugging and for referencing specific API calls in support tickets. The CLI currently discards this in table mode. Consider adding a `--verbose` or `--debug` mode that prints the request identifier.

### 10d. Missing Error Context in Sub-Endpoint Responses

When a sub-endpoint returns 404 (e.g., no continuity data for an application), the error message is generic. Each command should produce a specific, actionable error message. For example:

```
No continuity data found for application 18045436.
This application may not have parent or child relationships.
```

### 10e. Agent-Friendly Formatting Improvements

For AI agent consumption, the following improvements would help:

1. **Structured header lines**: Begin each table output with a machine-parseable header like `## CONTINUITY: 18045436` so agents can identify sections.

2. **Count lines**: Always print a count line like `Found 3 parent applications, 1 child application` before the table.

3. **URI fields in table mode**: When a record contains a downloadable URI (assignment documents, associated document XMLs), append a line like `Download URI: https://...` after the table, so agents can extract it without switching to JSON mode.

4. **Date consistency**: All dates should be rendered in `YYYY-MM-DD` format consistently (some responses include `T` timestamps that are stripped inconsistently).

---

## Appendix A: Complete Implementation Priority Matrix

| Task | Files to Change | Priority | Effort |
|------|----------------|----------|--------|
| Add `getExtension()` client method | `client.ts` | CRITICAL | Small |
| Add `extension`/`pte` command | `app.ts` | CRITICAL | Small |
| Add `PatentTermExtensionData` type | `api.ts` | CRITICAL | Small |
| Add `formatExtensionTable()` | `format.ts` | CRITICAL | Medium |
| Fix assignee name field bug (`assigneeNameText`) | `format.ts` line 177 | CRITICAL | Trivial |
| Add `formatForeignPriorityTable()` | `format.ts` | HIGH | Small |
| Add `formatAttorneyTable()` | `format.ts` | HIGH | Medium |
| Add `formatAdjustmentTable()` | `format.ts` | HIGH | Medium |
| Add `formatAssociatedDocsTable()` | `format.ts` | HIGH | Small |
| Add `--format` flag to `attorney`, `adjustment`, `fp`, `xml` commands | `app.ts` | HIGH | Small |
| Type all `Promise<any>` client methods | `client.ts` | HIGH | Medium |
| Add sub-endpoint response types | `api.ts` | HIGH | Medium |
| Add `ipOfficeAdjustmentDelayQuantity` to PTA type | `api.ts` | MEDIUM | Trivial |
| Type `patentTermAdjustmentHistoryDataBag` | `api.ts` | MEDIUM | Small |
| Add `Assignor`, `Assignee`, `AssigneeAddress` types | `api.ts` | MEDIUM | Small |
| Add `CorrespondenceAddress`, `DomesticRepresentative` types | `api.ts` | MEDIUM | Small |
| Add `RecordAttorney`, `AttorneyEntry` types | `api.ts` | MEDIUM | Small |
| Add `ForeignPriorityEntry` type | `api.ts` | MEDIUM | Trivial |
| Split `ContinuityData` into parent/child types | `api.ts` | MEDIUM | Small |
| Add cross-reference columns to continuity table | `format.ts` | MEDIUM | Small |
| Add `download-xml` command for associated docs | `app.ts` | MEDIUM | Medium |
| Add `assignment-download` command | `app.ts` | MEDIUM | Medium |
| Add `--offset` to status-codes command | `status.ts` | LOW | Trivial |
| Add `--all` to status-codes command | `status.ts` | LOW | Trivial |
| Add POST search for status codes | `client.ts` | LOW | Small |
| Add agent-friendly count headers to all formatters | `format.ts` | LOW | Medium |

## Appendix B: Endpoint Coverage Summary

| API Endpoint | Client Method | Command | Types | Formatter | Overall |
|-------------|--------------|---------|-------|-----------|---------|
| `/{app}/continuity` | YES | YES | Partial | YES (partial) | 70% |
| `/{app}/assignment` | YES | YES | Partial | YES (buggy) | 60% |
| `/{app}/foreign-priority` | YES | YES | NO | NO | 30% |
| `/{app}/associated-documents` | YES | YES | Partial | NO | 30% |
| `/{app}/adjustment` | YES | YES | Partial | NO | 40% |
| `/{app}/extension` | **NO** | **NO** | **NO** | **NO** | **0%** |
| `/{app}/attorney` | YES | YES | NO | NO | 30% |
| `/status-codes` GET | YES | YES | YES | YES | 90% |
| `/status-codes` POST | NO | NO | N/A | N/A | 0% |

