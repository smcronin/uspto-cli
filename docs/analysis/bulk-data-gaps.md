# Bulk Datasets API -- Gap Analysis

Generated: 2026-02-28

Compares the USPTO Open Data Portal Bulk Datasets API documentation against the current
CLI implementation (`src/commands/bulk.ts`, `src/api/client.ts`, `src/types/api.ts`,
`src/utils/format.ts`).

---

## Table of Contents

1. [Endpoint Coverage Summary](#1-endpoint-coverage-summary)
2. [Missing: `bulk download` Command (Entire Endpoint)](#2-missing-bulk-download-command-entire-endpoint)
3. [Search Endpoint Gaps](#3-search-endpoint-gaps)
4. [Product Data Endpoint Gaps](#4-product-data-endpoint-gaps)
5. [Type Definition Gaps](#5-type-definition-gaps)
6. [Formatter Gaps](#6-formatter-gaps)
7. [API Client Gaps](#7-api-client-gaps)
8. [Agent Workflow Improvements](#8-agent-workflow-improvements)
9. [Prioritized Implementation Plan](#9-prioritized-implementation-plan)

---

## 1. Endpoint Coverage Summary

| API Endpoint | URL Pattern | CLI Command | Status |
|---|---|---|---|
| Search | `GET /api/v1/datasets/products/search` | `bulk search [query]` | Partial |
| Product Data | `GET /api/v1/datasets/products/{productIdentifier}` | `bulk get <productId>` | Partial |
| Download | `GET /api/v1/datasets/products/files/{productIdentifier}/{fileName}` | **MISSING** | Not implemented |

The Download endpoint is completely absent from the CLI. The Search and Product Data endpoints
exist but have significant field and flag gaps.

---

## 2. Missing: `bulk download` Command (Entire Endpoint)

### What the API provides

```
GET /api/v1/datasets/products/files/{productIdentifier}/{fileName}
```

Returns a binary stream of data. Key constraints from the docs:
- Limited to **20 downloads of the same file per year** per API key (HTTP 429 on the 21st).
- Returns a binary stream (the actual file content, e.g., a ZIP file).
- Requires the API key header.

### What the CLI has

The `downloadDocument()` method in `client.ts` (line 341) is a generic binary downloader that
follows redirects and writes to disk. However:
- There is **no `bulk download` subcommand** registered in `bulk.ts`.
- There is **no `downloadBulkFile()` client method** that constructs the correct URL.
- There is **no workflow** that discovers a file via search/product-data and then downloads it.

### What to implement

#### 2a. Add `downloadBulkFile()` to the API client

```typescript
// In src/api/client.ts, add to the "Bulk Data Endpoints" section:

async downloadBulkFile(
  productIdentifier: string,
  fileName: string,
  outputPath: string,
  onProgress?: (bytesDownloaded: number, totalBytes: number | null) => void
): Promise<string> {
  await this.rateLimiter.waitForSlot();

  const url = `${this.config.baseUrl}/api/v1/datasets/products/files/${
    encodeURIComponent(productIdentifier)
  }/${encodeURIComponent(fileName)}`;

  if (this.config.debug) {
    console.error(`[DEBUG] DOWNLOAD ${url}`);
  }

  const response = await fetch(url, {
    headers: {
      "X-API-KEY": this.config.apiKey,
      Accept: "application/octet-stream",
    },
    redirect: "follow",
  });

  this.rateLimiter.markRequestComplete();

  if (!response.ok) {
    if (response.status === 429) {
      this.rateLimiter.markRateLimited();
      throw new Error(
        `Download rate limited (HTTP 429). You may have exceeded the 20-download-per-year ` +
        `limit for file "${fileName}" under product "${productIdentifier}".`
      );
    }
    throw new Error(`Bulk download failed: HTTP ${response.status} ${response.statusText}`);
  }

  const contentLength = response.headers.get("content-length");
  const totalBytes = contentLength ? parseInt(contentLength, 10) : null;

  const { createWriteStream } = await import("fs");
  const { pipeline } = await import("stream/promises");
  const { Readable } = await import("stream");

  const fileStream = createWriteStream(outputPath);
  const body = response.body;

  if (!body) {
    throw new Error("Response body is null");
  }

  // Stream the download to disk with optional progress callback
  const reader = body.getReader();
  let bytesDownloaded = 0;
  const readable = new Readable({
    async read() {
      const { done, value } = await reader.read();
      if (done) {
        this.push(null);
        return;
      }
      bytesDownloaded += value.length;
      if (onProgress) {
        onProgress(bytesDownloaded, totalBytes);
      }
      this.push(Buffer.from(value));
    },
  });

  await pipeline(readable, fileStream);
  return outputPath;
}
```

#### 2b. Add `bulk download` subcommand

```typescript
// In src/commands/bulk.ts, add after the "get" subcommand:

bulk
  .command("download")
  .alias("dl")
  .description("Download a bulk data file")
  .argument("<productId>", "Product identifier (e.g., PTFWPRE)")
  .argument("<fileName>", "File name to download")
  .option("-o, --output <path>", "Output file path (default: ./<fileName>)")
  .action(async (productId, fileName, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const outputPath = opts.output || `./${fileName}`;

    const { mkdirSync } = await import("fs");
    const { dirname } = await import("path");
    mkdirSync(dirname(outputPath), { recursive: true });

    console.log(`Downloading ${productId}/${fileName}...`);
    console.log(`Rate limit: 20 downloads per year per API key for this file.\n`);

    const saved = await client.downloadBulkFile(productId, fileName, outputPath,
      (downloaded, total) => {
        const pct = total ? ` (${((downloaded / total) * 100).toFixed(1)}%)` : "";
        const mb = (downloaded / 1024 / 1024).toFixed(1);
        process.stderr.write(`\r  ${mb} MB downloaded${pct}`);
      }
    );

    console.log(`\nSaved to: ${saved}`);
  });
```

#### 2c. Add `bulk files` subcommand (list downloadable files for a product)

```typescript
// This is a convenience command that fetches a product and lists its files,
// giving the user the exact fileName argument they need for `bulk download`.

bulk
  .command("files")
  .description("List downloadable files for a bulk data product")
  .argument("<productId>", "Product identifier (e.g., PTFWPRE)")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
  .action(async (productId, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.getBulkDataProduct(productId, { includeFiles: true });

    if (opts.format === "json") {
      console.log(formatOutput(result, "json"));
    } else {
      const product = result.bulkDataProductBag?.[0] || result;
      const files = product.productFileBag?.fileDataBag || [];
      console.log(`\n${product.productTitleText || productId}`);
      console.log(`${files.length} files available\n`);
      console.log(formatBulkFileTable(files));
      console.log(`\nTo download: uspto bulk download ${productId} <fileName>`);
    }
  });
```

---

## 3. Search Endpoint Gaps

### 3a. Missing search filter flags

The API docs state: "All search syntaxes are applicable to this endpoint, meaning any number
of combinations is possible." The API supports query parameter `q` with the full simplified
syntax. The current CLI only exposes a bare `[query]` argument, `--limit`, `--offset`, and
`--format`.

The following searchable fields from the response schema are not exposed as flags:

| API Field | Suggested Flag | Description |
|---|---|---|
| `productTitleText` | `--title <text>` | Filter by product title (e.g., "Patent File Wrapper") |
| `productFrequencyText` | `--frequency <freq>` | Filter by update frequency: WEEKLY, DAILY, etc. |
| `productLabelArrayText` | `--label <label>` | Filter by label: PATENT, TRADEMARK, RESEARCH |
| `productDatasetArrayText` | `--dataset <type>` | Filter by dataset type |
| `productDatasetCategoryArrayText` | `--category <cat>` | Filter by dataset category |
| `mimeTypeIdentifierArrayText` | `--mime <type>` | Filter by file type: JSON, XML, PDF |
| `productFromDate` / `productToDate` | `--from <date>` / `--to <date>` | Date range filter |

**Implementation:**

```typescript
// In src/commands/bulk.ts, update the search command:

bulk
  .command("search")
  .description("Search bulk data products")
  .argument("[query]", "Search query (uses USPTO simplified syntax)")
  .option("-l, --limit <n>", "Max results", "25")
  .option("-o, --offset <n>", "Starting offset", "0")
  .option("--title <text>", "Filter by product title")
  .option("--frequency <freq>", "Filter by frequency: WEEKLY, DAILY, MONTHLY, ANNUAL")
  .option("--label <label>", "Filter by label: PATENT, TRADEMARK, RESEARCH")
  .option("--category <cat>", "Filter by dataset category")
  .option("--mime <type>", "Filter by MIME type: JSON, XML, PDF")
  .option("--from <date>", "Products valid from date (yyyy-MM-dd)")
  .option("--to <date>", "Products valid to date (yyyy-MM-dd)")
  .option("--sort <field>", "Sort field (e.g., lastModifiedDateTime:desc)")
  .option("-f, --format <fmt>", "Output format: table, json", "table")
  .action(async (query, opts) => {
    // Build the query string with filter syntax
    let q = query || "";
    if (opts.title) q += ` productTitleText:("${opts.title}")`;
    if (opts.frequency) q += ` productFrequencyText:${opts.frequency}`;
    if (opts.label) q += ` productLabelArrayText:${opts.label}`;
    if (opts.category) q += ` productDatasetCategoryArrayText:("${opts.category}")`;
    if (opts.mime) q += ` mimeTypeIdentifierArrayText:${opts.mime}`;
    // Date range uses rangeFilter syntax: field:[from TO to]
    if (opts.from || opts.to) {
      const from = opts.from || "*";
      const to = opts.to || "*";
      q += ` productFromDate:[${from} TO ${to}]`;
    }

    const client = createClient({ debug: program.opts().debug });
    const result = await client.searchBulkData(q.trim() || undefined, {
      limit: parseInt(opts.limit),
      offset: parseInt(opts.offset),
      sort: opts.sort,
    });

    if (opts.format === "json") {
      console.log(formatOutput(result, "json"));
    } else {
      console.log(`\n${result.count} bulk data products found\n`);
      console.log(formatBulkDataTable(result.bulkDataProductBag));
    }
  });
```

### 3b. Missing sort support in the API client

The `searchBulkData()` method in `client.ts` does not pass a `sort` parameter, though the
API supports it. The fields most useful for sorting are `lastModifiedDateTime`,
`productTotalFileSize`, and `productFileTotalQuantity`.

```typescript
// In src/api/client.ts, update searchBulkData:

async searchBulkData(
  query?: string,
  opts: { limit?: number; offset?: number; sort?: string } = {}
): Promise<BulkDataResponse> {
  const params: Record<string, string> = {};
  if (query) params.q = query;
  if (opts.limit) params.limit = String(opts.limit);
  if (opts.offset) params.offset = String(opts.offset);
  if (opts.sort) params.sort = opts.sort;  // <-- ADD THIS
  return this.request<BulkDataResponse>("GET", "/api/v1/datasets/products/search", { params });
}
```

### 3c. Missing fields support

The API docs state: "If you don't specify which attributes you would like to see in the
response related to the search term(s), it returns all data attributes." This means the API
supports a `fields` parameter for projection. The client does not expose this.

```typescript
// Add fields support to searchBulkData:
if (opts.fields) params.fields = opts.fields;
```

---

## 4. Product Data Endpoint Gaps

### 4a. The `bulk get` command does not display structured data

Currently `bulk get` outputs raw JSON via `formatOutput(result, opts.format)`. Even when
`--format table` could be used, there is no table formatter for the product detail view.

The Product Data response includes significant detail that should be rendered:

| API Field | Currently Displayed | Status |
|---|---|---|
| `productIdentifier` | Only in JSON | Not formatted |
| `productTitleText` | Only in JSON | Not formatted |
| `productDescriptionText` | Only in JSON | Not formatted |
| `productFrequencyText` | Only in JSON | Not formatted |
| `daysOfWeekText` | Only in JSON | **Ignored** in type definition |
| `productLabelArrayText` | Only in JSON | Not formatted |
| `productFromDate` | Only in JSON | Not formatted |
| `productToDate` | Only in JSON | Not formatted |
| `productTotalFileSize` | Only in JSON | Not formatted |
| `productFileTotalQuantity` | Only in JSON | Not formatted |
| `lastModifiedDateTime` | Only in JSON | Not formatted |
| `mimeTypeIdentifierArrayText` | Only in JSON | Not formatted |
| `productFileBag` | Only in JSON | Not formatted |
| `productFileBag.fileDataBag[].fileDownloadURI` | Only in JSON | **Critical for download workflow** |

### 4b. Implement `formatBulkProductDetail()` formatter

```typescript
// In src/utils/format.ts, add:

export function formatBulkProductDetail(product: BulkDataProduct): string {
  const sizeMB = product.productTotalFileSize
    ? `${(product.productTotalFileSize / 1024 / 1024).toFixed(1)} MB`
    : "-";

  const lines = [
    "",
    chalk.bold.white(`  ${product.productTitleText || "Unknown"}`),
    "",
    `  ${chalk.gray("Product ID:")}     ${product.productIdentifier}`,
    `  ${chalk.gray("Description:")}    ${product.productDescriptionText || "-"}`,
    `  ${chalk.gray("Frequency:")}      ${product.productFrequencyText || "-"}`,
    `  ${chalk.gray("Release Day:")}    ${product.daysOfWeekText || "-"}`,
    `  ${chalk.gray("Valid From:")}     ${product.productFromDate || "-"}`,
    `  ${chalk.gray("Valid To:")}       ${product.productToDate || "-"}`,
    `  ${chalk.gray("Total Size:")}     ${sizeMB}`,
    `  ${chalk.gray("File Count:")}     ${product.productFileTotalQuantity || 0}`,
    `  ${chalk.gray("Last Modified:")}  ${product.lastModifiedDateTime || "-"}`,
    `  ${chalk.gray("Labels:")}         ${(product.productLabelArrayText || []).flat().join(", ") || "-"}`,
    `  ${chalk.gray("Datasets:")}       ${(product.productDataSetArrayText || []).flat().join(", ") || "-"}`,
    `  ${chalk.gray("Categories:")}     ${(product.productDataSetCategoryArrayText || []).flat().join(", ") || "-"}`,
    `  ${chalk.gray("MIME Types:")}     ${(product.mimeTypeIdentifierArrayText || []).flat().join(", ") || "-"}`,
    "",
  ];

  // Append file listing
  const files = product.productFileBag?.fileDataBag;
  if (files?.length) {
    lines.push(chalk.bold("  Files:"));
    lines.push("");
    lines.push(formatBulkFileTable(files));
    lines.push("");
  }

  return lines.join("\n");
}
```

### 4c. Implement `formatBulkFileTable()` formatter

```typescript
// In src/utils/format.ts, add:

export function formatBulkFileTable(files: BulkFileData[]): string {
  if (!files?.length) return chalk.yellow("No files found.");

  const table = new Table({
    head: [
      chalk.cyan("#"),
      chalk.cyan("File Name"),
      chalk.cyan("Size"),
      chalk.cyan("Type"),
      chalk.cyan("Data Range"),
      chalk.cyan("Released"),
    ],
    colWidths: [4, 50, 12, 8, 25, 13],
    wordWrap: true,
  });

  files.forEach((f, i) => {
    const sizeMB = f.fileSize ? `${(f.fileSize / 1024 / 1024).toFixed(1)} MB` : "-";
    const range = f.fileDataFromDate && f.fileDataToDate
      ? `${f.fileDataFromDate} - ${f.fileDataToDate}`
      : "-";
    const released = f.fileReleaseDate ? f.fileReleaseDate.split(" ")[0] : "-";

    table.push([
      i + 1,
      f.fileName || "",
      sizeMB,
      f.fileTypeText || "",
      range,
      released,
    ]);
  });

  return table.toString();
}
```

### 4d. Update `bulk get` to use the new formatter

```typescript
// In src/commands/bulk.ts, update the "get" action:

.action(async (productId, opts) => {
  const client = createClient({ debug: program.opts().debug });
  const result = await client.getBulkDataProduct(productId, {
    includeFiles: opts.includeFiles,
    latest: opts.latest,
  });

  if (opts.format === "json") {
    console.log(formatOutput(result, "json"));
  } else {
    const product = result.bulkDataProductBag?.[0] || result;
    console.log(formatBulkProductDetail(product));
  }
});
```

---

## 5. Type Definition Gaps

### 5a. Nested array type mismatch

The API response shows `productLabelArrayText`, `productDatasetArrayText`,
`productDatasetCategoryArrayText`, and `mimeTypeIdentifierArrayText` as **nested arrays**
(array of arrays) in the search response, but **flat arrays** in the product data response.

Current type definition in `api.ts` line 226-240:

```typescript
// CURRENT (incorrect for search response):
productLabelArrayText: string[];
productDataSetArrayText: string[];
productDataSetCategoryArrayText: string[];
mimeTypeIdentifierArrayText: string[];
```

The search response sample shows:
```json
"productLabelArrayText": [["RESEARCH", "PATENT"]]
```

**Fix:** Change the type to handle both forms:

```typescript
productLabelArrayText: string[] | string[][];
productDatasetArrayText: string[] | string[][];
productDatasetCategoryArrayText: string[] | string[][];
mimeTypeIdentifierArrayText: string[] | string[][];
```

### 5b. Field name casing mismatch

The API docs use `productDatasetArrayText` and `productDatasetCategoryArrayText` but the
type definition at line 233-234 uses `productDataSetArrayText` and
`productDataSetCategoryArrayText` (capital S in Set). The formatter at line 222 references
the type but the API might return either. The API JSON samples use lowercase "set".

**Fix:** Either use the lowercase version or define both:

```typescript
// Change to match API docs exactly:
productDatasetArrayText: string[] | string[][];
productDatasetCategoryArrayText: string[] | string[][];
```

### 5c. Missing `daysOfWeekText` from search response

The `BulkDataProduct` interface does have `daysOfWeekText: string` (line 231), so this is
correct. However, the `formatBulkDataTable` does not render it. The search endpoint returns
this field and it is useful for knowing when files are released.

### 5d. `bulkDataProductBag` nested array in search response

The API search response sample shows `bulkDataProductBag` as a **doubly nested array**:

```json
"bulkDataProductBag": [[{ ... }]]
```

But the `BulkDataResponse` type at line 247-251 defines it as:

```typescript
bulkDataProductBag: BulkDataProduct[];
```

This may cause runtime issues if the API returns `[[...]]` vs `[...]`. The formatter and
command code both iterate over `bulkDataProductBag` directly. If the API actually returns a
nested array, products will be arrays-of-objects rather than objects.

**Fix:** Add a normalization helper:

```typescript
// In src/api/client.ts or a utility module:
function normalizeBulkProducts(bag: any): BulkDataProduct[] {
  if (!bag) return [];
  // Handle [[{...}], [{...}]] -> [{...}, {...}]
  return bag.flat(Infinity).filter((item: any) => item && typeof item === "object");
}
```

---

## 6. Formatter Gaps

### 6a. `formatBulkDataTable` is missing key columns

The current table in `format.ts` (line 213-234) only shows:

| Column | Source Field |
|---|---|
| ID | `productIdentifier` |
| Title | `productTitleText` |
| Freq | `productFrequencyText` |
| Files | `productFileTotalQuantity` |
| Size | `productTotalFileSize` |

**Missing columns that would be valuable:**

| Field | Why It Matters |
|---|---|
| `lastModifiedDateTime` | Tells the user how recent the data is |
| `daysOfWeekText` | When new files are released |
| `mimeTypeIdentifierArrayText` | What file formats are available (JSON, XML, PDF) |
| `productLabelArrayText` | Whether it is PATENT vs TRADEMARK data |
| `productDatasetCategoryArrayText` | The category of data |

**Recommendation:** Add `--verbose` flag to show an expanded table, or add a `Last Updated`
column to the default view since it is the most operationally important missing field:

```typescript
// Updated formatBulkDataTable with Last Updated:
export function formatBulkDataTable(products: BulkDataProduct[]): string {
  if (!products?.length) return chalk.yellow("No bulk data products found.");

  const table = new Table({
    head: [
      chalk.cyan("ID"),
      chalk.cyan("Title"),
      chalk.cyan("Freq"),
      chalk.cyan("Types"),
      chalk.cyan("Files"),
      chalk.cyan("Size"),
      chalk.cyan("Updated"),
    ],
    colWidths: [15, 40, 10, 10, 7, 12, 13],
    wordWrap: true,
  });

  for (const p of products) {
    const sizeMB = p.productTotalFileSize
      ? `${(p.productTotalFileSize / 1024 / 1024).toFixed(0)} MB`
      : "-";
    const mimeTypes = (p.mimeTypeIdentifierArrayText || []).flat().join(", ");
    const updated = p.lastModifiedDateTime
      ? p.lastModifiedDateTime.split("T")[0].split(" ")[0]
      : "-";

    table.push([
      p.productIdentifier || "",
      (p.productTitleText || "").substring(0, 50),
      p.productFrequencyText || "",
      mimeTypes || "-",
      p.productFileTotalQuantity || 0,
      sizeMB,
      updated,
    ]);
  }

  return table.toString();
}
```

### 6b. No `BulkFileData` import in format.ts

The `BulkFileData` type is defined in `api.ts` but never imported into `format.ts`. It will
be needed for the new `formatBulkFileTable` and `formatBulkProductDetail` functions.

```typescript
// In src/utils/format.ts, update the import:
import type {
  PatentFileWrapper, Document, ProceedingData, PetitionDecision,
  EventData, ContinuityData, Assignment, BulkDataProduct,
  BulkFileData,  // <-- ADD THIS
} from "../types/api";
```

---

## 7. API Client Gaps

### 7a. `getBulkDataProduct()` returns `any`

At line 228-233 in `client.ts`:

```typescript
async getBulkDataProduct(productId: string, opts: { includeFiles?: boolean; latest?: boolean } = {}): Promise<any> {
```

This should return `Promise<BulkDataResponse>` to match the return type from the API (same
schema as search, just with `count: 1`).

```typescript
async getBulkDataProduct(
  productId: string,
  opts: { includeFiles?: boolean; latest?: boolean } = {}
): Promise<BulkDataResponse> {
```

### 7b. `includeFiles` and `latest` params may not be real API params

The Product Data API docs show the endpoint as:

```
GET /api/v1/datasets/products/{productIdentifier}
```

The docs do not mention `includeFiles` or `latest` as query parameters. The `productFileBag`
with `fileDataBag` is always included in the response based on the sample JSON. The current
CLI passes these as query params but they may be silently ignored by the API.

**Recommendation:** Test whether these params actually work. If they do not, remove them and
always return the full file listing. If they do, document them as unofficial params.

### 7c. No dedicated download URL builder

The `fileDownloadURI` field in the API response provides full download URLs, e.g.:

```
https://api.uspto.gov/api/v1/datasets/products/files/PTFWPRE/2001-2010-patent-filewrapper-full-json.zip
```

The client should be able to download using either:
1. The full `fileDownloadURI` from the API response (already handled by `downloadDocument()`).
2. A constructed URL from `productIdentifier` + `fileName` (the dedicated download endpoint).

The `downloadDocument()` method already handles full URLs, but it uses JSON accept headers
which is wrong for binary downloads:

```typescript
// CURRENT (line 79-85 in client.ts):
private get headers(): Record<string, string> {
  return {
    "X-API-KEY": this.config.apiKey,
    "Content-Type": "application/json",
    Accept: "application/json",  // <-- WRONG for binary downloads
  };
}
```

The `downloadDocument()` method at line 344 uses `this.headers` which sets
`Accept: application/json`. For binary file downloads, this should be
`Accept: application/octet-stream` or `*/*`.

**Fix:**

```typescript
async downloadDocument(url: string, outputPath: string): Promise<string> {
  await this.rateLimiter.waitForSlot();

  const response = await fetch(url, {
    headers: {
      "X-API-KEY": this.config.apiKey,
      Accept: "*/*",  // <-- Accept binary content
    },
    redirect: "follow",
  });

  this.rateLimiter.markRequestComplete();
  // ... rest unchanged
}
```

### 7d. No retry logic for 429 on bulk downloads

The rate limit for bulk downloads is unique: 20 downloads per year per file per API key, plus
5 files per 10 seconds per IP. The current 429 handler in `downloadDocument()` just throws.
It should distinguish between:
- Transient rate limit (too many requests in a window) -- retryable after delay.
- Annual limit exceeded (20/year) -- not retryable, inform user clearly.

```typescript
async downloadBulkFile(productId: string, fileName: string, outputPath: string): Promise<string> {
  // ... fetch logic ...

  if (response.status === 429) {
    const retryAfter = response.headers.get("Retry-After");
    if (retryAfter) {
      const waitSec = parseInt(retryAfter, 10) || 10;
      console.error(`Rate limited. Retrying in ${waitSec}s...`);
      await new Promise(r => setTimeout(r, waitSec * 1000));
      return this.downloadBulkFile(productId, fileName, outputPath); // Retry once
    }
    throw new Error(
      `Download rate limited (HTTP 429). You may have hit the 20-download/year limit ` +
      `for ${productId}/${fileName}. Try a different API key or wait until next year.`
    );
  }
}
```

---

## 8. Agent Workflow Improvements

### 8a. End-to-end discovery-to-download workflow

An agent currently cannot perform the full bulk data workflow because the download command is
missing. The ideal workflow is:

```
# Step 1: Discover what products exist
$ uspto bulk search --label PATENT --mime XML

# Step 2: Get details and file list for a specific product
$ uspto bulk get PTGRXML
# or
$ uspto bulk files PTGRXML

# Step 3: Download a specific file
$ uspto bulk download PTGRXML pftaps20250101_wk01.zip -o ./data/

# Step 4: (Future) Extract and inspect
```

### 8b. Add `bulk list-categories` convenience command

```typescript
// Fetches all products and extracts unique categories, labels, and frequencies
bulk
  .command("categories")
  .description("List all available dataset categories, labels, and frequencies")
  .action(async () => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.searchBulkData(undefined, { limit: 100 });

    const categories = new Set<string>();
    const labels = new Set<string>();
    const frequencies = new Set<string>();

    for (const p of result.bulkDataProductBag) {
      (p.productDatasetCategoryArrayText || []).flat().forEach(c => categories.add(c));
      (p.productLabelArrayText || []).flat().forEach(l => labels.add(l));
      if (p.productFrequencyText) frequencies.add(p.productFrequencyText);
    }

    console.log("\nCategories:", [...categories].sort().join(", "));
    console.log("Labels:", [...labels].sort().join(", "));
    console.log("Frequencies:", [...frequencies].sort().join(", "));
  });
```

### 8c. Smart download from product lookup

Add a flag that lets the agent go from product ID directly to downloading the latest file:

```typescript
bulk
  .command("download-latest")
  .description("Download the most recently released file from a bulk data product")
  .argument("<productId>", "Product identifier")
  .option("-o, --output <dir>", "Output directory (default: .)")
  .action(async (productId, opts) => {
    const client = createClient({ debug: program.opts().debug });
    const result = await client.getBulkDataProduct(productId);
    const product = result.bulkDataProductBag?.[0];

    if (!product?.productFileBag?.fileDataBag?.length) {
      console.error("No files available for this product.");
      process.exit(1);
    }

    // Sort by fileReleaseDate descending to find the latest
    const files = [...product.productFileBag.fileDataBag].sort((a, b) =>
      (b.fileReleaseDate || "").localeCompare(a.fileReleaseDate || "")
    );
    const latest = files[0];

    const outDir = opts.output || ".";
    const outPath = `${outDir}/${latest.fileName}`;
    const { mkdirSync } = await import("fs");
    mkdirSync(outDir, { recursive: true });

    const sizeMB = latest.fileSize ? `${(latest.fileSize / 1024 / 1024).toFixed(1)} MB` : "unknown size";
    console.log(`Latest file: ${latest.fileName} (${sizeMB})`);
    console.log(`Released: ${latest.fileReleaseDate}`);
    console.log(`Data range: ${latest.fileDataFromDate} - ${latest.fileDataToDate}\n`);

    const saved = await client.downloadBulkFile(productId, latest.fileName, outPath);
    console.log(`Saved to: ${saved}`);
  });
```

### 8d. File type awareness

The API returns `mimeTypeIdentifierArrayText` which includes values like JSON, XML, PDF. The
CLI should surface this prominently since an agent needs to know the format before downloading
multi-gigabyte files. The `fileTypeText` field (e.g., "Data") is also available per file but
currently not displayed in any table.

### 8e. Annual download budget tracking

Since the API limits downloads to 20 per file per year per API key, the CLI should track
download history locally. A simple JSON file could store `{ [productId/fileName]: count }`.
This would let the agent check remaining budget before attempting a download.

```typescript
// Suggested file: src/utils/download-tracker.ts
import { readFileSync, writeFileSync, existsSync } from "fs";
import { join } from "path";

const TRACKER_PATH = join(process.env.HOME || ".", ".uspto-cli", "download-tracker.json");

interface DownloadRecord {
  [key: string]: { count: number; lastDownload: string };
}

export function getDownloadCount(productId: string, fileName: string): number {
  if (!existsSync(TRACKER_PATH)) return 0;
  const data: DownloadRecord = JSON.parse(readFileSync(TRACKER_PATH, "utf-8"));
  return data[`${productId}/${fileName}`]?.count || 0;
}

export function recordDownload(productId: string, fileName: string): void {
  const dir = join(process.env.HOME || ".", ".uspto-cli");
  const { mkdirSync } = require("fs");
  mkdirSync(dir, { recursive: true });

  let data: DownloadRecord = {};
  if (existsSync(TRACKER_PATH)) {
    data = JSON.parse(readFileSync(TRACKER_PATH, "utf-8"));
  }
  const key = `${productId}/${fileName}`;
  data[key] = {
    count: (data[key]?.count || 0) + 1,
    lastDownload: new Date().toISOString(),
  };
  writeFileSync(TRACKER_PATH, JSON.stringify(data, null, 2));
}

export function getRemainingDownloads(productId: string, fileName: string): number {
  return 20 - getDownloadCount(productId, fileName);
}
```

---

## 9. Prioritized Implementation Plan

### P0 -- Critical (blocks agent workflows)

1. **Add `bulk download` subcommand and `downloadBulkFile()` client method.**
   - Files: `src/commands/bulk.ts`, `src/api/client.ts`
   - The Download endpoint is completely unimplemented. Without it, the entire bulk data
     workflow is incomplete.

2. **Fix `Accept` header in `downloadDocument()` for binary downloads.**
   - File: `src/api/client.ts`, line 344
   - Currently sends `Accept: application/json` for binary file downloads.

3. **Fix `getBulkDataProduct()` return type from `any` to `BulkDataResponse`.**
   - File: `src/api/client.ts`, line 228

### P1 -- High (significantly improves usability)

4. **Add `bulk files` subcommand.**
   - File: `src/commands/bulk.ts`
   - Lets agents discover exact file names needed for the download command.

5. **Add `formatBulkProductDetail()` and `formatBulkFileTable()` formatters.**
   - File: `src/utils/format.ts`
   - The `bulk get` command currently dumps raw JSON even in table mode.

6. **Add `sort` parameter support to `searchBulkData()`.**
   - File: `src/api/client.ts`, line 220-226

7. **Add search filter flags (`--title`, `--frequency`, `--label`, `--mime`, `--category`).**
   - File: `src/commands/bulk.ts`

8. **Add `lastModifiedDateTime` and `mimeTypeIdentifierArrayText` columns to the search
   table.**
   - File: `src/utils/format.ts`

### P2 -- Medium (improves reliability and correctness)

9. **Fix nested array types for `productLabelArrayText` etc.**
   - File: `src/types/api.ts`

10. **Fix field name casing (`productDataSetArrayText` vs `productDatasetArrayText`).**
    - File: `src/types/api.ts`

11. **Add `normalizeBulkProducts()` helper for doubly-nested `bulkDataProductBag`.**
    - File: `src/api/client.ts` or new utility

12. **Validate whether `includeFiles` and `latest` are real API params.**
    - File: `src/api/client.ts`, line 228-233

13. **Differentiate transient vs annual 429 errors for bulk downloads.**
    - File: `src/api/client.ts`

### P3 -- Nice to have (agent convenience)

14. **Add `bulk download-latest` subcommand.**
    - File: `src/commands/bulk.ts`

15. **Add `bulk categories` subcommand.**
    - File: `src/commands/bulk.ts`

16. **Implement local download budget tracker.**
    - New file: `src/utils/download-tracker.ts`

17. **Add streaming download with progress reporting.**
    - File: `src/api/client.ts`

---

## Appendix A: Available File Types from API

Based on the API documentation and response samples, bulk data files include:

| MIME Type | Typical Extensions | Example Products |
|---|---|---|
| JSON | `.json`, `.zip` (containing JSON) | Patent File Wrapper Weekly |
| XML | `.xml`, `.zip` (containing XML) | Patent Assignment XML, Grant XML |
| PDF | `.pdf` | Various document collections |

The `mimeTypeIdentifierArrayText` field on each product tells you what formats are available.
Individual files within a product have a `fileTypeText` field (e.g., "Data").

## Appendix B: Rate Limit Summary for Bulk Downloads

| Limit | Value | Scope |
|---|---|---|
| Per-file annual limit | 20 downloads/year | Per API key per file |
| Burst rate | 5 files per 10 seconds | Per IP address |
| 429 retry wait | At least 5 seconds | Per request |
| Burst concurrency | 1 (sequential only) | Per API key |

The per-file annual limit (20/year) is the most unusual constraint. On the 21st request for
the same file in a calendar year, the API returns HTTP 429. This cannot be retried -- the user
must wait until the next year or use a different API key. The CLI should track and warn about
this proactively.

## Appendix C: Complete Response Field Inventory

### Search / Product Data response fields vs. current usage

| Field | In Type? | In Formatter? | In CLI Flag? | Gap |
|---|---|---|---|---|
| `productIdentifier` | Yes | Table: Yes | No | Could be a filter flag |
| `productDescriptionText` | Yes | No | No | Not displayed in table view |
| `productTitleText` | Yes | Table: Yes | No | Should be a `--title` filter |
| `productFrequencyText` | Yes | Table: Yes | No | Should be a `--frequency` filter |
| `productFromDate` | Yes | No | No | Should be a `--from` filter |
| `productToDate` | Yes | No | No | Should be a `--to` filter |
| `productTotalFileSize` | Yes | Table: Yes | No | -- |
| `productFileTotalQuantity` | Yes | Table: Yes | No | -- |
| `lastModifiedDateTime` | Yes | No | No | Should be in default table |
| `mimeTypeIdentifierArrayText` | Yes | No | No | Should be in table + filter flag |
| `daysOfWeekText` | Yes | No | No | Should be in detail view |
| `productLabelArrayText` | Yes (wrong type) | No | No | Type is wrong; should be filter |
| `productDatasetArrayText` | Yes (wrong name) | No | No | Field name casing mismatch |
| `productDatasetCategoryArrayText` | Yes (wrong name) | No | No | Field name casing mismatch |
| `productFileBag.count` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileName` | Yes | No | No | Needed for download command |
| `productFileBag.fileDataBag[].fileSize` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileDataFromDate` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileDataToDate` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileTypeText` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileDownloadURI` | Yes | No | No | Critical for download |
| `productFileBag.fileDataBag[].fileReleaseDate` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileDate` | Yes | No | No | -- |
| `productFileBag.fileDataBag[].fileLastModifiedDateTime` | Yes | No | No | -- |
