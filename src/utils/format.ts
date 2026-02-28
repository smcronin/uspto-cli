import Table from "cli-table3";
import chalk from "chalk";
import type { PatentFileWrapper, Document, ProceedingData, PetitionDecision, EventData, ContinuityData, Assignment, BulkDataProduct } from "../types/api";

export type OutputFormat = "json" | "table" | "compact";

export function formatOutput(data: any, format: OutputFormat): string {
  if (format === "json") {
    return JSON.stringify(data, null, 2);
  }
  // Default to JSON for non-table data
  return JSON.stringify(data, null, 2);
}

export function formatPatentTable(patents: PatentFileWrapper[]): string {
  if (!patents?.length) return chalk.yellow("No patents found.");

  const table = new Table({
    head: [
      chalk.cyan("App #"),
      chalk.cyan("Patent #"),
      chalk.cyan("Title"),
      chalk.cyan("Filing Date"),
      chalk.cyan("Status"),
      chalk.cyan("Applicant"),
    ],
    colWidths: [14, 12, 40, 13, 22, 25],
    wordWrap: true,
  });

  for (const p of patents) {
    const m = p.applicationMetaData;
    table.push([
      p.applicationNumberText || "",
      m?.patentNumber || "-",
      (m?.inventionTitle || "").substring(0, 80),
      m?.filingDate || "",
      m?.applicationStatusDescriptionText || "",
      (m?.firstApplicantName || "").substring(0, 30),
    ]);
  }

  return table.toString();
}

export function formatPatentDetail(p: PatentFileWrapper): string {
  const m = p.applicationMetaData;
  const lines = [
    "",
    chalk.bold.white(`  ${m?.inventionTitle || "Unknown"}`),
    "",
    `  ${chalk.gray("Application #:")}  ${p.applicationNumberText}`,
    `  ${chalk.gray("Patent #:")}      ${m?.patentNumber || "-"}`,
    `  ${chalk.gray("Type:")}          ${m?.applicationTypeLabelName || m?.applicationTypeCode || "-"}`,
    `  ${chalk.gray("Status:")}        ${m?.applicationStatusDescriptionText || "-"} (${m?.applicationStatusCode || "-"})`,
    `  ${chalk.gray("Filing Date:")}   ${m?.filingDate || "-"}`,
    `  ${chalk.gray("Grant Date:")}    ${m?.grantDate || "-"}`,
    `  ${chalk.gray("Applicant:")}     ${m?.firstApplicantName || "-"}`,
    `  ${chalk.gray("Inventor:")}      ${m?.firstInventorName || "-"}`,
    `  ${chalk.gray("Examiner:")}      ${m?.examinerNameText || "-"}`,
    `  ${chalk.gray("Art Unit:")}      ${m?.groupArtUnitNumber || "-"}`,
    `  ${chalk.gray("Docket:")}        ${m?.docketNumber || "-"}`,
    `  ${chalk.gray("Customer #:")}    ${m?.customerNumber || "-"}`,
    `  ${chalk.gray("Entity:")}        ${m?.entityStatusData?.businessEntityStatusCategory || "-"}`,
    `  ${chalk.gray("USPC:")}          ${m?.uspcSymbolText || "-"}`,
    `  ${chalk.gray("CPC:")}           ${(m?.cpcClassificationBag || []).join(", ") || "-"}`,
    `  ${chalk.gray("Publication:")}   ${m?.earliestPublicationNumber || "-"} (${m?.earliestPublicationDate || "-"})`,
    `  ${chalk.gray("AIA/FITF:")}      ${m?.firstInventorToFileIndicator || "-"}`,
    "",
  ];
  return lines.join("\n");
}

export function formatDocumentTable(docs: Document[]): string {
  if (!docs?.length) return chalk.yellow("No documents found.");

  const table = new Table({
    head: [
      chalk.cyan("#"),
      chalk.cyan("Date"),
      chalk.cyan("Code"),
      chalk.cyan("Description"),
      chalk.cyan("Direction"),
      chalk.cyan("Pages"),
    ],
    colWidths: [5, 13, 8, 45, 12, 7],
    wordWrap: true,
  });

  docs.forEach((d, i) => {
    const date = d.officialDate ? d.officialDate.split("T")[0] : "";
    const pages = d.downloadOptionBag?.[0]?.pageTotalQuantity || "-";
    table.push([
      i + 1,
      date,
      d.documentCode || "",
      (d.documentCodeDescriptionText || "").substring(0, 60),
      d.documentDirectionCategory || "",
      pages,
    ]);
  });

  return table.toString();
}

export function formatTransactionTable(events: EventData[]): string {
  if (!events?.length) return chalk.yellow("No transactions found.");

  const table = new Table({
    head: [chalk.cyan("Date"), chalk.cyan("Code"), chalk.cyan("Description")],
    colWidths: [13, 8, 80],
    wordWrap: true,
  });

  for (const e of events) {
    table.push([e.eventDate || "", e.eventCode || "", e.eventDescriptionText || ""]);
  }

  return table.toString();
}

export function formatContinuityTable(parents: ContinuityData[], children: ContinuityData[]): string {
  const lines: string[] = [];

  if (parents?.length) {
    lines.push(chalk.bold("\n  Parent Applications:"));
    const table = new Table({
      head: [chalk.cyan("App #"), chalk.cyan("Patent #"), chalk.cyan("Type"), chalk.cyan("Filing Date"), chalk.cyan("Status")],
      colWidths: [16, 12, 8, 13, 30],
      wordWrap: true,
    });
    for (const p of parents) {
      table.push([
        p.parentApplicationNumberText || "",
        p.parentPatentNumber || "-",
        p.claimParentageTypeCode || "",
        p.parentApplicationFilingDate || "",
        p.parentApplicationStatusDescriptionText || "",
      ]);
    }
    lines.push(table.toString());
  }

  if (children?.length) {
    lines.push(chalk.bold("\n  Child Applications:"));
    const table = new Table({
      head: [chalk.cyan("App #"), chalk.cyan("Patent #"), chalk.cyan("Type"), chalk.cyan("Filing Date"), chalk.cyan("Status")],
      colWidths: [16, 12, 8, 13, 30],
      wordWrap: true,
    });
    for (const c of children) {
      table.push([
        c.childApplicationNumberText || "",
        c.childPatentNumber || "-",
        c.claimParentageTypeCode || "",
        c.childApplicationFilingDate || "",
        c.childApplicationStatusDescriptionText || "",
      ]);
    }
    lines.push(table.toString());
  }

  return lines.length ? lines.join("\n") : chalk.yellow("No continuity data found.");
}

export function formatAssignmentTable(assignments: Assignment[]): string {
  if (!assignments?.length) return chalk.yellow("No assignments found.");

  const table = new Table({
    head: [chalk.cyan("Reel/Frame"), chalk.cyan("Recorded"), chalk.cyan("Conveyance"), chalk.cyan("Assignor"), chalk.cyan("Assignee")],
    colWidths: [14, 13, 25, 25, 25],
    wordWrap: true,
  });

  for (const a of assignments) {
    const assignors = (a.assignorBag || []).map((x: any) => x.name || x.assignorName || "").join(", ");
    const assignees = (a.assigneeBag || []).map((x: any) => x.name || x.assigneeName || "").join(", ");
    table.push([
      a.reelAndFrameNumber || `${a.reelNumber}/${a.frameNumber}`,
      a.assignmentRecordedDate || "",
      (a.conveyanceText || "").substring(0, 40),
      assignors.substring(0, 30),
      assignees.substring(0, 30),
    ]);
  }

  return table.toString();
}

export function formatProceedingTable(proceedings: ProceedingData[]): string {
  if (!proceedings?.length) return chalk.yellow("No proceedings found.");

  const table = new Table({
    head: [chalk.cyan("Trial #"), chalk.cyan("Type"), chalk.cyan("Status"), chalk.cyan("Patent #"), chalk.cyan("Owner"), chalk.cyan("Petitioner")],
    colWidths: [18, 6, 16, 12, 25, 25],
    wordWrap: true,
  });

  for (const p of proceedings) {
    table.push([
      p.trialNumber || "",
      p.trialMetaData?.trialTypeCode || "",
      p.trialMetaData?.trialStatusCategory || "",
      p.patentOwnerData?.patentNumber || "",
      (p.patentOwnerData?.patentOwnerName || "").substring(0, 30),
      (p.regularPetitionerData?.realPartyInInterestName || "").substring(0, 30),
    ]);
  }

  return table.toString();
}

export function formatBulkDataTable(products: BulkDataProduct[]): string {
  if (!products?.length) return chalk.yellow("No bulk data products found.");

  const table = new Table({
    head: [chalk.cyan("ID"), chalk.cyan("Title"), chalk.cyan("Freq"), chalk.cyan("Files"), chalk.cyan("Size")],
    colWidths: [15, 50, 10, 7, 15],
    wordWrap: true,
  });

  for (const p of products) {
    const sizeMB = p.productTotalFileSize ? `${(p.productTotalFileSize / 1024 / 1024).toFixed(0)} MB` : "-";
    table.push([
      p.productIdentifier || "",
      (p.productTitleText || "").substring(0, 60),
      p.productFrequencyText || "",
      p.productFileTotalQuantity || 0,
      sizeMB,
    ]);
  }

  return table.toString();
}

export function formatCount(count: number, label: string): string {
  return chalk.gray(`${count} ${label} found`);
}
