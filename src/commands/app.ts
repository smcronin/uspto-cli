import { Command } from "commander";
import { createClient } from "../api/client";
import {
  formatPatentDetail,
  formatDocumentTable,
  formatTransactionTable,
  formatContinuityTable,
  formatAssignmentTable,
  formatOutput,
} from "../utils/format";

export function registerAppCommand(program: Command) {
  const app = program
    .command("app")
    .description("Get patent application data by application number");

  app
    .command("get")
    .description("Get full application data")
    .argument("<appNumber>", "Application number (e.g., 16123456)")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (appNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getApplication(appNumber);

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else if (result.patentFileWrapperDataBag?.length) {
        console.log(formatPatentDetail(result.patentFileWrapperDataBag[0]));
      }
    });

  app
    .command("meta")
    .description("Get application metadata")
    .argument("<appNumber>", "Application number")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (appNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getMetadata(appNumber);

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else if (result.patentFileWrapperDataBag?.length) {
        console.log(formatPatentDetail(result.patentFileWrapperDataBag[0]));
      }
    });

  app
    .command("docs")
    .description("List documents in the file wrapper")
    .argument("<appNumber>", "Application number")
    .option("--codes <codes>", "Filter by document codes (comma-separated)")
    .option("--from <date>", "Filter from date (yyyy-MM-dd)")
    .option("--to <date>", "Filter to date (yyyy-MM-dd)")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (appNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getDocuments(appNumber, {
        documentCodes: opts.codes,
        officialDateFrom: opts.from,
        officialDateTo: opts.to,
      });

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(formatDocumentTable(result.documentBag));
      }
    });

  app
    .command("transactions")
    .alias("txn")
    .description("Get transaction/prosecution history")
    .argument("<appNumber>", "Application number")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (appNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getTransactions(appNumber);

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        console.log(formatTransactionTable(result.eventDataBag || result.patentFileWrapperDataBag?.[0]?.eventDataBag));
      }
    });

  app
    .command("continuity")
    .alias("cont")
    .description("Get continuity (parent/child) data")
    .argument("<appNumber>", "Application number")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (appNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getContinuity(appNumber);

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        const data = result.patentFileWrapperDataBag?.[0] || result;
        console.log(formatContinuityTable(data.parentContinuityBag, data.childContinuityBag));
      }
    });

  app
    .command("assignments")
    .alias("assign")
    .description("Get assignment/ownership data")
    .argument("<appNumber>", "Application number")
    .option("-f, --format <fmt>", "Output format: table, json", "table")
    .action(async (appNumber, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getAssignment(appNumber);

      if (opts.format === "json") {
        console.log(formatOutput(result, "json"));
      } else {
        const data = result.patentFileWrapperDataBag?.[0] || result;
        console.log(formatAssignmentTable(data.assignmentBag));
      }
    });

  app
    .command("attorney")
    .description("Get attorney/agent data")
    .argument("<appNumber>", "Application number")
    .action(async (appNumber) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getAttorney(appNumber);
      console.log(formatOutput(result, "json"));
    });

  app
    .command("adjustment")
    .alias("pta")
    .description("Get patent term adjustment data")
    .argument("<appNumber>", "Application number")
    .action(async (appNumber) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getAdjustment(appNumber);
      console.log(formatOutput(result, "json"));
    });

  app
    .command("foreign-priority")
    .alias("fp")
    .description("Get foreign priority data")
    .argument("<appNumber>", "Application number")
    .action(async (appNumber) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getForeignPriority(appNumber);
      console.log(formatOutput(result, "json"));
    });

  app
    .command("associated-docs")
    .alias("xml")
    .description("Get associated XML document metadata")
    .argument("<appNumber>", "Application number")
    .action(async (appNumber) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getAssociatedDocuments(appNumber);
      console.log(formatOutput(result, "json"));
    });

  app
    .command("download")
    .alias("dl")
    .description("Download a document PDF from the file wrapper")
    .argument("<appNumber>", "Application number")
    .argument("[docIndex]", "Document index (from docs list), default: first", "1")
    .option("-o, --output <path>", "Output file path")
    .option("--codes <codes>", "Filter by document codes first")
    .action(async (appNumber, docIndex, opts) => {
      const client = createClient({ debug: program.opts().debug });
      const result = await client.getDocuments(appNumber, { documentCodes: opts.codes });

      const docs = result.documentBag;
      if (!docs?.length) {
        console.error("No documents found.");
        process.exit(1);
      }

      const idx = parseInt(docIndex) - 1;
      if (idx < 0 || idx >= docs.length) {
        console.error(`Invalid index. Found ${docs.length} documents.`);
        process.exit(1);
      }

      const doc = docs[idx];
      const pdfOption = doc.downloadOptionBag?.find((o) => o.mimeTypeIdentifier === "PDF");
      if (!pdfOption) {
        console.error("No PDF download available for this document.");
        process.exit(1);
      }

      const outPath = opts.output || `${appNumber}_${doc.documentCode}_${doc.officialDate?.split("T")[0] || "doc"}.pdf`;
      const { mkdirSync } = await import("fs");
      const { dirname } = await import("path");
      mkdirSync(dirname(outPath), { recursive: true });

      console.log(`Downloading: ${doc.documentCodeDescriptionText}`);
      const savedPath = await client.downloadDocument(pdfOption.downloadUrl, outPath);
      console.log(`Saved to: ${savedPath}`);
    });
}
