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
- app fulltext or app claims on a granted patent (should succeed)
- app claims on a pending application (should fail gracefully — no grant XML)
- Agent explaining the data availability difference
- Agent recovering from the expected grant XML failure on pending app

## Expected Behavior

1. Agent searches for granted Qualcomm 5G patents, picks one
2. Extracts full text / claims from the granted patent (succeeds)
3. Searches for pending Qualcomm 5G applications, picks one
4. Attempts to extract claims from the pending app (fails)
5. Explains that grant XML commands only work on granted patents
6. Suggests using `app docs` + PDF download as the fallback for pending apps

## Pass Criteria

- Agent finds at least one granted and one pending application
- Claims are successfully extracted from the granted patent
- Agent handles the pending-app failure gracefully (no crash, no confusion)
- Agent explains WHY claims aren't available for pending apps
- Agent suggests the PDF document fallback

