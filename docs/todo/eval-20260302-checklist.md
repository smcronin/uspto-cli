# Eval 2026-03-02 Action Checklist

Source: `docs/todo/eval-20260302.md`

Legend:
- `[ ]` open
- `[x]` completed

## Bugs Found
- [x] Fix `search --type` help text for Design code (`DES`, not `DSN`; list `UTL, DES, PLT, PP, RE`) [High]
- [x] Fix `search --download csv` so active range/filter parameters are passed through [High]
- [x] Investigate/fix silent 404 on `--filed-after` + `--granted` combos [Medium]
- [x] Investigate/fix `--filed-within` combo failures; add fallback or clear guidance [Medium]
- [x] Fix or clarify `petition search --sort` behavior without text query [Low]
- [x] Document or broaden `ptab decisions-for` to include institution decisions [Low]
- [x] Improve legacy grant XML citation parsing for older patents [Low]

## CLI Improvements

### Search Enhancements
- [x] Add `search --count-only` to return totals without full result payload
- [x] Add `search --cpc-group H01M` shorthand
- [x] Improve conflicting-flag error hints (instead of generic 404)
- [x] Document GET vs POST trigger conditions and limitations

### App Subcommand Enhancements
- [x] Include foreign priority data in `summary`
- [x] Add top-level citation summary counts in `app citations` (examiner/applicant)
- [x] Allow `app dl` by `documentIdentifier` (not just 1-based index)
- [x] Add `app docs --sort date:asc`
- [x] Add `app attorney --primary`
- [x] On pending-app `app claims/fulltext` failure, suggest `app docs --codes CLM`
- [x] Add alias `--publication-number` for `--pub-number`
- [x] In `download-all`, report unique files vs filename collisions

### Family Enhancements
- [x] Include filing dates in family tree output
- [x] Distinguish CON/DIV/CIP in `allApplicationNumbers`
- [x] Add `family --with-dates` or `family --verbose` (filing/grant/applicant)

### PTAB Enhancements
- [x] Add `ptab search --family`
- [x] Add `ptab search --app`
- [x] Return empty array (not 404) for valid no-result PTAB queries
- [x] Document `decisions-for` institution-decision behavior

### Petition Enhancements
- [x] Warn on `--decision GRANTED` 404 with dataset limitation hint
- [x] Add facets query for available petition decision types

### Bulk Data Enhancements
- [x] Add `bulk files --limit`
- [x] Add `bulk get --latest --type Data` filter

### Export / Output
- [x] Add `search --all -f csv` style client-side concat helper flow for CSV export UX
- [x] Fix `search --download csv` to pass POST filters

### New Commands (Stretch)
- [x] Add `prosecution-timeline` command
- [x] Document `--codes` aliases (`rejection`, `allowance`, etc.)
- [x] Add pgpub XML parsing for pending apps

## Skill Improvements
- [x] Document `--granted-after` reliability vs `--filed-after + --granted`
- [x] Document `--download csv` POST-filter behavior
- [x] Document petition dataset `DENIED`-only reality
- [x] Update `--type` code reference to `DES` (not `DSN`)
- [x] Document `ptab decisions-for` institution-decision behavior
- [x] Document pre-2010 citation gaps possibility
- [x] Document `app assign` null behavior for direct-company filings

## Eval Runner Follow-up
- [x] Split P10 into 2 prompts or run with `--timeout 900` for complete coverage
