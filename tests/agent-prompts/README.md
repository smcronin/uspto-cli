# USPTO CLI Agent Stress Test Prompts

Numbered prompts written as an IP practitioner or automation developer would
naturally ask them. The runner discovers the corpus dynamically; the current
matrix exercises the major patent and trademark CLI surfaces. Hand each prompt to
your agent in a fresh session with the `/uspto` skill loaded.

## Coverage Matrix

| Prompt | Commands Exercised | Formats | Edge Cases |
|--------|-------------------|---------|------------|
| 1 | search (--patent), summary, app claims, app citations, app cont | json | Patent# → app# resolution, grant XML |
| 2 | search (--assignee, --granted, --filed-within, --all, --facets, --fields, --download, --sort) | csv, json | Facets, field projection, server-side export |
| 3 | patent bundle | table, json | Auto-ID resolution from pub number, bundle directory structure |
| 4 | summary, app txn, app docs (--codes), app dl | json, pdf | Document code filtering, PDF download by index |
| 5 | search (--title, --inventor), app fulltext, app abstract on pending | json | Grant XML on pending app (should fail gracefully) |
| 6 | ptab search (--type, --patent), ptab get, ptab decisions-for, ptab docs-for | json | Full PTAB workflow |
| 7 | family (--depth), summary (multiple members), app assign | json | Recursive family tree, multi-member analysis |
| 8 | search (--examiner, --art-unit, --status, --filter), petition search, status | json, csv | POST filter syntax, petition decisions, status lookup |
| 9 | app meta, app attorney, app pta, app fp, app dl-all (--codes) | json, pdf | All the "minor" app subcommands |
| 10 | search (--assignor, --reel-frame), bulk search, bulk get, bulk files, search --dry-run | json | Assignment search, bulk data catalog, dry-run |
| 11 | trademark search (--wordmark, --status, --class, --fields, --count-only) | json | Keyless discovery, provider boundary |
| 12 | config show, trademark search, missing-key guidance | json | ODP vs TSDR auth, secret-safe onboarding, exit 3 |
| 13 | trademark case status/get/parties/goods/events/assignments | json | Current owner vs history, event chronology, provenance |
| 14 | trademark docs list/info/fetch | json, native asset | View-relative index, modern CMS URL, credential containment |
| 15 | trademark search, trademark batch (--ids-file) | json, txt | Deduplication, >25-case chunking, partial failures |
| 16 | trademark api-spec, trademark request | json | Swagger inventory, output validation, origin guard |
| 17 | trademark search/case automation | json | Envelope parsing, per-record recovery, source provenance |
| 18 | trademark validation failures and recovery policy | json | Ambiguous IDs, invalid ranges, exit codes, rate limits |

## How to Score

For each prompt, note:
- Did the agent pick the right commands without being told which CLI flags to use?
- Did it handle the patent# → app# resolution correctly?
- Did it recover gracefully from expected failures (e.g., grant XML on pending app)?
- Did it present results in a useful, summarized way (not just raw JSON dumps)?
- Did it chain multiple commands logically without unnecessary calls?
- Did it route trademark discovery to keyless Trademark Search and identified
  record retrieval to keyed TSDR without mixing their credentials?
- Did automation use quiet JSON envelopes and preserve provider provenance?
- Did it validate downloads and recover conservatively from partial/transient
  failures rather than fan out retries?

## Real Patent Numbers Used

These are real patents chosen for testing richness:

- **US 10,902,286** — NEC Corp, ML/federated learning, has citations, claims, family
- **US 7,657,849** — Apple "slide to unlock" — big family tree, PTAB history
- **US 11,574,018** — Moderna mRNA patent — assignments, continuity, recent
- **US20230259568A1** — A published application (may or may not be granted yet)
- **IPR2020-00388** — A real IPR proceeding with decisions

## Real Trademark Records Used

- **SN 97054561** — Public sample used for status, parties, goods, events, and assignments
- **SN 97238896** — Modern document history containing CMS-style native document links
- **RN 3500038** — Registered mark used for maintenance and identifier-ambiguity tests

Trademark Search is a keyless discovery surface. Official identified records
come from TSDR and require the separate trademark credential; the patent ODP
key must never be substituted.

## Runner

The evaluator discovers every two-digit `NN-*.md` prompt; no corpus-size
constant needs updating when a prompt is added.

```powershell
# Validate/render the complete corpus without making agent or network calls
python tests/agent-prompts/eval_runner.py --dry-run

# Render only the trademark scenarios
python tests/agent-prompts/eval_runner.py --prompts 11,12,13,14,15,16,17,18 --dry-run
```


