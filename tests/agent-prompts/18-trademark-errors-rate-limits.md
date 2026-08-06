# Prompt 18: Validation Errors and Rate-Limit Recovery

## The Prompt

> Before I automate a large trademark review, safely prove how the CLI handles
> bad input. Without making unnecessary USPTO calls, try these cases and record
> each command's exit code and concise error:
>
> 1. A bare seven-digit trademark identifier `3500038` with automatic type
>    detection (it is ambiguous between U.S. and international registration)
> 2. For `sn:97238896`, a document date range whose start is `2025-12-31`
>    and end is `2025-01-01`
> 3. Trademark search status `pending`, which is not one of live/dead/all
> 4. A raw request using a protocol-relative URL beginning `//example.com/`
>
> Then give me a checkpoint-and-resume policy for HTTP 429, 502/503, and TSDR
> exit codes 3 through 6. Include the published peak/off-peak metadata and
> PDF/ZIP limits, explain `Retry-After`, and make clear why several parallel
> CLI processes should not fan out heavy downloads.

## What This Tests

- Identifier ambiguity and explicit `rn:`/`ir:` remediation
- Cross-field date validation and search-enum validation
- Raw request origin validation
- Exit-code-aware automation and transient-error recovery
- Understanding of shared metadata versus PDF/ZIP rate-limit lanes

## Expected Behavior

1. Agent uses `--dry-run` where helpful and chooses commands that fail local
   validation before a USPTO request is sent
2. Agent captures nonzero exit codes instead of aborting the whole exercise
3. Agent recommends explicit `rn:3500038` or `ir:3500038` based on known type
4. Agent describes exit 3 as auth/config, 4 as not found/identifier/key check,
   5 as exhausted rate/transient retries, and 6 as backend/transient service
5. Agent honors `Retry-After`, checkpoints completed work, and resumes
   serially/conservatively rather than immediately fanning out
6. Agent reports peak limits of 60 total and 4 PDF/ZIP requests per key/minute,
   and off-peak limits (10 p.m.–5 a.m. Eastern) of 120 total and 12 PDF/ZIP
   requests per key/minute

## Pass Criteria

- All four invalid inputs are rejected safely and their exit codes captured
- No request is made to `example.com` and no key is exposed
- Reversed dates and invalid status are corrected explicitly
- Published peak/off-peak total and PDF/ZIP limits are accurately stated
- Recovery guidance distinguishes retryable failures from missing credentials
  and does not recommend unbounded parallel retries
