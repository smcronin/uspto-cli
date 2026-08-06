# Prompt 11: Keyless Trademark Discovery

## The Prompt

> I am on a clean machine and do not have either USPTO API key yet. Find up
> to five live U.S. trademark applications containing the word OPENAI. For
> each result, give me the serial number, mark wording, current owner,
> international classes, and a short goods/services excerpt. Then tell me
> how many live NIKE records are in International Class 025 without
> downloading all of them.
>
> Please keep discovery keyless. Tell me which USPTO service supplied the
> results and clearly distinguish candidate search results from an official
> hydrated TSDR case record.

## What This Tests

- `trademark search` without an ODP or TSDR credential
- Friendly wordmark, status, class, limit, field projection, and count flags
- Understanding that Trademark Search and TSDR are separate providers
- Useful summarization instead of dumping the entire search response

## Expected Behavior

1. Agent uses `uspto trademark search`, not a patent search or TSDR route
2. Search succeeds without attempting to configure or expose an API key
3. Agent projects or extracts the requested fields and limits results to five
4. Agent uses a count-only query for live NIKE marks in class 025
5. Agent identifies the provider as `tmsearch` and describes results as
   discovery candidates that should be hydrated through TSDR when authoritative
   current status matters

## Pass Criteria

- No key is requested, invented, or reused for the discovery calls
- Both OPENAI discovery and NIKE class-count tasks are completed
- Serial, wording, owner, class, and goods/services are presented when present
- The response names Trademark Search/TM Search rather than TSDR as the source
- The agent does not claim search hits are complete official case files

