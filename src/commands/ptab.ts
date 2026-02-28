import { Command } from "commander";
import { createClient } from "../api/client";
import { formatProceedingTable, formatOutput } from "../utils/format";

export function registerPtabCommand(program: Command) {
  const ptab = program
    .command("ptab")
    .description("PTAB proceedings, decisions, documents, appeals, interferences");

  // ─── Proceedings ──────────────────────────────────────────────

  ptab
    .command("search")
    .description("Search PTAB trial proceedings")
    .argument("[query]", "Search query (e.g., trialMetaData.trialTypeCode:IPR)")
    .option("-l, --limit <n>", "Max results", "25")
    .option("-o, --offset <n>", "Starting offset", "0")
    .option("-s, --sort <field>", "Sort field and order")
    .option("--type <code>", "Trial type: IPR, PGR, CBM")
    .option("--patent <number>", "Patent number")
    .option("--owner <name>", "Patent owner name")
    .option("--petitioner <name>", "Petitioner name")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      const parts: string[] = [];
      if (query) parts.push(query);
      if (opts.type) parts.push(`trialMetaData.trialTypeCode:${opts.type}`);
      if (opts.patent) parts.push(`patentOwnerData.patentNumber:${opts.patent}`);
      if (opts.owner) parts.push(`patentOwnerData.patentOwnerName:${opts.owner.includes(" ") ? `"${opts.owner}"` : opts.owner}`);
      if (opts.petitioner) parts.push(`regularPetitionerData.realPartyInInterestName:${opts.petitioner.includes(" ") ? `"${opts.petitioner}"` : opts.petitioner}`);

      const q = parts.join(" AND ") || undefined;
      const result = await client.searchProceedings(q, {
        limit: parseInt(opts.limit),
        offset: parseInt(opts.offset),
        sort: opts.sort,
      });

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(`\n${result.count} proceedings found\n`);
        console.log(formatProceedingTable(result.patentTrialProceedingDataBag));
      }
    });

  ptab
    .command("get")
    .description("Get a specific trial proceeding")
    .argument("<trialNumber>", "Trial number (e.g., IPR2023-00001)")
    .option("-f, --format <fmt>", "Output format", "json")
    .action(async (trialNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getProceeding(trialNumber);
      console.log(formatOutput(result, opts.format));
    });

  // ─── Decisions ────────────────────────────────────────────────

  ptab
    .command("decisions")
    .description("Search or get trial decisions")
    .argument("[query]", "Search query or trial number")
    .option("-l, --limit <n>", "Max results", "25")
    .option("--trial <number>", "Get decisions for a specific trial")
    .option("-f, --format <fmt>", "Output format", "json")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      if (opts.trial) {
        const result = await client.getTrialDecisions(opts.trial);
        console.log(formatOutput(result, opts.format));
      } else {
        const result = await client.searchTrialDecisions(query, { limit: parseInt(opts.limit) });
        console.log(formatOutput(result, opts.format));
      }
    });

  // ─── Documents ────────────────────────────────────────────────

  ptab
    .command("docs")
    .description("Search or get trial documents")
    .argument("[query]", "Search query")
    .option("-l, --limit <n>", "Max results", "25")
    .option("--trial <number>", "Get documents for a specific trial")
    .option("-f, --format <fmt>", "Output format", "json")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      if (opts.trial) {
        const result = await client.getTrialDocuments(opts.trial);
        console.log(formatOutput(result, opts.format));
      } else {
        const result = await client.searchTrialDocuments(query, { limit: parseInt(opts.limit) });
        console.log(formatOutput(result, opts.format));
      }
    });

  // ─── Appeals ──────────────────────────────────────────────────

  ptab
    .command("appeals")
    .description("Search or get appeal decisions")
    .argument("[query]", "Search query")
    .option("-l, --limit <n>", "Max results", "25")
    .option("--appeal <number>", "Get decisions for a specific appeal")
    .option("-f, --format <fmt>", "Output format", "json")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      if (opts.appeal) {
        const result = await client.getAppealDecisions(opts.appeal);
        console.log(formatOutput(result, opts.format));
      } else {
        const result = await client.searchAppealDecisions(query, { limit: parseInt(opts.limit) });
        console.log(formatOutput(result, opts.format));
      }
    });

  // ─── Interferences ────────────────────────────────────────────

  ptab
    .command("interferences")
    .alias("intf")
    .description("Search or get interference decisions")
    .argument("[query]", "Search query")
    .option("-l, --limit <n>", "Max results", "25")
    .option("--interference <number>", "Get decisions for a specific interference")
    .option("-f, --format <fmt>", "Output format", "json")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      if (opts.interference) {
        const result = await client.getInterferenceDecisions(opts.interference);
        console.log(formatOutput(result, opts.format));
      } else {
        const result = await client.searchInterferenceDecisions(query, { limit: parseInt(opts.limit) });
        console.log(formatOutput(result, opts.format));
      }
    });
}
