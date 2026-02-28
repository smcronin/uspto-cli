import { describe, test, expect, beforeAll } from "bun:test";
import "dotenv/config";
import { createClient, UsptoClient, UsptoApiError } from "../../src/api/client";

let client: UsptoClient;

// Known test data
const TEST_APP_NUMBER = "16123456"; // A real application
const TEST_PATENT_NUMBER = "10902286"; // Its patent number
const TEST_IPR_NUMBER = "IPR2020-00388";

beforeAll(() => {
  client = createClient();
});

describe("Patent Application API", () => {
  test("search patents by title", async () => {
    const result = await client.searchPatents("applicationMetaData.inventionTitle:wireless", { limit: 5 });
    expect(result.count).toBeGreaterThan(0);
    expect(result.patentFileWrapperDataBag).toBeInstanceOf(Array);
    expect(result.patentFileWrapperDataBag.length).toBeGreaterThan(0);
    expect(result.patentFileWrapperDataBag.length).toBeLessThanOrEqual(5);
  });

  test("search patents by applicant", async () => {
    const result = await client.searchPatents('applicationMetaData.firstApplicantName:"Apple Inc."', { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
  });

  test("search patents with POST body", async () => {
    const result = await client.searchPatentsPost({
      q: "applicationMetaData.inventionTitle:battery",
      pagination: { offset: 0, limit: 3 },
      sort: [{ field: "applicationMetaData.filingDate", order: "desc" }],
    });
    expect(result.count).toBeGreaterThan(0);
    expect(result.patentFileWrapperDataBag.length).toBeLessThanOrEqual(3);
  });

  test("get application by number", async () => {
    const result = await client.getApplication(TEST_APP_NUMBER);
    expect(result.count).toBe(1);
    expect(result.patentFileWrapperDataBag[0].applicationNumberText).toBe(TEST_APP_NUMBER);
  });

  test("get application metadata", async () => {
    const result = await client.getMetadata(TEST_APP_NUMBER);
    expect(result.patentFileWrapperDataBag).toBeDefined();
    const meta = result.patentFileWrapperDataBag[0].applicationMetaData;
    expect(meta.inventionTitle).toBeTruthy();
  });

  test("get patent term adjustment", async () => {
    const result = await client.getAdjustment(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });

  test("get assignment data", async () => {
    const result = await client.getAssignment(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });

  test("get attorney data", async () => {
    const result = await client.getAttorney(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });

  test("get continuity data", async () => {
    const result = await client.getContinuity(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });

  test("get foreign priority", async () => {
    const result = await client.getForeignPriority(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });

  test("get transaction history", async () => {
    const result = await client.getTransactions(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });

  test("get documents list", async () => {
    const result = await client.getDocuments(TEST_APP_NUMBER);
    expect(result.documentBag).toBeInstanceOf(Array);
    expect(result.documentBag.length).toBeGreaterThan(0);
    expect(result.documentBag[0].documentCode).toBeTruthy();
  });

  test("get documents filtered by code", async () => {
    const result = await client.getDocuments(TEST_APP_NUMBER, { documentCodes: "CLM" });
    expect(result).toBeDefined();
  });

  test("get associated documents (XML)", async () => {
    const result = await client.getAssociatedDocuments(TEST_APP_NUMBER);
    expect(result).toBeDefined();
  });
});

describe("Status Codes API", () => {
  test("search status codes by number", async () => {
    const result = await client.searchStatusCodes("applicationStatusCode:150");
    expect(result.count).toBeGreaterThan(0);
    expect(result.statusCodeBag[0].applicationStatusCode).toBe(150);
    expect(result.statusCodeBag[0].applicationStatusDescriptionText).toBe("Patented Case");
  });

  test("search status codes by text", async () => {
    const result = await client.searchStatusCodes("rejection", { limit: 5 });
    expect(result.count).toBeGreaterThan(0);
  });
});

describe("Bulk Data API", () => {
  test("search bulk data products", async () => {
    const result = await client.searchBulkData("patent", { limit: 5 });
    expect(result.count).toBeGreaterThan(0);
    expect(result.bulkDataProductBag).toBeInstanceOf(Array);
  });

  test("get bulk data product details", async () => {
    const result = await client.getBulkDataProduct("PTFWPRE", { includeFiles: true });
    expect(result).toBeDefined();
  });
});

describe("PTAB API", () => {
  test("search trial proceedings (IPR)", async () => {
    const result = await client.searchProceedings("trialMetaData.trialTypeCode:IPR", { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
    expect(result.patentTrialProceedingDataBag).toBeInstanceOf(Array);
  });

  test("search trial decisions", async () => {
    const result = await client.searchTrialDecisions(undefined, { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
  });

  test("search trial documents", async () => {
    const result = await client.searchTrialDocuments(undefined, { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
  });

  test("search appeal decisions", async () => {
    const result = await client.searchAppealDecisions(undefined, { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
  });

  test("search interference decisions", async () => {
    const result = await client.searchInterferenceDecisions(undefined, { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
  });
});

describe("Petition Decision API", () => {
  test("search petition decisions", async () => {
    const result = await client.searchPetitionDecisions(undefined, { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
    expect(result.petitionDecisionDataBag).toBeInstanceOf(Array);
  });

  test("search petition decisions by office", async () => {
    const result = await client.searchPetitionDecisions('finalDecidingOfficeName:"OFFICE OF PETITIONS"', { limit: 3 });
    expect(result.count).toBeGreaterThan(0);
  });
});

describe("Error Handling", () => {
  test("invalid application number returns 404", async () => {
    try {
      await client.getApplication("00000000");
      expect(true).toBe(false); // Should not reach here
    } catch (err) {
      expect(err).toBeInstanceOf(UsptoApiError);
      expect((err as UsptoApiError).statusCode).toBe(404);
    }
  });

  test("invalid API key returns 403", async () => {
    const badClient = new (await import("../../src/api/client")).UsptoClient({
      apiKey: "invalid-key",
    });
    try {
      await badClient.searchPatents("test", { limit: 1 });
      expect(true).toBe(false);
    } catch (err) {
      expect(err).toBeInstanceOf(UsptoApiError);
      expect((err as UsptoApiError).statusCode).toBe(403);
    }
  });
});
