# Prompt 3: Full Patent Bundle Download

## The Prompt

> Download everything you can get on publication US20050021049A1. I want
> the full text, claims, citations, the PDFs from the file wrapper, and
> the raw XML. Put it all in a folder called ./patent-review/US20050021049A1.
> After the download, tell me what you got — how many PDFs downloaded,
> whether the grant XML was available, and give me a quick summary of
> what the patent is about.

## What This Tests

- patent bundle with publication number input
- --out flag for custom output directory
- Auto-ID resolution (publication → app number)
- Agent reading the bundle README and artifacts after download
- Agent summarizing the fulltext.json content

## Expected Behavior

1. Agent runs `patent bundle US20050021049A1 --out ./patent-review/US20050021049A1`
2. Reports the resolved application number
3. Lists what artifacts were created (resolution, fulltext, docs, PDFs, XML)
4. Reports PDF download stats (downloaded/skipped/failed)
5. Reads the fulltext JSON or README and summarizes the invention
6. Notes any warnings (e.g., pgpub XML unavailable)

## Pass Criteria

- Bundle directory is created at the specified path
- Agent reports the app number that was resolved
- PDF count is reported
- Agent provides a substantive summary of the invention (not just "download complete")
- Grant/pgpub XML availability is noted

