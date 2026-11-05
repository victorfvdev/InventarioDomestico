"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { SiteHeader } from "@/components/site-header";
import { LoadingBlock } from "@/components/loading-block";
import { ErrorBlock } from "@/components/error-block";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  api,
  type FiscalDocument,
  type FiscalItem,
  type Session,
} from "@/lib/api";

type EditableItem = FiscalItem & {
  quantityInput: string;
  productChoice: string; // productId | "__new__" | ""
};

type Mode = "key" | "manual" | "photo";

function matchLabel(status: FiscalItem["matchStatus"]) {
  switch (status) {
    case "exact":
      return "Match exato";
    case "suggested":
      return "Sugestão";
    case "ambiguous":
      return "Ambiguidade";
    case "manual":
      return "Manual";
    default:
      return "Novo produto";
  }
}

function matchVariant(status: FiscalItem["matchStatus"]): "confirmed" | "estimated" | "muted" {
  if (status === "exact") return "confirmed";
  if (status === "ambiguous") return "estimated";
  return "muted";
}

export default function NFCePage() {
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);
  const [mode, setMode] = useState<Mode>("key");
  const [accessKey, setAccessKey] = useState("");
  const [fixtureKey, setFixtureKey] = useState("");
  const [loading, setLoading] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notes, setNotes] = useState<string | null>(null);
  const [limits, setLimits] = useState<string[]>([]);
  const [document, setDocument] = useState<FiscalDocument | null>(null);
  const [items, setItems] = useState<EditableItem[]>([]);
  const [doneMessage, setDoneMessage] = useState<string | null>(null);
  const [usedFixture, setUsedFixture] = useState(false);

  // manual fallback
  const [manualIssuer, setManualIssuer] = useState("Supermercado");
  const [manualRows, setManualRows] = useState([
    { description: "", quantity: "1", unit: "un", unitPrice: "0", category: "geral" },
  ]);

  // photo stub
  const [photo, setPhoto] = useState<File | null>(null);

  useEffect(() => {
    api.session().then(setSession).catch(() => undefined);
    api
      .nfceFixture()
      .then((f) => {
        setFixtureKey(f.accessKey);
        setLimits(f.sefazLimits ?? []);
      })
      .catch(() => undefined);
  }, []);

  const includedCount = useMemo(() => items.filter((i) => i.included).length, [items]);

  function hydrateDocument(doc: FiscalDocument, meta?: { notes?: string; usedFixture?: boolean; sefazLimits?: string[] }) {
    setDocument(doc);
    setNotes(meta?.notes ?? doc.fetchNotes ?? null);
    setUsedFixture(Boolean(meta?.usedFixture ?? doc.source === "fixture"));
    if (meta?.sefazLimits?.length) setLimits(meta.sefazLimits);
    setItems(
      (doc.items ?? []).map((item) => {
        let productChoice = "";
        if (item.matchStatus === "ambiguous") {
          productChoice = "";
        } else if (item.matchedProductId) {
          productChoice = item.matchedProductId;
        } else {
          productChoice = "__new__";
        }
        return {
          ...item,
          quantityInput: String(item.quantity),
          productChoice,
        };
      }),
    );
  }

  async function onLookup(e: FormEvent, forceFixture = false) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setDoneMessage(null);
    setDocument(null);
    setItems([]);
    try {
      const body = forceFixture
        ? { useFixture: true, accessKey: fixtureKey || undefined }
        : { accessKey: accessKey.trim() || undefined, useFixture: false };
      if (!forceFixture && !accessKey.trim()) {
        throw new Error("Cole a chave de 44 dígitos ou a URL do QR Code.");
      }
      const result = await api.nfceLookup(body);
      if (result.document.alreadyPosted) {
        setDoneMessage(result.notes || "Esta NFC-e já entrou no estoque.");
      }
      hydrateDocument(result.document, {
        notes: result.notes,
        usedFixture: result.usedFixture,
        sefazLimits: result.sefazLimits,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha na consulta NFC-e");
    } finally {
      setLoading(false);
    }
  }

  async function onManual(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setDoneMessage(null);
    try {
      const payloadItems = manualRows
        .filter((r) => r.description.trim())
        .map((r) => ({
          description: r.description.trim(),
          quantity: Number(r.quantity),
          unit: r.unit || "un",
          unitPrice: Number(r.unitPrice || 0),
          category: r.category || "geral",
        }));
      if (payloadItems.length === 0) {
        throw new Error("Adicione ao menos um item manual.");
      }
      const result = await api.nfceManual({
        accessKey: accessKey.trim() || undefined,
        issuerName: manualIssuer,
        notes: "Entrada manual de itens da compra",
        items: payloadItems,
      });
      hydrateDocument(result.document, { notes: result.document.fetchNotes, usedFixture: false });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha na entrada manual");
    } finally {
      setLoading(false);
    }
  }

  async function onPhoto(e: FormEvent) {
    e.preventDefault();
    if (!photo) {
      setError("Escolha uma foto do cupom (stub local, sem OpenAI).");
      return;
    }
    setLoading(true);
    setError(null);
    setDoneMessage(null);
    try {
      const result = await api.nfcePhotoStub(photo);
      hydrateDocument(result.document, { notes: result.message, usedFixture: false });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha no stub de foto");
    } finally {
      setLoading(false);
    }
  }

  function updateItem(id: string, patch: Partial<EditableItem>) {
    setItems((prev) => prev.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  }

  async function onConfirm() {
    if (!document) return;
    setConfirming(true);
    setError(null);
    try {
      for (const item of items) {
        if (item.included && item.matchStatus === "ambiguous" && !item.productChoice) {
          throw new Error(`Resolva a ambiguidade de «${item.description}» antes de confirmar.`);
        }
      }
      const payload = items.map((item) => ({
        itemId: item.id,
        included: item.included,
        description: item.description,
        quantity: Number(item.quantityInput),
        unit: item.unit,
        category: item.category,
        productId: item.productChoice && item.productChoice !== "__new__" ? item.productChoice : null,
        createNew: item.productChoice === "__new__" || (!item.productChoice && item.matchStatus === "new"),
      }));
      const result = await api.nfceConfirm(document.id, payload);
      setDoneMessage(result.message);
      setDocument(result.document);
      setTimeout(() => router.push("/"), 900);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao confirmar compra");
    } finally {
      setConfirming(false);
    }
  }

  return (
    <div className="min-h-screen pantry-grid">
      <SiteHeader household={session?.household.name} />
      <main className="mx-auto max-w-3xl px-4 py-8 sm:px-6">
        <section className="animate-rise mb-8">
          <h1 className="font-[family-name:var(--font-display)] text-4xl text-[var(--brand)] sm:text-5xl">
            NFC-e Mato Grosso
          </h1>
          <p className="mt-3 max-w-2xl text-[var(--muted)]">
            Fluxo B: chave ou QR → itens → associação aos produtos → confirmação → estoque.
            Mesma chave nunca duplica entrada.
          </p>
        </section>

        <div className="animate-rise-delay mb-6 flex flex-wrap gap-2">
          {(
            [
              ["key", "Chave / QR"],
              ["manual", "Itens manuais"],
              ["photo", "Foto do cupom"],
            ] as const
          ).map(([id, label]) => (
            <Button
              key={id}
              type="button"
              size="sm"
              variant={mode === id ? "default" : "outline"}
              onClick={() => {
                setMode(id);
                setError(null);
              }}
            >
              {label}
            </Button>
          ))}
        </div>

        {mode === "key" ? (
          <form onSubmit={(e) => onLookup(e, false)} className="animate-rise mb-6 space-y-4">
            <div className="space-y-2">
              <Label htmlFor="accessKey">Chave de acesso (44 dígitos) ou URL do QR</Label>
              <Input
                id="accessKey"
                value={accessKey}
                onChange={(e) => setAccessKey(e.target.value)}
                placeholder="5124… ou https://www.sefaz.mt.gov.br/nfce/..."
              />
            </div>
            <div className="flex flex-wrap gap-3">
              <Button type="submit" disabled={loading} size="lg">
                {loading ? "Consultando…" : "Buscar NFC-e"}
              </Button>
              <Button
                type="button"
                variant="secondary"
                disabled={loading}
                onClick={(e) => onLookup(e as unknown as FormEvent, true)}
              >
                Usar fixture Cuiabá
              </Button>
            </div>
            {fixtureKey ? (
              <p className="text-xs text-[var(--muted)]">
                Fixture demo: <code className="break-all">{fixtureKey}</code>
              </p>
            ) : null}
          </form>
        ) : null}

        {mode === "manual" ? (
          <form onSubmit={onManual} className="animate-rise mb-6 space-y-4">
            <div className="space-y-2">
              <Label htmlFor="issuer">Emitente</Label>
              <Input id="issuer" value={manualIssuer} onChange={(e) => setManualIssuer(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="manualKey">Chave (opcional, para idempotência)</Label>
              <Input
                id="manualKey"
                value={accessKey}
                onChange={(e) => setAccessKey(e.target.value)}
                placeholder="44 dígitos MT, se tiver"
              />
            </div>
            {manualRows.map((row, idx) => (
              <div key={idx} className="grid gap-2 rounded-md border border-[var(--line)]/80 bg-[var(--paper)]/70 p-3 sm:grid-cols-4">
                <div className="sm:col-span-2 space-y-1">
                  <Label>Descrição</Label>
                  <Input
                    value={row.description}
                    onChange={(e) => {
                      const next = [...manualRows];
                      next[idx] = { ...row, description: e.target.value };
                      setManualRows(next);
                    }}
                    required={idx === 0}
                  />
                </div>
                <div className="space-y-1">
                  <Label>Qtd</Label>
                  <Input
                    type="number"
                    min="0"
                    step="0.001"
                    value={row.quantity}
                    onChange={(e) => {
                      const next = [...manualRows];
                      next[idx] = { ...row, quantity: e.target.value };
                      setManualRows(next);
                    }}
                  />
                </div>
                <div className="space-y-1">
                  <Label>Unidade</Label>
                  <Input
                    value={row.unit}
                    onChange={(e) => {
                      const next = [...manualRows];
                      next[idx] = { ...row, unit: e.target.value };
                      setManualRows(next);
                    }}
                  />
                </div>
              </div>
            ))}
            <div className="flex flex-wrap gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  setManualRows((prev) => [
                    ...prev,
                    { description: "", quantity: "1", unit: "un", unitPrice: "0", category: "geral" },
                  ])
                }
              >
                + item
              </Button>
              <Button type="submit" disabled={loading} size="lg">
                {loading ? "Montando…" : "Preparar revisão"}
              </Button>
            </div>
          </form>
        ) : null}

        {mode === "photo" ? (
          <form onSubmit={onPhoto} className="animate-rise mb-6 space-y-4">
            <p className="text-sm text-[var(--muted)]">
              Stub local sem OpenAI: a foto só dispara sugestões típicas de cupom MT para você revisar.
            </p>
            <div className="space-y-2">
              <Label htmlFor="photo">Foto do cupom</Label>
              <Input
                id="photo"
                type="file"
                accept="image/*"
                onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
              />
            </div>
            <Button type="submit" disabled={loading} size="lg">
              {loading ? "Interpretando stub…" : "Usar stub de cupom"}
            </Button>
          </form>
        ) : null}

        {loading ? <LoadingBlock label="Processando NFC-e…" /> : null}
        {error ? <ErrorBlock message={error} /> : null}
        {doneMessage ? <p className="mb-4 text-sm text-[var(--ok)]">{doneMessage}</p> : null}

        {document && !document.alreadyPosted ? (
          <section className="animate-rise space-y-4">
            <div className="rounded-md border border-[var(--line)]/70 bg-[var(--paper)]/80 p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h2 className="font-[family-name:var(--font-display)] text-2xl text-[var(--brand)]">
                    {document.issuerName || "Documento fiscal"}
                  </h2>
                  <p className="mt-1 text-sm text-[var(--muted)]">
                    {document.uf} · origem {document.source}
                    {usedFixture ? " · fixture offline" : ""}
                    {document.accessKey ? ` · ${document.accessKey}` : ""}
                  </p>
                </div>
                <Badge variant="muted">Total R$ {document.totalAmount}</Badge>
              </div>
              {notes ? <p className="mt-3 text-sm text-[var(--muted)]">{notes}</p> : null}
              {limits.length > 0 ? (
                <ul className="mt-3 list-disc space-y-1 pl-5 text-xs text-[var(--muted)]">
                  {limits.map((l) => (
                    <li key={l}>{l}</li>
                  ))}
                </ul>
              ) : null}
            </div>

            <div className="space-y-3">
              {items.map((item) => (
                <div
                  key={item.id}
                  className="rounded-md border border-[var(--line)]/70 bg-[var(--paper)]/70 p-4 transition-opacity"
                  style={{ opacity: item.included ? 1 : 0.55 }}
                >
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <label className="flex items-start gap-2">
                      <input
                        type="checkbox"
                        className="mt-1"
                        checked={item.included}
                        onChange={(e) => updateItem(item.id, { included: e.target.checked })}
                      />
                      <span>
                        <span className="font-medium">{item.description}</span>
                        <span className="mt-1 block text-xs text-[var(--muted)]">
                          linha {item.lineNumber} · {item.category}
                        </span>
                      </span>
                    </label>
                    <Badge variant={matchVariant(item.matchStatus)}>{matchLabel(item.matchStatus)}</Badge>
                  </div>

                  <div className="mt-3 grid gap-3 sm:grid-cols-3">
                    <div className="space-y-1">
                      <Label>Quantidade</Label>
                      <Input
                        type="number"
                        min="0"
                        step="0.001"
                        value={item.quantityInput}
                        onChange={(e) => updateItem(item.id, { quantityInput: e.target.value })}
                        disabled={!item.included}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label>Unidade</Label>
                      <Input
                        value={item.unit}
                        onChange={(e) => updateItem(item.id, { unit: e.target.value })}
                        disabled={!item.included}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label>Associar a</Label>
                      <select
                        className="flex h-11 w-full rounded-md border border-[var(--line)] bg-transparent px-3 text-sm"
                        value={item.productChoice}
                        disabled={!item.included}
                        onChange={(e) => updateItem(item.id, { productChoice: e.target.value })}
                      >
                        <option value="">
                          {item.matchStatus === "ambiguous" ? "Escolha (obrigatório)" : "Selecionar…"}
                        </option>
                        <option value="__new__">Criar produto novo</option>
                        {(item.ambiguousCandidates ?? []).map((c) => (
                          <option key={c.productId} value={c.productId}>
                            {c.description} ({Math.round(c.score * 100)}%)
                          </option>
                        ))}
                        {item.matchedProductId &&
                        !(item.ambiguousCandidates ?? []).some((c) => c.productId === item.matchedProductId) ? (
                          <option value={item.matchedProductId}>
                            {item.matchedProductName || "Produto sugerido"}
                          </option>
                        ) : null}
                      </select>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <div className="flex flex-wrap items-center gap-3 pt-2">
              <Button type="button" size="lg" disabled={confirming || includedCount === 0} onClick={onConfirm}>
                {confirming ? "Confirmando…" : `Confirmar ${includedCount} item(ns) no estoque`}
              </Button>
              <p className="text-sm text-[var(--muted)]">
                Ambiguidade exige escolha humana. Movimentações vão como purchase_entry.
              </p>
            </div>
          </section>
        ) : null}

        {document?.alreadyPosted ? (
          <section className="animate-rise rounded-md border border-[var(--ok)]/30 bg-[var(--ok-soft)] p-4">
            <h2 className="font-[family-name:var(--font-display)] text-xl text-[var(--ok)]">
              Já lançada no estoque
            </h2>
            <p className="mt-2 text-sm text-[var(--ink)]">
              A chave {document.accessKey} já gerou purchase_entry. Idempotência impediu nova postagem.
            </p>
            <Button asChild className="mt-4" variant="outline">
              <Link href="/movimentacoes">Ver movimentações</Link>
            </Button>
          </section>
        ) : null}
      </main>
    </div>
  );
}
