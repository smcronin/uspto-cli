# Prompt 6: PTAB Challenge Investigation

## The Prompt

> Has Apple's slide-to-unlock patent (US 7,657,849) ever been challenged
> at the PTAB? If so, find the IPR proceeding number, who the petitioner
> was, what the current status is, and whether there's a final written
> decision. If there is a decision, get the decision document details.
> Also pull up any other documents filed in the proceeding.

## What This Tests

- ptab search --patent (finding IPR proceedings for a specific patent)
- ptab get (proceeding details — parties, status, dates)
- ptab decisions-for (all decisions for a trial)
- ptab docs-for (all filed documents for a trial)
- Agent navigating the PTAB data model (proceedings → decisions → documents)
- Handling the case where there may be multiple proceedings

## Expected Behavior

1. Agent searches PTAB for patent 7657849
2. If IPR(s) found, gets proceeding details (petitioner, patent owner, status)
3. Fetches decisions for the proceeding
4. Fetches the document list
5. Summarizes: who challenged, on what grounds, what happened
6. If no PTAB proceedings found, reports that clearly

## Pass Criteria

- Agent correctly searches by patent number in PTAB
- If proceedings exist, parties and status are identified
- Decisions are retrieved (or "no decisions" is reported)
- Document list is retrieved
- Agent provides a litigation-style summary, not just data dumps
- If no proceedings found, agent says so clearly rather than erroring

