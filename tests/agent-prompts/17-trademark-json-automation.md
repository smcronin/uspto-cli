# Prompt 17: Machine-Readable Trademark Automation

## The Prompt

> Create `./trademark-report/report.json` for the first three live OPENAI
> search results. The file should contain a generated timestamp and, for each
> candidate, the serial number, wordmark, search owner, official TSDR status,
> official current owner, latest event date/code, and any retrieval error.
>
> Use only quiet JSON CLI output as the input to your script or shell pipeline.
> Preserve enough provenance to show which records came from `tmsearch` and
> which came from `tsdr`. A failure for one serial must be recorded in that
> row and must not discard the other rows. Validate the final file as JSON.

## What This Tests

- Stable `-f json -q` envelopes for search and case commands
- Search-result extraction followed by per-identifier hydration
- `provider` and `commandPath` provenance fields
- Checkpointable, per-record error handling
- Creation and validation of a useful downstream JSON artifact

## Expected Behavior

1. Agent searches for three live OPENAI candidates in JSON mode
2. Agent extracts serials from `results` rather than scraping table output
3. Agent queries official status/parties/events for each `sn:` identifier
4. Agent continues after an individual not-found or transient error and stores
   the error alongside that candidate
5. Agent writes one valid JSON document with explicit TM Search and TSDR
   provenance and verifies it with a local parser

## Pass Criteria

- Every CLI data command uses `-f json -q`
- The report contains no terminal decoration or embedded raw table text
- Up to three records are hydrated without losing successful peers on failure
- Search owner and official current owner remain distinct fields
- `report.json` parses successfully and includes source/provider information

