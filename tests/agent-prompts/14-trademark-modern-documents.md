# Prompt 14: Document Indices and Modern CMS Fetch

## The Prompt

> Review the newest documents for trademark serial 97238896. List them newest
> first, then inspect document 2 by its displayed index. Retrieve document 1
> to `./tm-doc-review/native-document.xml` using the document's native USPTO
> URL if it does not have a legacy TSDR document ID.
>
> Explain the difference between a displayed index and a document ID, verify
> the downloaded payload is nonempty and looks like the advertised file type,
> and do not put a TSDR key into a public CMS URL. Do not overwrite an existing
> review artifact without an explicit overwrite decision.

## What This Tests

- `trademark docs list` with stable sorting and 1-based indices
- `trademark docs info <identifier> <index>` resolution
- `trademark docs fetch` for modern CMS records without legacy document IDs
- Same-origin credential safety for public USPTO document assets
- Atomic download, content validation, and overwrite protection

## Expected Behavior

1. Agent lists documents with `--sort date:D` and machine-readable output
2. Agent uses numeric index `2` only in combination with the same filters/sort
3. Agent uses `docs fetch ... 1 --sort date:D` for the native modern document
4. Agent checks the resulting file's existence, size, and leading content/type
5. Agent explains that list indices are view-relative selectors, while a
   document ID is a persistent legacy TSDR identifier when one exists

## Pass Criteria

- The selected records match the displayed newest-first list
- A modern no-document-ID record does not get forced through legacy PDF routes
- The saved file is under `./tm-doc-review/` and is validated
- No secret is forwarded to or embedded in the native CMS URL
- Existing-file behavior is handled explicitly rather than silently replaced

