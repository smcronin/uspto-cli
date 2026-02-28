#!/usr/bin/env bun
import "dotenv/config";
import { Command } from "commander";
import { registerSearchCommand } from "./src/commands/search";
import { registerAppCommand } from "./src/commands/app";
import { registerPtabCommand } from "./src/commands/ptab";
import { registerPetitionCommand } from "./src/commands/petition";
import { registerBulkCommand } from "./src/commands/bulk";
import { registerStatusCommand } from "./src/commands/status";
import { UsptoApiError } from "./src/api/client";

const program = new Command();

program
  .name("uspto")
  .description("USPTO Open Data Portal CLI - Agent-ready patent data access")
  .version("0.1.0")
  .option("--debug", "Enable debug logging")
  .option("--api-key <key>", "USPTO API key (or set USPTO_API_KEY env var)")
  .hook("preAction", (thisCommand) => {
    const opts = thisCommand.opts();
    if (opts.apiKey) {
      process.env.USPTO_API_KEY = opts.apiKey;
    }
  });

// Register all commands
registerSearchCommand(program);
registerAppCommand(program);
registerPtabCommand(program);
registerPetitionCommand(program);
registerBulkCommand(program);
registerStatusCommand(program);

// Global error handler
program.exitOverride();

async function main() {
  try {
    await program.parseAsync(process.argv);
  } catch (err: any) {
    if (err instanceof UsptoApiError) {
      console.error(`\nAPI Error (${err.statusCode}): ${err.message}`);
      if (err.statusCode === 429) {
        console.error("Rate limit exceeded. Wait a moment and retry.");
      } else if (err.statusCode === 403) {
        console.error("Check your API key. Set USPTO_API_KEY or use --api-key.");
      }
      process.exit(1);
    }
    if (err.code === "commander.helpDisplayed" || err.code === "commander.version") {
      process.exit(0);
    }
    if (err.code === "commander.missingArgument" || err.code === "commander.unknownCommand") {
      process.exit(1);
    }
    console.error(`\nError: ${err.message}`);
    process.exit(1);
  }
}

main();
