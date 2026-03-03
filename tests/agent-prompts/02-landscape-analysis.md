# Prompt 2: Competitive Landscape Analysis

## The Prompt

> I want to understand what Tesla has been patenting in the battery space
> over the last 3 years. Give me a count of how many granted patents they
> have, break it down by technology area if you can, and export the full
> list to a CSV file I can open in Excel. Sort by filing date, newest first.
> For the top 3 most recent patents, pull the titles and claim 1.

## What This Tests

- search --assignee + --granted + --filed-within + --sort
- --facets for technology breakdown (CPC or application type)
- --all for auto-pagination
- --download csv for server-side export (or -f csv redirect)
- --fields for field projection (reducing response size)
- app claims on specific patents (grant XML)
- Agent's ability to aggregate and summarize data

## Expected Behavior

1. Agent searches for Tesla's granted battery patents (last 3 years)
2. Uses facets or post-processing to show technology breakdown
3. Exports full result set to CSV
4. Identifies the 3 most recent by filing date
5. Extracts claim 1 from each of those 3 patents
6. Presents a summary table + the CSV file path

## Pass Criteria

- Search returns results (Tesla has battery patents)
- CSV file is created and path is reported
- Results are sorted newest-first
- Agent extracts actual claim text, not just claim counts
- Technology breakdown is presented (by CPC section or applicant name facet)

