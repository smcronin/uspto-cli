# UAT Run 2 Remediation Notes

Date: 2026-03-01

This file tracks what was fixed from `docs/claude/uat-run2-agent-self-play.md`.

## Fixed

- `search --facets` decode failure:
  - Verified fixed on latest codebase.
  - `go run . search --filter "applicationTypeLabelName=Utility" --facets "applicationTypeCategory" --limit 1 -f json --minify --quiet` now succeeds and returns parsed facets.

- PTAB download endpoint wiring:
  - Verified latest commit correctly wires `Download*Search` endpoints (`proceedings`, `decisions`, `documents`, `appeals`, `interferences`) via `--download`.

- Dry-run consistency:
  - Added dry-run handling to previously executing commands:
    - `app get/meta/docs/transactions/continuity/assignments/attorney/adjustment/foreign-priority/associated-docs`
    - `ptab` search/get/docs/doc/decisions/decision/for-variants/appeals/interferences
    - XML extraction commands (`app abstract/citations/claims/description/fulltext`)
    - `summary`, `family`
  - Dry-run now prints request plans and avoids API execution.

- Petition `--decision` filter:
  - Fixed mapping to `decisionTypeCodeDescriptionText` with enum validation (`GRANTED|DENIED|DISMISSED`).
  - `go run . petition search --decision DENIED --limit 1 -f json --minify --quiet` now succeeds.

- Petition `--include-documents` parsing:
  - Added `documentBag` parsing to petition result type (`PetitionDocument`).
  - Verified `petition get ... --include-documents` returns document metadata.

- Input validation upgrades:
  - Added strict date validation for `YYYY-MM-DD` and range order checks.
  - Added search conflict check for `--granted` + `--pending`.
  - Added sort expression validation (`field[:asc|desc]`).
  - Added `--timeout` validation (`> 0` required).

- Quiet-mode download polish:
  - `bulk download -q` now suppresses table output in table format.

## Added Tests

- New integration suite: `tests/integration/t015_quality_test.go`
  - Dry-run no-execution assertions
  - Search conflict validation
  - Date validation
  - Timeout validation
  - Petition decision filter correctness

## Validation Run

- `go test ./cmd ./tests/integration` passes.

## Workspace Note

- There are additional unrelated/untracked files in this workspace (including new test files outside this remediation scope). Those were not modified by this remediation pass.


