# Prompt 5: Granted vs Pending Comparison

## The Prompt

> I have two applications I need to look at. First, find any granted patent
> by Qualcomm with "5G" in the title that was filed in the last 2 years —
> pick one and pull the full text including claims and abstract. Second,
> find a pending (not yet granted) Qualcomm application also about 5G.
> For the pending one, try to get the claims too. Tell me if there's any
> difference in what data is available for granted vs pending apps.

## What This Tests

- search --title + --assignee + --filed-within + --granted
- search --title + --assignee + --filed-within + --pending
- app fulltext or app claims on a granted patent (`source: "grant"`)
- app claims on a pending application: answered from the pre-grant
  publication XML (`source: "pgpub"`, with `publicationNumber` and
  `publicationDate`, no `patentNumber` or `grantDate`)
- Agent reading `source` and explaining the difference
- Agent recovering if the pending application it picked is not yet
  published (the only case where the XML commands fail)

## Expected Behavior

1. Agent searches for granted Qualcomm 5G patents, picks one
2. Extracts full text / claims from the granted patent (`source: "grant"`)
3. Searches for pending Qualcomm 5G applications, picks one
4. Extracts claims from the pending application; the result carries
   `source: "pgpub"` and the publication number and date
5. Explains that these are the claims as published, which may have been
   amended since, not granted claims; that citations exist only on the
   grant (`app citations` on a pgpub source returns none, with a warning);
   and that a pending application has no patent number or grant date
6. If the chosen pending application has no publication yet (no grant and
   no pgpub XML), says so, and either picks a published one or falls back
   to the file wrapper: `app docs <app> --codes CLM`, then `app text` or
   `app download` with the claims document's `documentIdentifier`

## Pass Criteria

- Agent finds at least one granted and one pending application
- Claims are extracted from both, and the agent reports which source each
  came from (`grant` vs `pgpub`)
- Agent does not present the pending application's claims as granted
  claims, or its publication number as a patent number
- Agent explains the granted-vs-pending differences (claims as granted vs
  as published, no citations, no patent number or grant date)
- If it hits an unpublished application, it handles the failure without
  confusion and uses the file-wrapper fallback, selecting the document by
  `documentIdentifier`
