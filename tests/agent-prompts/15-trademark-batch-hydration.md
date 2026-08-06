# Prompt 15: Search-to-Batch Trademark Hydration

## The Prompt

> Build a reproducible mini-portfolio of 30 live NIKE trademark applications.
> First discover 30 serial numbers using the keyless trademark search. Save
> the serials to `./nike-portfolio/serials.txt`, deliberately include the
> first serial a second time at the end, and then retrieve official TSDR
> status for the file as serial-number cases.
>
> I want the normal duplicate-safe behavior, not duplicate results. Confirm
> that the CLI chunks the more-than-25 unique identifiers safely, and report
> any transaction failures, missed elements, or oversized records instead of
> treating a partial response as complete.

## What This Tests

- Keyless search feeding keyed identifier retrieval
- JSON extraction into a newline-delimited `--ids-file`
- `trademark batch serial` deduplication
- Automatic chunking around TSDR's 25-case request maximum
- Partial-success fields: transactions, missed elements, and oversized cases

## Expected Behavior

1. Agent searches for live NIKE applications with a stable limit/projection
2. Agent writes 30 usable eight-digit serial numbers plus one deliberate repeat
3. Agent calls `trademark batch serial --ids-file ... -f json -q` without
   `--allow-dupes`
4. Agent verifies that duplicate input was removed and multiple requests were
   used or otherwise recognizes chunked execution
5. Agent audits all server transaction/error collections before summarizing

## Pass Criteria

- The portfolio file is reproducible and contains the requested duplicate
- At least 26 unique serials reach the batch command, exercising chunking
- Default deduplication is preserved
- Partial failures are surfaced by serial number and not silently dropped
- Search results are called candidates; batch TSDR results are official records

