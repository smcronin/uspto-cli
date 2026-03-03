# Prompt 4: Prosecution History Review

## The Prompt

> I'm preparing for a reexamination and need to understand the prosecution
> history of patent 11,574,018. Walk me through the timeline — when was
> it filed, what office actions were issued, and when was it allowed?
> I specifically need to see any non-final rejections, final rejections,
> and the notice of allowance. If you can find those documents, download
> the first office action PDF for me.

## What This Tests

- Patent number → app number resolution
- summary command for the overview timeline
- app txn for full prosecution transaction history
- app docs --codes "CTNF,CTFR,NOA" for filtered document list
- app dl for downloading a specific PDF by index
- Agent's ability to read transaction codes and construct a prosecution narrative

## Expected Behavior

1. Agent resolves patent 11574018 to app number
2. Runs summary for the timeline overview
3. Pulls full transaction history and identifies key prosecution events
4. Filters documents to office actions and NOA
5. Downloads the first office action PDF
6. Presents a chronological prosecution narrative

## Pass Criteria

- Agent finds the application and its filing/grant dates
- Transaction history is presented chronologically
- Office action documents are identified by code (CTNF/CTFR/NOA)
- At least one PDF is downloaded to disk
- Agent provides a narrative, not just a raw event list

