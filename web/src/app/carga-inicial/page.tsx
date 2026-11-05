"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { SiteHeader } from "@/components/site-header";
import { LoadingBlock } from "@/components/loading-block";
import { ErrorBlock } from "@/components/error-block";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { api, type Interpretation, type Session, type VisionSuggestion } from "@/lib/api";

type EditableItem = VisionSuggestion & { quantityInput: string };

export default function CargaInicialPage() {
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [interpreting, setInterpreting] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [interpretation, setInterpretation] = useState<Interpretation | null>(null);
  const [mocked, setMocked] = useState(false);
  const [provider, setProvider] = useState("");
  const [items, setItems] = useState<EditableItem[]>([]);
  const [doneMessage, setDoneMessage] = useState<string | null>(null);

  useEffect(() => {
    api.session().then(setSession).catch(() => undefined);
  }, []);

  useEffect(() => {
    return () => {
      if (preview) URL.revokeObjectURL(preview);
    };
  }, [preview]);

  const includedCount = useMemo(() => items.filter((i) => i.included).length, [items]);

  function onFileChange(f: File | null) {
    setFile(f);
    setInterpretation(null);
    setItems([]);
    setDoneMessage(null);
    setError(null);
    if (preview) URL.revokeObjectURL(preview);
    setPreview(f ? URL.createObjectURL(f) : null);
  }

  async function onInterpret(e: FormEvent) {
    e.preventDefault();
    if (!file) {
      setError("Escolha uma foto da despensa ou geladeira.");
      return;
    }
    setInterpreting(true);
    setError(null);
    setDoneMessage(null);
    try {
      const result = await api.interpretPhoto(file);
      setInterpretation(result.interpretation);
      setMocked(result.mocked);
      setProvider(result.provider);
      setItems(
        result.interpretation.suggestions.map((s) => ({
          ...s,
          included: s.included !== false,
          quantityInput: String(s.quantity),
        })),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha na interpretação");
    } finally {
      setInterpreting(false);
    }
  }

  function updateItem(tempId: string, patch: Partial<EditableItem>) {
    setItems((prev) => prev.map((item) => (item.tempId === tempId ? { ...item, ...patch } : item)));
  }

  function addManualItem() {
    const tempId = crypto.randomUUID();
    setItems((prev) => [
      ...prev,
      {
        tempId,
        description: "",
        category: "geral",
        unit: "un",
        quantity: "1",
        quantityInput: "1",
        confidence: 1,
        notes: "Incluído manualmente",
        included: true,
      },
    ]);
  }

  async function onConfirm() {
    if (!interpretation) return;
    const payloadItems = items.map((item) => ({
      tempId: item.tempId,
      description: item.description.trim(),
      category: item.category || "geral",
      unit: item.unit || "un",
      quantity: Number(item.quantityInput || 0),
      included: item.included,
      notes: item.notes,
    }));

    if (!payloadItems.some((i) => i.included)) {
      setError("Marque ao menos um item para confirmar o inventário.");
      return;
    }
    for (const item of payloadItems) {
      if (item.included && !item.description) {
        setError("Todo item incluído precisa de descrição.");
        return;
      }
    }

    setConfirming(true);
    setError(null);
    try {
      const result = await api.confirmInitialLoad({
        interpretationId: interpretation.id,
        items: payloadItems,
      });
      setDoneMessage(result.message);
      setTimeout(() => router.push("/"), 900);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao confirmar inventário");
    } finally {
      setConfirming(false);
    }
  }

  return (
    <div className="min-h-screen pantry-grid">
      <SiteHeader household={session?.household.name} />
      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        <section className="animate-rise mb-8 max-w-2xl">
          <p className="text-sm font-semibold uppercase tracking-[0.14em] text-[var(--accent)]">
            Fluxo A · RN01
          </p>
          <h1 className="mt-2 font-[family-name:var(--font-display)] text-4xl text-[var(--brand)] sm:text-5xl">
            Carga inicial por foto
          </h1>
          <p className="mt-3 text-[var(--muted)]">
            A visão estima produtos e quantidades. Nada vira saldo até você revisar e confirmar.
          </p>
        </section>

        <section className="animate-rise-delay grid gap-8 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <form onSubmit={onInterpret} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="photo">Foto da despensa / geladeira</Label>
              <Input
                id="photo"
                type="file"
                accept="image/*"
                capture="environment"
                onChange={(e) => onFileChange(e.target.files?.[0] ?? null)}
              />
            </div>

            {preview ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={preview}
                alt="Pré-visualização da foto enviada"
                className="max-h-72 w-full rounded-md object-cover"
              />
            ) : (
              <div className="flex h-48 items-center justify-center rounded-md border border-dashed border-[var(--line)] bg-white/40 text-sm text-[var(--muted)]">
                A foto ocupa esta área — escolha um arquivo para começar
              </div>
            )}

            <Button type="submit" disabled={interpreting || !file} size="lg">
              {interpreting ? "Interpretando…" : "Interpretar com IA"}
            </Button>

            {interpreting ? <LoadingBlock label="Analisando imagem…" /> : null}
            {error ? <ErrorBlock message={error} /> : null}
            {doneMessage ? (
              <p className="rounded-md bg-[var(--ok-soft)] px-3 py-2 text-sm text-[var(--ok)]">
                {doneMessage}
              </p>
            ) : null}
          </form>

          <div>
            {!interpretation ? (
              <div className="rounded-md border border-[var(--line)] bg-white/50 px-4 py-8 text-[var(--muted)]">
                As sugestões aparecem aqui com confiança. Corrija, remova ou inclua itens antes de
                confirmar.
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={mocked ? "estimated" : "confirmed"}>
                    {mocked ? "Mock local" : "Visão real"}
                  </Badge>
                  <span className="text-sm text-[var(--muted)]">{provider}</span>
                  {interpretation.overallConfidence != null ? (
                    <span className="text-sm text-[var(--muted)]">
                      confiança geral {(interpretation.overallConfidence * 100).toFixed(0)}%
                    </span>
                  ) : null}
                </div>

                <ul className="space-y-4">
                  {items.map((item) => (
                    <li
                      key={item.tempId}
                      className="border-b border-[var(--line)]/70 pb-4 last:border-0"
                    >
                      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                        <label className="flex items-center gap-2 text-sm font-medium">
                          <input
                            type="checkbox"
                            checked={item.included}
                            onChange={(e) => updateItem(item.tempId, { included: e.target.checked })}
                          />
                          Incluir no inventário
                        </label>
                        <Badge variant={item.confidence >= 0.7 ? "confirmed" : "estimated"}>
                          {(item.confidence * 100).toFixed(0)}% confiança
                        </Badge>
                      </div>
                      <div className="grid gap-3 sm:grid-cols-2">
                        <div className="space-y-1 sm:col-span-2">
                          <Label>Descrição</Label>
                          <Input
                            value={item.description}
                            onChange={(e) => updateItem(item.tempId, { description: e.target.value })}
                          />
                        </div>
                        <div className="space-y-1">
                          <Label>Categoria</Label>
                          <Input
                            value={item.category}
                            onChange={(e) => updateItem(item.tempId, { category: e.target.value })}
                          />
                        </div>
                        <div className="space-y-1">
                          <Label>Unidade</Label>
                          <Input
                            value={item.unit}
                            onChange={(e) => updateItem(item.tempId, { unit: e.target.value })}
                          />
                        </div>
                        <div className="space-y-1">
                          <Label>Quantidade</Label>
                          <Input
                            type="number"
                            min="0"
                            step="0.001"
                            value={item.quantityInput}
                            onChange={(e) =>
                              updateItem(item.tempId, { quantityInput: e.target.value })
                            }
                          />
                        </div>
                        <div className="space-y-1">
                          <Label>Notas</Label>
                          <Input
                            value={item.notes || ""}
                            onChange={(e) => updateItem(item.tempId, { notes: e.target.value })}
                          />
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>

                <div className="flex flex-wrap gap-3">
                  <Button type="button" variant="secondary" onClick={addManualItem}>
                    Incluir item manual
                  </Button>
                  <Button
                    type="button"
                    size="lg"
                    disabled={confirming || includedCount === 0}
                    onClick={() => void onConfirm()}
                  >
                    {confirming
                      ? "Confirmando…"
                      : `Confirmar inventário (${includedCount})`}
                  </Button>
                </div>
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}
