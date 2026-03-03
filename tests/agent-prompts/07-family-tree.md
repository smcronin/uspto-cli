# Prompt 7: Patent Family Mapping

## The Prompt

> I need to map out the full patent family for Apple's slide-to-unlock
> patent (US 7,657,849). Trace all the continuations, divisionals, and
> CIPs — go at least 3 levels deep. How many total family members are
> there? For each member, I need: application number, title, status
> (granted/pending/abandoned), and patent number if granted. Then pick
> the 2 most recently filed family members and get their assignment
> history — I want to confirm Apple still owns them.

## What This Tests

- Patent number → app number resolution
- family --depth 3 (recursive family tree)
- Processing allApplicationNumbers from the family response
- summary on multiple family members (batch lookups)
- app assign for ownership verification
- Agent's ability to build a structured family table

## Expected Behavior

1. Agent resolves patent 7657849 to app number
2. Runs family command with depth 3
3. Counts total unique family members
4. Extracts key metadata for each member (status, patent#, title)
5. Identifies the 2 most recently filed members
6. Runs assignment lookups on those 2 members
7. Confirms current assignee(s)

## Pass Criteria

- Family tree returns multiple members (this patent has a large family)
- Agent reports an accurate count of unique family members
- Status of each member is identified (not all will be granted)
- Assignment data shows the current owner
- Agent presents results as a structured table or list
- No duplicate family members in the output

