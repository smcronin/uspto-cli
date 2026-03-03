# Patent Grant XML Schema

The USPTO publishes full-text patent grant XML files that contain structured claims, citations, abstract, and description data. These are accessible through the ODP `associated-documents` endpoint.

## How to Access

1. Call `GET /api/v1/patent/applications/{appNumber}/associated-documents`
2. The response contains `grantDocumentMetaData.fileLocationURI` (for granted patents) and `pgpubDocumentMetaData.fileLocationURI` (for published applications)
3. The file location URI points to the bulk data split file endpoint
4. The API returns a 302 redirect to a signed S3 URL
5. Follow the redirect to download the XML file

**Important**: The download counts against a rate limit of 20 downloads per file per year per API key.

## CLI Usage

```bash
# Extract claims
uspto app claims 16123456 -f json -q

# Extract citations (patent + non-patent literature)
uspto app citations 16123456 -f json -q

# Extract abstract
uspto app abstract 16123456 -f json -q

# Get the raw XML metadata (URLs)
uspto app xml 16123456 -f json -q
```

## XML Structure

The grant XML follows DTD `us-patent-grant-v45-2014-04-03.dtd`. Key elements:

```xml
<us-patent-grant>
  <us-bibliographic-data-grant>
    <publication-reference>
      <document-id>
        <country>US</country>
        <doc-number>10902286</doc-number>
        <kind>B2</kind>
        <date>20210126</date>
      </document-id>
    </publication-reference>

    <application-reference appl-type="utility">
      <document-id>
        <country>US</country>
        <doc-number>16123456</doc-number>
        <date>20180906</date>
      </document-id>
    </application-reference>

    <us-references-cited>
      <us-citation>
        <patcit num="00001">
          <document-id>
            <country>US</country>
            <doc-number>2007/0217688</doc-number>
            <kind>A1</kind>
            <name>Sabe</name>
            <date>20070900</date>
          </document-id>
        </patcit>
        <category>cited by examiner</category>
      </us-citation>

      <us-citation>
        <nplcit num="00005">
          <othercit>Non-patent literature text here...</othercit>
        </nplcit>
        <category>cited by applicant</category>
      </us-citation>
    </us-references-cited>

    <number-of-claims>10</number-of-claims>

    <classifications-cpc>
      <!-- CPC classification entries -->
    </classifications-cpc>

    <classifications-ipcr>
      <!-- IPC classification entries -->
    </classifications-ipcr>
  </us-bibliographic-data-grant>

  <abstract>
    <p id="p-0001">Abstract text with possible inline references...</p>
  </abstract>

  <description>
    <description-of-drawings>
      <p>Figure descriptions...</p>
    </description-of-drawings>
    <p>Full specification text...</p>
  </description>

  <claims>
    <claim id="CLM-00001" num="00001">
      <claim-text>1. A learning assistance device comprising a processor
        configured to:
        <claim-text>output learning discriminators to a plurality of
          respective terminal devices;</claim-text>
        <claim-text>acquire a plurality of learned discriminators...</claim-text>
      </claim-text>
    </claim>

    <claim id="CLM-00002" num="00002">
      <claim-text>2. The learning assistance device according to
        <claim-ref idref="CLM-00001">claim 1</claim-ref>,
        wherein the processor outputs an actually operated
        discriminator...</claim-text>
    </claim>
  </claims>
</us-patent-grant>
```

## Citation Categories

The `<category>` element in citations indicates who cited the reference:

| Category | Meaning |
|----------|---------|
| `cited by examiner` | Examiner found and cited this reference |
| `cited by applicant` | Applicant disclosed this reference (e.g., in IDS) |
| `cited by third party` | Third party submission |

## Claim Structure

Claims use nested `<claim-text>` elements for sub-clauses. The `<claim-ref>` element links dependent claims to their parent. Independent claims have no `<claim-ref>`.

Key attributes:
- `id`: Unique identifier (e.g., `CLM-00001`)
- `num`: Claim number as zero-padded string (e.g., `00001`)

## Pre-Grant Publication XML

The pre-grant publication XML (`pgpubDocumentMetaData`) uses a similar but slightly different DTD (`us-patent-application-v44-2014-04-03.dtd`). The structure is nearly identical but uses `<us-patent-application>` as the root element instead of `<us-patent-grant>`.

Pre-grant publications contain:
- Claims as filed (or most recently amended)
- Abstract
- Description
- Published drawings references

They do NOT contain citations (since citations are added during examination).

## Parsing Notes for Go

- Use `encoding/xml` with struct tags matching element names
- Inner XML text (claim text with nested tags) uses `,innerxml` tag
- Strip XML tags from inner text with regex `<[^>]+>` for clean output
- HTML entities need unescaping (`html.UnescapeString`)
- The `<claim-ref>` elements contain the text "claim 1" etc. and are preserved when stripping tags
- Some claims have deeply nested `<claim-text>` elements (3-4 levels deep)

## File Size

Grant XML files are typically 50-200KB for a single patent. The split files (one patent per XML) are much smaller than the weekly bulk ZIP files.

