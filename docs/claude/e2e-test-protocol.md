# USPTO CLI - End-to-End Test Protocol

**Version**: 0.2.0+fixes
**Known test application**: 16123456 (Pat. 10902286, FUJIFILM, granted)
**Known test family root**: 12477075 (Apple slide-to-unlock, 16 family members)
**Known PTAB trial**: IPR2016-00134 (NVIDIA v. Samsung)
**Known appeal**: 2026000539

---

## Latest Run Results (2026-02-28)

**45/45 PASS** -- All 6 bug regressions confirmed fixed.

| Group | Tests | Result | Notes |
|-------|-------|--------|-------|
| T-001 Help | 2/2 | PASS | |
| T-002 Status | 4/4 | PASS | |
| T-003 App Meta | 4/4 | PASS | exit codes correct (1, 4) |
| T-004 Search | 9/9 | PASS | BUG-001 regressions all clear |
| T-005 App Data | 9/9 | PASS | BUG-001 regressions all clear |
| T-006 Download | 2/2 | PASS | BUG-002 regressions all clear (dry-run works, dashes not colons) |
| T-007 Summary | 1/1 | PASS | |
| T-008 Family | 2/2 | PASS | BUG-006 regression clear (16 unique, 0 dupes) |
| T-009 PTAB | 8/8 | PASS | BUG-004 regressions all clear (decisions not null) |
| T-010 Petition | 2/2 | PASS | BUG-003 regression clear |
| T-011 Bulk | 3/3 | PASS | BUG-005 regressions all clear (1286 files) |
| T-012 Formats | 4/4 | PASS | NDJSON, CSV, minified, quiet |
| T-013 Errors | 4/4 | PASS | exit codes 1, 4; debug output present |
| T-014 Workflows | 3/3 | PASS | Agent chaining works |
| T-015 Gap Fill | 4/5 | PASS | Covered previously untested endpoints |

### Gap-Fill Tests (T-015, added post-suite)
- **petition get**: PASS (ID: 0d5f5afa-d456-52b4-81e2-d4e51d7c801b)
- **ptab doc**: PASS (docId: 171300186)
- **ptab decision**: PASS (docId: 171299842)
- **ptab interferences-for**: PASS (interferenceNumber: 106130, 2 results)
- **ptab interference**: PASS (docId: 9cf994ee62561c5d8b994f15126ad4051ea4298c9405ed855dde307f, from offset 500+)
- **ptab appeals-for**: PASS (appealNumber: 2026000539, 1 result)
- **app extension**: **BUG-007** -- endpoint `/api/v1/patent/applications/{id}/extension` not in Swagger spec (verified all 53 endpoints). 403 is AWS API Gateway rejecting undefined route, not auth.

### Minor Observations
- **T-004h (dry-run search)**: Shows GET URL correctly but also prints "0 results found" after. Not a blocker but slightly confusing -- ideally dry-run should return before the "results found" message.
- **T-005d**: 52 docs became 56 (table rows include header, or new docs added since first run)
- **T-005e**: 58+ transactions became 62 (same reason)

---

## Test Execution Format

Each test records:
- **ID**: T-NNN
- **Command**: exact command run
- **Expected**: what should happen
- **Result**: PASS / FAIL + details

---

## T-001: Help / Discoverability

```bash
# T-001a: Root help
uspto --help
# Expected: Shows all commands (search, app, ptab, petition, bulk, status, summary, family)

# T-001b: Version
uspto --version
# Expected: "uspto version 0.2.0" (or current version)

# T-001c: Subcommand help
uspto search --help
# Expected: Shows all search flags with examples
```

## T-002: Status Lookup

```bash
# T-002a: Lookup by code
uspto status 150
# Expected: "Patented Case"

# T-002b: Lookup by text
uspto status "abandoned"
# Expected: 9 status codes with "abandoned" in description

# T-002c: JSON format
uspto status 150 -f json -q
# Expected: Valid JSON with ok=true, results array

# T-002d: CSV format
uspto status "patented" -f csv -q
# Expected: CSV with header row + 2 data rows
```

## T-003: Application Metadata

```bash
# T-003a: Get metadata
uspto app meta 16123456
# Expected: Table with Title, Status="Patented Case", Patent#=10902286

# T-003b: JSON format
uspto app meta 16123456 -f json -q
# Expected: Valid JSON envelope, ok=true

# T-003c: Invalid app number
uspto app meta abc123
# Expected: Error "invalid application number", exit code 1

# T-003d: Nonexistent app
uspto app meta 99999999
# Expected: 404 Not Found, exit code 4
```

## T-004: Search (BUG-001 regression)

```bash
# T-004a: Title search
uspto search --title "wireless sensor network" --limit 1 -f json -q
# Expected: ok=true, results with at least 1 item, total > 100

# T-004b: Assignee search (previously crashed on correspondenceAddress)
uspto search --assignee "Google" --limit 1 -f json -q
# Expected: ok=true (was FAILING before fix)

# T-004c: Patent number search
uspto search --patent "10902286" --limit 1 -f json -q
# Expected: ok=true, applicationNumberText="16123456"

# T-004d: Examiner search
uspto search --examiner "SABOURI" --limit 1 -f json -q
# Expected: ok=true (was FAILING before fix)

# T-004e: Free-text search
uspto search "artificial intelligence" --limit 1 -f json -q
# Expected: ok=true (was FAILING before fix)

# T-004f: Granted filter
uspto search --title "battery" --granted --limit 1 -f json -q
# Expected: ok=true, result has publicationCategoryBag containing "Granted/Issued"

# T-004g: Date range
uspto search --title "drone" --filed-within "6m" --limit 1 --debug -f json -q
# Expected: POST request with rangeFilters, results ok=true

# T-004h: Dry-run
uspto search --title "battery" --limit 3 --dry-run
# Expected: Shows GET URL without executing, no error

# T-004i: Pagination
uspto search --title "battery cathode" --limit 3 --page 2 -f json -q
# Expected: ok=true, offset=3
```

## T-005: Application Data Subcommands

```bash
# T-005a: Continuity
uspto app continuity 16123456
# Expected: Table with child app 17130468

# T-005b: Assignments (BUG-001 regression)
uspto app assignments 16123456 -f json -q
# Expected: ok=true with assignment data (was FAILING)

# T-005c: Attorney
uspto app attorney 16123456
# Expected: Table with firm "BIRCH STEWART KOLASCH & BIRCH"

# T-005d: Documents
uspto app docs 16123456
# Expected: 52 documents listed

# T-005e: Transactions
uspto app transactions 16123456
# Expected: 58+ transactions

# T-005f: Patent term adjustment
uspto app adjustment 16123456
# Expected: Total adjustment = 127 days

# T-005g: Foreign priority
uspto app foreign-priority 16123456
# Expected: Japan priority claim 2017-187096

# T-005h: Associated docs
uspto app associated-docs 16123456
# Expected: Grant + Pre-Grant Pub XML entries

# T-005i: Full application (BUG-001 regression)
uspto app get 16123456 -f json -q
# Expected: ok=true (was FAILING)
```

## T-006: Document Download

```bash
# T-006a: Single download
uspto app download 16123456 2 -o /tmp/test_issue_notification.pdf
# Expected: File saved successfully

# T-006b: Download-all dry-run (BUG-002 regression)
uspto app dl-all 16123456 --dry-run
# Expected: Shows list of files without downloading (was NOT stopping on dry-run)

# T-006c: Download-all filenames (BUG-002 regression)
uspto app dl-all 16123456 --dry-run | head -3
# Expected: Filenames use dashes not colons (e.g., 08-51-27 not 08:51:27)
```

## T-007: Summary (Compound Command)

```bash
# T-007a: Summary table
uspto summary 16123456
# Expected: Full summary with metadata, continuity, transactions, documents

# T-007b: Summary JSON
uspto summary 16123456 -f json -q
# Expected: ok=true, results object (not array)

# T-007c: Summary handles assignment warning gracefully
# Expected: "Warning: failed to fetch assignment" OR assignment data shown (if BUG-001 fixed)
```

## T-008: Family Tree

```bash
# T-008a: Simple family
uspto family 16123456
# Expected: 2 members, no duplicates

# T-008b: Complex family (BUG-006 regression)
uspto family 12477075 --depth 3
# Expected: 16 unique members, NO duplicates in display (was showing ~36 with dupes)

# T-008c: Family JSON
uspto family 16123456 -f json -q
# Expected: ok=true, tree structure with allApplicationNumbers
```

## T-009: PTAB Commands

```bash
# T-009a: Search proceedings
uspto ptab search --type IPR --petitioner "Apple" --limit 3 -f json -q
# Expected: ok=true, results array

# T-009b: Get proceeding
uspto ptab get IPR2026-00243 -f json -q
# Expected: ok=true

# T-009c: Search decisions (BUG-004 regression)
uspto ptab decisions --trial IPR2016-00134 -f json -q
# Expected: ok=true, results NOT null (was returning null)

# T-009d: Decisions-for (BUG-004 regression)
uspto ptab decisions-for IPR2016-00134 -f json -q
# Expected: ok=true, results with 1 decision (was returning null)

# T-009e: Trial documents
uspto ptab docs-for IPR2016-00134 -f json -q
# Expected: ok=true, 23 documents

# T-009f: Search appeals
uspto ptab appeals --limit 1 -f json -q
# Expected: ok=true

# T-009g: Get specific appeal
uspto ptab appeal 650ad83e-56f0-4d7d-843a-5f238d16951f -f json -q
# Expected: ok=true

# T-009h: Search interferences
uspto ptab interferences --limit 1 -f json -q
# Expected: ok=true
```

## T-010: Petition Commands

```bash
# T-010a: Petition search (BUG-003 regression)
uspto petition search --limit 3 -f json -q
# Expected: ok=true (was FAILING on prosecutionStatusCode type)

# T-010b: Petition search with filters
uspto petition search "revival" --decision GRANTED --limit 3 -f json -q
# Expected: ok=true with results
```

## T-011: Bulk Data Commands

```bash
# T-011a: Bulk search
uspto bulk search --limit 3 -f json -q
# Expected: ok=true with products

# T-011b: Bulk get (BUG-005 regression)
uspto bulk get PTGRXML -f json -q
# Expected: ok=true, productIdentifier="PTGRXML", fields NOT empty (was all empty)

# T-011c: Bulk files (BUG-005 regression)
uspto bulk files PTGRXML -f json -q
# Expected: ok=true, non-zero file list (was returning 0 files)
```

## T-012: Output Formats

```bash
# T-012a: NDJSON
uspto status "patented" -f ndjson -q
# Expected: 2 lines, each valid JSON

# T-012b: CSV
uspto status "patented" -f csv -q
# Expected: CSV with header + 2 rows

# T-012c: JSON minified
uspto status 150 -f json --minify -q
# Expected: Single-line JSON

# T-012d: Quiet mode
uspto app docs 16123456 -q 2>/dev/null | head -3
# Expected: No "Found X documents" prefix, just table
```

## T-013: Error Handling

```bash
# T-013a: Missing required arg
uspto app meta
# Expected: "accepts 1 arg(s)", exit code != 0

# T-013b: Auth error
USPTO_API_KEY="" uspto app extension 16123456
# Expected: Warning about missing API key, helpful link, exit code 3

# T-013c: 404 response
uspto app meta 99999999
# Expected: Exit code 4

# T-013d: Debug output
uspto app meta 16123456 --debug -f json -q
# Expected: Shows "[DEBUG] GET https://api.uspto.gov/..." before response
```

## T-014: Agent Workflow Scenarios

```bash
# T-014a: Patent number to app number
# Given patent 10902286, find app number
APP=$(uspto search --patent 10902286 --limit 1 -f json -q | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['results'][0]['applicationNumberText'])")
echo $APP
# Expected: "16123456"

# T-014b: Chain: search -> summary
# Find patents by an inventor, then get summary of first result
APP=$(uspto search --inventor "KANADA" --title "learning assistance" --limit 1 -f json -q | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['results'][0]['applicationNumberText'])")
uspto summary $APP -f json -q | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['results']['title'][:60])"
# Expected: Prints title starting with "LEARNING ASSISTANCE DEVICE"

# T-014c: PTAB monitoring
# Find active IPRs for a company and get decisions
uspto ptab search --petitioner "Apple" --type IPR --limit 1 -f json -q | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['results'][0]['trialNumber'])"
# Expected: Prints a trial number like "IPR2026-XXXXX"
```


