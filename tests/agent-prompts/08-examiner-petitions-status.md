# Prompt 8: Examiner Analytics, Petitions, and Status Codes

## The Prompt

> I need a few things:
>
> 1. Find all patents granted in art unit 2617 since January 2025. How many
>    are there? Export the list to CSV.
>
> 2. I've heard that status code 150 means "Patented Case" — can you confirm
>    that? Also look up what status codes relate to "abandoned" applications.
>
> 3. Search for any petition decisions in the Office of Petitions that were
>    GRANTED in the last 30 days. How many revival petitions were granted?
>
> 4. I want to search for Design patents filed in the last year. I know the
>    regular --type flag can be tricky, so use whatever method works best.

## What This Tests

- search --art-unit + --granted-after + --all + -f csv (examiner analytics)
- status 150 (code lookup)
- status "abandoned" (text search)
- petition search --office + --decision (petition decisions)
- search --filter "applicationTypeLabelName=Design" (POST filter for design patents — the known gotcha)
- Agent knowing to use --filter instead of --type for design patents

## Expected Behavior

1. Agent searches art unit 2617, exports to CSV, reports count
2. Confirms status 150 = "Patented Case"
3. Finds abandoned-related status codes (161, 162, etc.)
4. Searches petition decisions, filters to GRANTED, reports count
5. Uses the POST filter syntax for design patents (not --type DSN)

## Pass Criteria

- Art unit search returns results and CSV is created
- Status code 150 is confirmed correctly
- Multiple abandoned status codes are found
- Petition search returns results (or reports count = 0 clearly)
- Agent uses --filter for design patents (not --type which is unreliable)
- Agent explains WHY it used --filter instead of --type (bonus points)

