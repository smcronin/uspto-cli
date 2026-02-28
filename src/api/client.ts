import type {
  SearchRequest,
  DownloadRequest,
  PatentDataResponse,
  DocumentBagResponse,
  StatusCodeResponse,
  BulkDataResponse,
  ProceedingDataResponse,
  TrialDocumentResponse,
  AppealDecisionResponse,
  InterferenceDecisionResponse,
  PetitionDecisionResponse,
  ErrorResponse,
} from "../types/api";

export interface ClientConfig {
  apiKey: string;
  baseUrl?: string;
  debug?: boolean;
}

export class UsptoApiError extends Error {
  constructor(
    public statusCode: number,
    public errorBody: ErrorResponse,
  ) {
    super(`USPTO API Error ${statusCode}: ${errorBody.error || errorBody.message} - ${errorBody.errorDetails || errorBody.detailedMessage || ""}`);
    this.name = "UsptoApiError";
  }
}

export class RateLimiter {
  private timestamps: number[] = [];
  private downloadTimestamps: number[] = [];

  private get isPeakHours(): boolean {
    const now = new Date();
    const estHour = (now.getUTCHours() - 5 + 24) % 24;
    return estHour >= 5 && estHour < 22;
  }

  get maxRequestsPerMinute(): number {
    return this.isPeakHours ? 60 : 120;
  }

  get maxDownloadsPerMinute(): number {
    return this.isPeakHours ? 4 : 12;
  }

  async waitForSlot(isDownload = false): Promise<void> {
    const now = Date.now();
    const oneMinuteAgo = now - 60_000;

    this.timestamps = this.timestamps.filter((t) => t > oneMinuteAgo);
    if (isDownload) {
      this.downloadTimestamps = this.downloadTimestamps.filter((t) => t > oneMinuteAgo);
    }

    const limit = isDownload ? this.maxDownloadsPerMinute : this.maxRequestsPerMinute;
    const stamps = isDownload ? this.downloadTimestamps : this.timestamps;

    if (stamps.length >= limit) {
      const waitMs = stamps[0] - oneMinuteAgo + 100;
      await new Promise((resolve) => setTimeout(resolve, waitMs));
    }

    this.timestamps.push(Date.now());
    if (isDownload) {
      this.downloadTimestamps.push(Date.now());
    }
  }
}

export class UsptoClient {
  private config: ClientConfig;
  private rateLimiter = new RateLimiter();

  constructor(config: ClientConfig) {
    this.config = {
      baseUrl: "https://api.uspto.gov",
      ...config,
    };
  }

  private get headers(): Record<string, string> {
    return {
      "X-API-KEY": this.config.apiKey,
      "Content-Type": "application/json",
      Accept: "application/json",
    };
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    options: { body?: any; params?: Record<string, string>; isDownload?: boolean } = {},
  ): Promise<T> {
    await this.rateLimiter.waitForSlot(options.isDownload);

    let url = `${this.config.baseUrl}${path}`;
    if (options.params) {
      const searchParams = new URLSearchParams();
      for (const [key, value] of Object.entries(options.params)) {
        if (value !== undefined && value !== "") {
          searchParams.set(key, value);
        }
      }
      const qs = searchParams.toString();
      if (qs) url += `?${qs}`;
    }

    if (this.config.debug) {
      console.error(`[DEBUG] ${method} ${url}`);
      if (options.body) console.error(`[DEBUG] Body: ${JSON.stringify(options.body, null, 2)}`);
    }

    const response = await fetch(url, {
      method,
      headers: this.headers,
      body: options.body ? JSON.stringify(options.body) : undefined,
    });

    if (!response.ok) {
      let errorBody: ErrorResponse;
      try {
        errorBody = await response.json();
      } catch {
        errorBody = {
          code: response.status,
          error: response.statusText,
          errorDetails: `HTTP ${response.status}`,
        };
      }
      throw new UsptoApiError(response.status, errorBody);
    }

    return response.json() as Promise<T>;
  }

  // ─── Patent Application Endpoints ────────────────────────────

  async searchPatents(query?: string, opts: { limit?: number; offset?: number; sort?: string; fields?: string; filters?: string; facets?: string } = {}): Promise<PatentDataResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    if (opts.sort) params.sort = opts.sort;
    if (opts.fields) params.fields = opts.fields;
    if (opts.filters) params.filters = opts.filters;
    if (opts.facets) params.facets = opts.facets;
    return this.request<PatentDataResponse>("GET", "/api/v1/patent/applications/search", { params });
  }

  async searchPatentsPost(body: SearchRequest): Promise<PatentDataResponse> {
    return this.request<PatentDataResponse>("POST", "/api/v1/patent/applications/search", { body });
  }

  async downloadPatents(query?: string, format: "json" | "csv" = "json", opts: { limit?: number; offset?: number; sort?: string } = {}): Promise<PatentDataResponse> {
    const params: Record<string, string> = { format };
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    if (opts.sort) params.sort = opts.sort;
    return this.request<PatentDataResponse>("GET", "/api/v1/patent/applications/search/download", { params, isDownload: true });
  }

  async getApplication(appNumber: string): Promise<PatentDataResponse> {
    return this.request<PatentDataResponse>("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}`);
  }

  async getMetadata(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/meta-data`);
  }

  async getAdjustment(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/adjustment`);
  }

  async getAssignment(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/assignment`);
  }

  async getAttorney(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/attorney`);
  }

  async getContinuity(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/continuity`);
  }

  async getForeignPriority(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/foreign-priority`);
  }

  async getTransactions(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/transactions`);
  }

  async getDocuments(appNumber: string, opts: { documentCodes?: string; officialDateFrom?: string; officialDateTo?: string } = {}): Promise<DocumentBagResponse> {
    const params: Record<string, string> = {};
    if (opts.documentCodes) params.documentCodes = opts.documentCodes;
    if (opts.officialDateFrom) params.officialDateFrom = opts.officialDateFrom;
    if (opts.officialDateTo) params.officialDateTo = opts.officialDateTo;
    return this.request<DocumentBagResponse>("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/documents`, { params });
  }

  async getAssociatedDocuments(appNumber: string): Promise<any> {
    return this.request("GET", `/api/v1/patent/applications/${encodeURIComponent(appNumber)}/associated-documents`);
  }

  async searchStatusCodes(query?: string, opts: { limit?: number; offset?: number } = {}): Promise<StatusCodeResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    return this.request<StatusCodeResponse>("GET", "/api/v1/patent/status-codes", { params });
  }

  // ─── Bulk Data Endpoints ─────────────────────────────────────

  async searchBulkData(query?: string, opts: { limit?: number; offset?: number } = {}): Promise<BulkDataResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    return this.request<BulkDataResponse>("GET", "/api/v1/datasets/products/search", { params });
  }

  async getBulkDataProduct(productId: string, opts: { includeFiles?: boolean; latest?: boolean } = {}): Promise<any> {
    const params: Record<string, string> = {};
    if (opts.includeFiles) params.includeFiles = "true";
    if (opts.latest) params.latest = "true";
    return this.request("GET", `/api/v1/datasets/products/${encodeURIComponent(productId)}`, { params });
  }

  // ─── PTAB Proceedings ────────────────────────────────────────

  async searchProceedings(query?: string, opts: { limit?: number; offset?: number; sort?: string } = {}): Promise<ProceedingDataResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    if (opts.sort) params.sort = opts.sort;
    return this.request<ProceedingDataResponse>("GET", "/api/v1/patent/trials/proceedings/search", { params });
  }

  async getProceeding(trialNumber: string): Promise<ProceedingDataResponse> {
    return this.request<ProceedingDataResponse>("GET", `/api/v1/patent/trials/proceedings/${encodeURIComponent(trialNumber)}`);
  }

  // ─── PTAB Trial Decisions ────────────────────────────────────

  async searchTrialDecisions(query?: string, opts: { limit?: number; offset?: number } = {}): Promise<TrialDocumentResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    return this.request<TrialDocumentResponse>("GET", "/api/v1/patent/trials/decisions/search", { params });
  }

  async getTrialDecision(documentId: string): Promise<TrialDocumentResponse> {
    return this.request<TrialDocumentResponse>("GET", `/api/v1/patent/trials/decisions/${encodeURIComponent(documentId)}`);
  }

  async getTrialDecisions(trialNumber: string): Promise<TrialDocumentResponse> {
    return this.request<TrialDocumentResponse>("GET", `/api/v1/patent/trials/${encodeURIComponent(trialNumber)}/decisions`);
  }

  // ─── PTAB Trial Documents ────────────────────────────────────

  async searchTrialDocuments(query?: string, opts: { limit?: number; offset?: number } = {}): Promise<TrialDocumentResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    return this.request<TrialDocumentResponse>("GET", "/api/v1/patent/trials/documents/search", { params });
  }

  async getTrialDocument(documentId: string): Promise<TrialDocumentResponse> {
    return this.request<TrialDocumentResponse>("GET", `/api/v1/patent/trials/documents/${encodeURIComponent(documentId)}`);
  }

  async getTrialDocuments(trialNumber: string): Promise<TrialDocumentResponse> {
    return this.request<TrialDocumentResponse>("GET", `/api/v1/patent/trials/${encodeURIComponent(trialNumber)}/documents`);
  }

  // ─── Appeal Decisions ────────────────────────────────────────

  async searchAppealDecisions(query?: string, opts: { limit?: number; offset?: number } = {}): Promise<AppealDecisionResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    return this.request<AppealDecisionResponse>("GET", "/api/v1/patent/appeals/decisions/search", { params });
  }

  async getAppealDecision(documentId: string): Promise<AppealDecisionResponse> {
    return this.request<AppealDecisionResponse>("GET", `/api/v1/patent/appeals/decisions/${encodeURIComponent(documentId)}`);
  }

  async getAppealDecisions(appealNumber: string): Promise<AppealDecisionResponse> {
    return this.request<AppealDecisionResponse>("GET", `/api/v1/patent/appeals/${encodeURIComponent(appealNumber)}/decisions`);
  }

  // ─── Interference Decisions ──────────────────────────────────

  async searchInterferenceDecisions(query?: string, opts: { limit?: number; offset?: number } = {}): Promise<InterferenceDecisionResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    return this.request<InterferenceDecisionResponse>("GET", "/api/v1/patent/interferences/decisions/search", { params });
  }

  async getInterferenceDecision(documentId: string): Promise<InterferenceDecisionResponse> {
    return this.request<InterferenceDecisionResponse>("GET", `/api/v1/patent/interferences/decisions/${encodeURIComponent(documentId)}`);
  }

  async getInterferenceDecisions(interferenceNumber: string): Promise<InterferenceDecisionResponse> {
    return this.request<InterferenceDecisionResponse>("GET", `/api/v1/patent/interferences/${encodeURIComponent(interferenceNumber)}/decisions`);
  }

  // ─── Petition Decisions ──────────────────────────────────────

  async searchPetitionDecisions(query?: string, opts: { limit?: number; offset?: number; sort?: string } = {}): Promise<PetitionDecisionResponse> {
    const params: Record<string, string> = {};
    if (query) params.q = query;
    if (opts.limit) params.limit = String(opts.limit);
    if (opts.offset) params.offset = String(opts.offset);
    if (opts.sort) params.sort = opts.sort;
    return this.request<PetitionDecisionResponse>("GET", "/api/v1/petition/decisions/search", { params });
  }

  async getPetitionDecision(recordId: string, includeDocuments = false): Promise<any> {
    const params: Record<string, string> = {};
    if (includeDocuments) params.includeDocuments = "true";
    return this.request("GET", `/api/v1/petition/decisions/${encodeURIComponent(recordId)}`, { params });
  }

  // ─── Document Download ───────────────────────────────────────

  async downloadDocument(url: string, outputPath: string): Promise<string> {
    await this.rateLimiter.waitForSlot(true);

    const response = await fetch(url, { headers: this.headers, redirect: "follow" });
    if (!response.ok) {
      throw new Error(`Download failed: HTTP ${response.status}`);
    }

    const buffer = await response.arrayBuffer();
    const { writeFile } = await import("fs/promises");
    await writeFile(outputPath, Buffer.from(buffer));
    return outputPath;
  }
}

export function createClient(config?: Partial<ClientConfig>): UsptoClient {
  const apiKey = config?.apiKey || process.env.USPTO_API_KEY;
  if (!apiKey) {
    throw new Error("USPTO API key required. Set USPTO_API_KEY env var or pass --api-key flag.");
  }
  return new UsptoClient({
    apiKey,
    baseUrl: config?.baseUrl || process.env.USPTO_API_BASE_URL || "https://api.uspto.gov",
    debug: config?.debug,
  });
}
