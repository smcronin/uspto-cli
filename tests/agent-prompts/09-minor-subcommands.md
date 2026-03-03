# Prompt 9: Attorney, PTA, Foreign Priority, and Bulk Downloads

## The Prompt

> For patent application 16/123,456 (note the formatting), I need:
>
> 1. The basic metadata — title, status, filing date
> 2. Who is the attorney or agent of record?
> 3. Has there been any patent term adjustment (PTA)? If so, how many days?
> 4. Are there any foreign priority claims?
> 5. Download all the claims-related documents (just claims, not the full
>    file wrapper) as PDFs into a folder called ./claims-review/
>
> Also, separately — I've heard the USPTO publishes weekly bulk data files
> for patent grants. Can you show me what bulk data products are available
> for "patent grant" and list the most recent files?

## What This Tests

- Application number format handling (agent must strip "16/123,456" → "16123456")
- app meta (basic metadata)
- app attorney (attorney/agent info)
- app pta (patent term adjustment)
- app fp (foreign priority)
- app dl-all --codes "CLM" (filtered bulk PDF download)
- bulk search "patent grant" (bulk data catalog)
- bulk get + bulk files (product details and file listing)

## Expected Behavior

1. Agent strips formatting from "16/123,456" to get "16123456"
2. Pulls metadata, attorney, PTA, and foreign priority in parallel or sequence
3. Downloads only claims documents (--codes "CLM") to ./claims-review/
4. Searches bulk data catalog for patent grant products
5. Lists available files for the identified product

## Pass Criteria

- Agent correctly strips the formatted app number (doesn't pass "16/123,456" literally)
- All four data types are retrieved (meta, attorney, PTA, foreign priority)
- PDF download is filtered to claims only (not full file wrapper)
- Bulk data search returns product identifiers
- Agent lists recent bulk files
- No validation errors from malformed app numbers

