export type ConfirmationStatus = "estimated" | "confirmed";

export type Product = {
  id: string;
  householdId: string;
  description: string;
  descriptionNormalized: string;
  category: string;
  unit: string;
  quantity: string;
  minQuantity?: string | null;
  confirmationStatus: ConfirmationStatus;
  createdAt: string;
  updatedAt: string;
};

export type Movement = {
  id: string;
  householdId: string;
  productId: string;
  movementType: string;
  quantityDelta: string;
  balanceAfter: string;
  origin: string;
  notes?: string;
  isConsumption: boolean;
  createdAt: string;
  productName?: string;
};

export type VisionSuggestion = {
  tempId: string;
  description: string;
  category: string;
  unit: string;
  quantity: string;
  confidence: number;
  notes?: string;
  included: boolean;
};

export type Interpretation = {
  id: string;
  householdId: string;
  sourceType: string;
  imageFilename?: string;
  provider: string;
  suggestions: VisionSuggestion[];
  decision: string;
  overallConfidence?: number;
  createdAt: string;
};

export type Session = {
  user: { id: string; displayName: string; householdId: string };
  household: { id: string; name: string };
};

export type ProductMatch = {
  productId: string;
  description: string;
  score: number;
};

export type FiscalItem = {
  id: string;
  fiscalDocumentId: string;
  lineNumber: number;
  description: string;
  descriptionNormalized: string;
  quantity: string;
  unit: string;
  unitPrice: string;
  totalPrice: string;
  ean?: string;
  category: string;
  matchStatus: "exact" | "suggested" | "ambiguous" | "new" | "manual";
  matchConfidence?: number;
  matchedProductId?: string | null;
  matchedProductName?: string;
  createNew: boolean;
  included: boolean;
  ambiguousCandidates?: ProductMatch[];
};

export type FiscalDocument = {
  id: string;
  householdId: string;
  accessKey: string;
  uf: string;
  documentType: string;
  issuerName: string;
  issuerCnpj: string;
  issuedAt?: string;
  totalAmount: string;
  status: string;
  source: string;
  fetchNotes?: string;
  createdAt: string;
  confirmedAt?: string;
  items?: FiscalItem[];
  alreadyPosted?: boolean;
};

export type NFCeLookupResult = {
  document: FiscalDocument;
  liveTried: boolean;
  liveOk: boolean;
  usedFixture: boolean;
  notes: string;
  sefazLimits: string[];
};

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://127.0.0.1:3847";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      ...(init?.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
      ...init?.headers,
    },
    cache: "no-store",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || `Erro ${res.status}`);
  }
  return data as T;
}

export const api = {
  baseUrl: API_BASE,
  session: () => request<Session>("/api/session"),
  products: () => request<{ products: Product[] }>("/api/products"),
  movements: (limit = 30) =>
    request<{ movements: Movement[] }>(`/api/movements?limit=${limit}`),
  manualEntry: (body: {
    description: string;
    category: string;
    unit: string;
    quantity: number;
    minQuantity?: number;
    notes?: string;
  }) =>
    request<{ product: Product; movement: Movement }>("/api/products/manual-entry", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  adjust: (productId: string, newQuantity: number, notes?: string) =>
    request<{ product: Product; movement: Movement }>(`/api/products/${productId}/adjust`, {
      method: "POST",
      body: JSON.stringify({ productId, newQuantity, notes }),
    }),
  interpretPhoto: async (file: File) => {
    const form = new FormData();
    form.append("photo", file);
    return request<{ interpretation: Interpretation; provider: string; mocked: boolean }>(
      "/api/vision/interpret",
      { method: "POST", body: form },
    );
  },
  confirmInitialLoad: (body: {
    interpretationId: string;
    items: Array<{
      tempId: string;
      description: string;
      category: string;
      unit: string;
      quantity: number;
      included: boolean;
      notes?: string;
    }>;
  }) =>
    request<{ products: Product[]; movements: Movement[]; message: string }>(
      "/api/vision/confirm-initial-load",
      { method: "POST", body: JSON.stringify(body) },
    ),
  nfceFixture: () =>
    request<{
      accessKey: string;
      uf: string;
      city: string;
      issuer: string;
      qrUrl: string;
      howTo: string;
      sefazLimits: string[];
    }>("/api/nfce/fixture"),
  nfceLookup: (body: { accessKey?: string; qrUrl?: string; useFixture?: boolean }) =>
    request<NFCeLookupResult>("/api/nfce/lookup", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  nfceManual: (body: {
    accessKey?: string;
    issuerName?: string;
    notes?: string;
    items: Array<{
      description: string;
      quantity: number;
      unit: string;
      unitPrice?: number;
      category?: string;
    }>;
  }) =>
    request<{ document: FiscalDocument }>("/api/nfce/manual", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  nfcePhotoStub: async (file: File) => {
    const form = new FormData();
    form.append("photo", file);
    return request<{ document: FiscalDocument; mocked: boolean; provider: string; message: string }>(
      "/api/nfce/photo-stub",
      { method: "POST", body: form },
    );
  },
  nfceConfirm: (
    documentId: string,
    items: Array<{
      itemId: string;
      included: boolean;
      description: string;
      quantity: number;
      unit: string;
      category: string;
      productId?: string | null;
      createNew: boolean;
    }>,
  ) =>
    request<{ document: FiscalDocument; movements: Movement[]; message: string }>(
      `/api/nfce/documents/${documentId}/confirm`,
      { method: "POST", body: JSON.stringify({ documentId, items }) },
    ),
};
