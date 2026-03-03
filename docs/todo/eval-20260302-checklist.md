# Eval 2026-03-02 Action Checklist

Source: `docs/todo/eval-20260302.md`

Legend:
- `[ ]` open
- `[x]` completed

## Bugs Found
- [ ] Fix `search --type` help text for Design code (`DES`, not `DSN`; list `UTL, DES, PLT, PP, RE`) [High]
- [ ] Fix `search --download csv` so active range/filter parameters are passed through [High]
- [ ] Investigate/fix silent 404 on `--filed-after` + `--granted` combos [Medium]
- [ ] Investigate/fix `--filed-within` combo failures; add fallback or clear guidance [Medium]
- [ ] Fix or clarify `petition search --sort` behavior without text query [Low]
- [ ] Document or broaden `ptab decisions-for` to include institution decisions [Low]
- [ ] Improve legacy grant XML citation parsing for older patents [Low]

## CLI Improvements

### Search Enhancements
- [x] Add `search --count-only` to return totals without full result payload
- [ ] Add `search --cpc-group H01M` shorthand
- [ ] Improve conflicting-flag error hints (instead of generic 404)
- [ ] Document GET vs POST trigger conditions and limitations

### App Subcommand Enhancements
- [ ] Include foreign priority data in `summary`
- [ ] Add top-level citation summary counts in `app citations` (examiner/applicant)
- [ ] Allow `app dl` by `documentIdentifier` (not just 1-based index)
- [ ] Add `app docs --sort date:asc`
- [ ] Add `app attorney --primary`
- [ ] On pending-app `app claims/fulltext` failure, suggest `app docs --codes CLM`
- [ ] Add alias `--publication-number` for `--pub-number`
- [ ] In `download-all`, report unique files vs filename collisions

### Family Enhancements
- [ ] Include filing dates in family tree output
- [ ] Distinguish CON/DIV/CIP in `allApplicationNumbers`
- [ ] Add `family --with-dates` or `family --verbose` (filing/grant/applicant)

### PTAB Enhancements
- [ ] Add `ptab search --family`
- [ ] Add `ptab search --app`
- [ ] Return empty array (not 404) for valid no-result PTAB queries
- [ ] Document `decisions-for` FWD-only limitation

### Petition Enhancements
- [ ] Warn on `--decision GRANTED` 404 with dataset limitation hint
- [ ] Add facets query for available petition decision types

### Bulk Data Enhancements
- [ ] Add `bulk files --limit`
- [ ] Add `bulk get --latest --type Data` filter

### Export / Output
- [ ] Add `search --all -f csv` style client-side concat helper flow for CSV export UX
- [ ] Fix `search --download csv` to pass POST filters

### New Commands (Stretch)
- [ ] Add `prosecution-timeline` command
- [ ] Document `--codes` aliases (`rejection`, `allowance`, etc.)
- [ ] Add pgpub XML parsing for pending apps

## Skill Improvements
- [ ] Document `--granted-after` reliability vs `--filed-after + --granted`
- [ ] Document `--download csv` filter limitation/workaround
- [ ] Document petition dataset `DENIED`-only reality
- [ ] Update `--type` code reference to `DES` (not `DSN`)
- [ ] Document `ptab decisions-for` FWD-only behavior
- [ ] Document pre-2010 citation gaps possibility
- [ ] Document `app assign` null behavior for direct-company filings

## Eval Runner Follow-up
- [ ] Split P10 into 2 prompts or run with `--timeout 900` for complete coverage
