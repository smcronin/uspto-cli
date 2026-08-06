# Prompt 13: Trademark Status, Owners, Events, and Assignments

## The Prompt

> Do a compact due-diligence pass on U.S. trademark serial 97054561. I need
> the current status and status date, the present owner and correspondence
> parties, the goods/services and classes, the ten most recent prosecution
> events, and any recorded assignments. If the focused views disagree with
> the full case record, call that out rather than silently choosing one.
>
> Use machine-readable output for retrieval, but give me a concise human
> synthesis with the identifiers and source/provider preserved.

## What This Tests

- `trademark case status` and schema-tolerant parsed `trademark case get`
- Focused `case parties`, `case goods`, `case events`, and `case assignments`
- Event limiting and chronology
- Current-owner interpretation versus historical/assignment parties
- JSON-envelope provenance (`commandPath`, `provider`, identifier)

## Expected Behavior

1. Agent hydrates the known serial through TSDR with explicit `sn:` notation
2. Agent obtains the status plus focused owner, goods, event, and assignment
   views, using `--latest 10` for events
3. Agent distinguishes current owner/correspondence data from historical
   assignors and assignees
4. Agent summarizes material facts and notes missing or inconsistent fields
5. Retrieval commands use `-f json -q` so downstream parsing is deterministic

## Pass Criteria

- All five requested data areas are addressed
- No bare ambiguous identifier or patent command is used
- The newest events are presented in a defensible order
- Assignment history is not mislabeled as the current owner
- The answer retains the serial number and identifies TSDR as the provider
