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
};
