import { Command } from "commander";
import { createClient } from "../api/client";
import { formatPatentTable, formatPatentDetail, formatOutput } from "../utils/format";

export function registerSearchCommand(program: Command) {
  program
    .command("search")
    .description("Search patent applications")
    .argument("[query]", "Search query (e.g., inventionTitle:wireless)")
    .option("-l, --limit <n>", "Max results", "25")
    .option("-o, --offset <n>", "Starting offset", "0")
    .option("-s, --sort <field>", "Sort field and order (e.g., filingDate desc)")
    .option("--fields <fields>", "Comma-separated fields to return")
    .option("--filters <filters>", "Field-value filters")
    .option("--facets <facets>", "Comma-separated facet fields")
    .option("-f, --format <fmt>", "Output format: table, json, compact", "table")
    .option("--title <title>", "Search by invention title (shorthand)")
    .option("--applicant <name>", "Search by applicant name (shorthand)")
    .option("--inventor <name>", "Search by inventor name (shorthand)")
    .option("--patent <number>", "Search by patent number (shorthand)")
    .option("--cpc <class>", "Search by CPC classification (shorthand)")
    .option("--status <code>", "Filter by status code (shorthand)")
    .option("--filed-after <date>", "Filed after date (yyyy-MM-dd)")
    .option("--filed-before <date>", "Filed before date (yyyy-MM-dd)")
    .option("--type <code>", "Application type: UTL, PLT, DSN, REI")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      // Build query from shorthands
      const parts: string[] = [];
      if (query) parts.push(query);
      if (opts.title) parts.push(`applicationMetaData.inventionTitle:${opts.title.includes(" ") ? `"${opts.title}"` : opts.title}`);
      if (opts.applicant) parts.push(`applicationMetaData.firstApplicantName:${opts.applicant.includes(" ") ? `"${opts.applicant}"` : opts.applicant}`);
      if (opts.inventor) parts.push(`applicationMetaData.firstInventorName:${opts.inventor.includes(" ") ? `"${opts.inventor}"` : opts.inventor}`);
      if (opts.patent) parts.push(`applicationMetaData.patentNumber:${opts.patent}`);
      if (opts.cpc) parts.push(`applicationMetaData.cpcClassificationBag:${opts.cpc}`);
      if (opts.status) parts.push(`applicationMetaData.applicationStatusCode:${opts.status}`);
      if (opts.type) parts.push(`applicationMetaData.applicationTypeCode:${opts.type}`);

      // Date range
      if (opts.filedAfter || opts.filedBefore) {
        const from = opts.filedAfter || "2001-01-01";
        const to = opts.filedBefore || "2099-12-31";
        parts.push(`applicationMetaData.filingDate:[${from} TO ${to}]`);
      }

      const q = parts.join(" AND ") || undefined;

      const result = await client.searchPatents(q, {
        limit: parseInt(opts.limit),
        offset: parseInt(opts.offset),
        sort: opts.sort,
        fields: opts.fields,
        filters: opts.filters,
        facets: opts.facets,
      });

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(`\n${result.count} results found\n`);
        console.log(formatPatentTable(result.patentFileWrapperDataBag));
      }
    });
}
