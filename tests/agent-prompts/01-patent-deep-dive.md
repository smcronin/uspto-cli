# Prompt 1: Single Patent Deep Dive

## The Prompt

> I need to do due diligence on US patent 10,902,286. Can you pull up a
> summary of the patent — who owns it, what's the title, when was it
> filed and granted, what art unit and examiner handled it? Then I need
> to read the actual claims and see what prior art the examiner cited.
> Also check if this patent has any parent or child applications.

## What This Tests

- Patent number → application number resolution (search --patent)
- summary command (5-API compound call)
- app claims (grant XML extraction)
- app citations (grant XML extraction, examiner vs applicant categories)
- app cont (continuity/parent-child)
- Agent's ability to chain commands and synthesize results into a readable briefing

## Expected Behavior

1. Agent searches by patent number to resolve the app number
2. Runs summary to get the overview
3. Extracts claims from grant XML
4. Extracts citations and notes which were examiner-cited vs applicant-cited
5. Pulls continuity data showing parent/child relationships
6. Presents all of this in a coherent briefing, not raw JSON

## Pass Criteria

- Agent correctly resolves patent 10902286 to an application number
- Claims are returned as structured text (not "error" or empty)
- Citations include both patent and NPL references with categories
- Continuity shows at least parent application(s)
- Agent doesn't ask the user for the application number

