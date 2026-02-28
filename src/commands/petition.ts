import { Command } from "commander";
import { createClient } from "../api/client";
import { formatOutput } from "../utils/format";
import chalk from "chalk";
import Table from "cli-table3";

export function registerPetitionCommand(program: Command) {
  const petition = program
    .command("petition")
    .description("Petition decision data");

  petition
    .command("search")
    .description("Search petition decisions")
    .argument("[query]", "Search query")
    .option("-l, --limit <n>", "Max results", "25")
    .option("-o, --offset <n>", "Starting offset", "0")
    .option("-s, --sort <field>", "Sort field and order")
    .option("--office <name>", "Deciding office filter")
    .option("--decision <type>", "Decision type: GRANTED, DENIED")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });

      const parts: string[] = [];
      if (query) parts.push(query);
      if (opts.office) parts.push(`finalDecidingOfficeName:"${opts.office}"`);
      if (opts.decision) parts.push(`decisionTypeCodeDescriptionText:${opts.decision}`);

      const q = parts.join(" AND ") || undefined;
      const result = await client.searchPetitionDecisions(q, {
        limit: parseInt(opts.limit),
        offset: parseInt(opts.offset),
        sort: opts.sort,
      });

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(`\n${result.count} petition decisions found\n`);
        if (result.petitionDecisionDataBag?.length) {
          const table = new Table({
            head: [chalk.cyan("App #"), chalk.cyan("Patent #"), chalk.cyan("Decision"), chalk.cyan("Type"), chalk.cyan("Date"), chalk.cyan("Applicant")],
            colWidths: [14, 12, 10, 30, 13, 25],
            wordWrap: true,
          });
          for (const d of result.petitionDecisionDataBag) {
            table.push([
              d.applicationNumberText || "",
              d.patentNumber || "-",
              d.decisionTypeCode || "",
              (d.decisionPetitionTypeCodeDescriptionText || "").substring(0, 40),
              d.decisionDate || d.petitionMailDate || "",
              (d.firstApplicantName || "").substring(0, 30),
            ]);
          }
          console.log(table.toString());
        }
      }
    });

  petition
    .command("get")
    .description("Get a specific petition decision")
    .argument("<recordId>", "Petition decision record identifier (UUID)")
    .option("--include-docs", "Include associated documents")
    .action(async (recordId, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getPetitionDecision(recordId, opts.includeDocs);
      console.log(formatOutput(result, "json"));
    });
}
