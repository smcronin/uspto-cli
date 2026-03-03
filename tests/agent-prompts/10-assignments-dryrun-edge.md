# Prompt 10: Assignment Searches, Dry-Run, and Edge Cases

## The Prompt

> I need to trace some IP transfers:
>
> 1. Search for any patents where Google was the assignor (meaning Google
>    transferred the patent to someone else). Show me the first 10 results.
>
> 2. I have a reel/frame number from an assignment recordation: 060620/769.
>    Can you find the patent(s) associated with that assignment?
>
> 3. Before running a big search, I want to preview what API call would be
>    made. Show me a dry-run of: all patents assigned to "Boston Dynamics"
>    that were granted in the last 2 years, sorted by grant date descending.
>    Don't actually run it — just show me the request.
>
> 4. Finally, try to look up application number 09123456 — this is an old
>    pre-2001 application. What happens? I want to see how the tool handles
>    data that might not be in the system.

## What This Tests

- search --assignor (assignment transfer search)
- search --reel-frame (reel/frame lookup)
- search --assignee + --granted-after + --sort + --dry-run (dry-run mode)
- app meta or summary on a pre-2001 app number (expected 404 / not-found)
- Agent handling and explaining the 404 gracefully
- Agent understanding the difference between --assignee and --assignor

## Expected Behavior

1. Agent searches for Google as assignor, shows first 10 results
2. Searches by reel/frame number, finds associated patent(s)
3. Constructs the Boston Dynamics search with --dry-run, shows the API URL
   WITHOUT executing it
4. Attempts to look up the old app number, gets 404 or not-found
5. Explains that the ODP API only covers applications from 2001 onward

## Pass Criteria

- Assignor search returns results (Google has transferred patents)
- Reel/frame search returns at least one result
- Dry-run shows the API URL without making the actual request
- Pre-2001 lookup fails gracefully (exit code 4 or empty result)
- Agent explains the 2001 data coverage limitation
- Agent correctly distinguishes assignee (current owner) from assignor (previous owner who transferred)

