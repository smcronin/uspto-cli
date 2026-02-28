import { Command } from "commander";
import { createClient } from "../api/client";
import { formatBulkDataTable, formatOutput } from "../utils/format";

export function registerBulkCommand(program: Command) {
  const bulk = program
    .command("bulk")
    .description("Bulk dataset search and download");

  bulk
    .command("search")
    .description("Search bulk data products")
    .argument("[query]", "Search query")
    .option("-l, --limit <n>", "Max results", "25")
    .option("-o, --offset <n>", "Starting offset", "0")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (query, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.searchBulkData(query, {
        limit: parseInt(opts.limit),
        offset: parseInt(opts.offset),
      });

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(`\n${result.count} bulk data products found\n`);
        console.log(formatBulkDataTable(result.bulkDataProductBag));
      }
    });

  bulk
    .command("get")
    .description("Get bulk data product details")
    .argument("<productId>", "Product identifier")
    .option("--include-files", "Include file listings")
    .option("--latest", "Show only latest files")
    .option("-f, --format <fmt>", "Output format", "json")
    .action(async (productId, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getBulkDataProduct(productId, {
        includeFiles: opts.includeFiles,
        latest: opts.latest,
      });
      console.log(formatOutput(result, opts.format));
    });
}
