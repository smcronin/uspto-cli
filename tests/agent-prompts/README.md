# USPTO CLI Agent Stress Test Prompts

10 prompts written as a patent practitioner would naturally ask them.
Together they exercise every major CLI feature. Hand each prompt to
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

## How to Score

For each prompt, note:
- Did the agent pick the right commands without being told which CLI flags to use?
- Did it handle the patent# → app# resolution correctly?
- Did it recover gracefully from expected failures (e.g., grant XML on pending app)?
- Did it present results in a useful, summarized way (not just raw JSON dumps)?
- Did it chain multiple commands logically without unnecessary calls?

## Real Patent Numbers Used

These are real patents chosen for testing richness:

- **US 10,902,286** — NEC Corp, ML/federated learning, has citations, claims, family
- **US 7,657,849** — Apple "slide to unlock" — big family tree, PTAB history
- **US 11,574,018** — Moderna mRNA patent — assignments, continuity, recent
- **US20230259568A1** — A published application (may or may not be granted yet)
- **IPR2020-00388** — A real IPR proceeding with decisions


