import { Command } from "commander";
import { createClient } from "../api/client";
import { formatOutput } from "../utils/format";
import chalk from "chalk";
import Table from "cli-table3";

export function registerStatusCommand(program: Command) {
  program
    .command("status-codes")
    .alias("status")
    .description("Search patent application status codes")
    .argument("[query]", "Search query (code number or description text)")
    .option("-l, --limit <n>", "Max results", "25")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.searchStatusCodes(query, { limit: parseInt(opts.limit) });

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(`\n${result.count} status codes found\n`);
        if (result.statusCodeBag?.length) {
          const table = new Table({
            head: [chalk.cyan("Code"), chalk.cyan("Description")],
            colWidths: [8, 80],
            wordWrap: true,
          });
          for (const s of result.statusCodeBag) {
            table.push([s.applicationStatusCode, s.applicationStatusDescriptionText]);
          }
          console.log(table.toString());
        }
      }
    });
}
