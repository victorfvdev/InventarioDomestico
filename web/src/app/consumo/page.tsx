"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { SiteHeader } from "@/components/site-header";
import { LoadingBlock } from "@/components/loading-block";
import { ErrorBlock } from "@/components/error-block";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  api,
  type ConsumptionLine,
  type ConsumptionShortcut,
  type Movement,
  type Session,
} from "@/lib/api";
import { formatQty } from "@/lib/utils";

type EditableLine = ConsumptionLine & {
  quantityInput: string;
  productChoice: string;
};

function matchLabel(status: ConsumptionLine["matchStatus"]) {
  switch (status) {
    case "exact":
      return "Match exato";
    case "suggested":
      return "Sugestão";
    case "ambiguous":
      return "Ambiguidade";
    default:
      return "Sem produto no estoque";
  }
}

function matchVariant(status: ConsumptionLine["matchStatus"]): "confirmed" | "estimated" | "muted" {
  if (status === "exact") return "confirmed";
  if (status === "ambiguous" || status === "suggested") return "estimated";
  return "muted";
}

const EXAMPLES = [
  "Comi duas bananas.",
  "Fiz três ovos mexidos.",
  "Utilizei uma caixa de creme de leite.",
  "Usei aproximadamente 200 ml de leite.",
];

export default function ConsumoPage() {
  const [session, setSession] = useState<Session | null>(null);
  const [text, setText] = useState("");
  const [loadingSession, setLoadingSession] = useState(true);
  const [interpreting, setInterpreting] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [interpretationId, setInterpretationId] = useState<string | null>(null);
  const [mocked, setMocked] = useState(false);
  const [provider, setProvider] = useState("");
  const [lines, setLines] = useState<EditableLine[]>([]);
  const [shortcuts, setShortcuts] = useState<ConsumptionShortcut[]>([]);
  const [loadingShortcuts, setLoadingShortcuts] = useState(true);
  const [pendingShortcut, setPendingShortcut] = useState<ConsumptionShortcut | null>(null);
  const [quickBusy, setQuickBusy] = useState(false);
  const [recent, setRecent] = useState<Movement[]>([]);
  const [doneMessage, setDoneMessage] = useState<string | null>(null);

  const loadShortcutsAndRecent = useCallback(async () => {
    setLoadingShortcuts(true);
    try {
      const [s, m] = await Promise.all([
        api.consumptionShortcuts(),
        api.movements(12),
      ]);
      setShortcuts(s.shortcuts);
      setRecent(m.movements.filter((x) => x.isConsumption && !x.notes?.includes("[desfeito]")));
    } catch {
      // empty/error handled by UI blocks when needed
    } finally {
      setLoadingShortcuts(false);
    }
  }, []);

  useEffect(() => {
    setLoadingSession(true);
    api
      .session()
      .then(setSession)
      .catch(() => setError("Não foi possível carregar a sessão"))
      .finally(() => setLoadingSession(false));
    void loadShortcutsAndRecent();
  }, [loadShortcutsAndRecent]);

  const includedCount = useMemo(() => lines.filter((l) => l.included).length, [lines]);

  async function onInterpret(e: FormEvent) {
    e.preventDefault();
    if (!text.trim()) {
      setError("Escreva o que foi consumido, em português.");
      return;
    }
    setInterpreting(true);
    setError(null);
    setDoneMessage(null);
    setLines([]);
    setInterpretationId(null);
    try {
      const result = await api.interpretConsumption(text.trim());
      setInterpretationId(result.interpretation.id);
      setMocked(result.mocked);
      setProvider(result.provider);
      setLines(
        result.interpretation.lines.map((line) => {
          let productChoice = "";
          if (line.matchStatus === "ambiguous") {
            productChoice = "";
          } else if (line.matchedProductId) {
            productChoice = line.matchedProductId;
          }
          return {
            ...line,
            quantityInput: String(line.quantity),
            productChoice,
            included: line.included && !line.unmatched && line.matchStatus !== "ambiguous",
          };
        }),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao interpretar");
    } finally {
      setInterpreting(false);
    }
  }

  function updateLine(tempId: string, patch: Partial<EditableLine>) {
    setLines((prev) => prev.map((line) => (line.tempId === tempId ? { ...line, ...patch } : line)));
  }

  async function onConfirm(e: FormEvent) {
    e.preventDefault();
    if (!interpretationId) return;
    const payload = lines
      .filter((l) => l.included)
      .map((l) => ({
        tempId: l.tempId,
        included: true,
        productId: l.productChoice || l.matchedProductId || "",
        quantity: Number(l.quantityInput),
        unit: l.unit,
        notes: l.notes,
      }));
    if (payload.some((p) => !p.productId)) {
      setError("Resolva ambiguidades ou desmarque itens sem produto no estoque.");
      return;
    }
    if (payload.length === 0) {
      setError("Inclua ao menos um item para confirmar a baixa.");
      return;
    }
    setConfirming(true);
    setError(null);
    try {
      const result = await api.confirmConsumption({
        interpretationId,
        items: payload,
      });
      setDoneMessage(result.message);
      setRecent((prev) => [...result.movements, ...prev].slice(0, 12));
      setLines([]);
      setInterpretationId(null);
      setText("");
      await loadShortcutsAndRecent();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao confirmar consumo");
    } finally {
      setConfirming(false);
    }
  }

  async function confirmShortcut() {
    if (!pendingShortcut) return;
    setQuickBusy(true);
    setError(null);
    setDoneMessage(null);
    try {
      const result = await api.quickConsume({
        productId: pendingShortcut.productId,
        quantity: Number(pendingShortcut.quantity),
        unit: pendingShortcut.unit,
        confirm: true,
        notes: `Atalho: ${pendingShortcut.label}`,
      });
      setDoneMessage(result.message);
      setRecent((prev) => [result.movement, ...prev].slice(0, 12));
      setPendingShortcut(null);
      await loadShortcutsAndRecent();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha no atalho");
    } finally {
      setQuickBusy(false);
    }
  }

  async function undoMovement(id: string) {
    setError(null);
    try {
      const result = await api.undoConsumption(id);
      setDoneMessage(result.message);
      setRecent((prev) => prev.filter((m) => m.id !== id));
      await loadShortcutsAndRecent();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao desfazer");
    }
  }

  return (
    <div className="min-h-screen pantry-grid">
      <SiteHeader household={session?.household.name} />
      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        <section className="animate-rise mb-8 max-w-2xl">
          <h1 className="font-[family-name:var(--font-display)] text-4xl leading-tight text-[var(--brand)] sm:text-5xl">
            Consumo
          </h1>
          <p className="mt-3 text-base text-[var(--muted)] sm:text-lg">
            Descreva o que saiu da despensa em português. O parser local só sugere — a baixa só
            acontece depois da sua confirmação.
          </p>
        </section>

        {loadingSession ? <LoadingBlock label="Abrindo sessão…" /> : null}
        {error ? <ErrorBlock message={error} onRetry={() => setError(null)} /> : null}
        {doneMessage ? (
          <p className="animate-rise mb-6 rounded-md border border-[var(--ok)]/30 bg-[var(--ok-soft)] px-4 py-3 text-sm text-[var(--ok)]">
            {doneMessage}
          </p>
        ) : null}

        <section className="animate-rise-delay mb-10">
          <div className="mb-3 flex items-end justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold">Atalhos rápidos</h2>
              <p className="text-sm text-[var(--muted)]">
                Histórico de consumo ou sementes dos itens que já estão no estoque
              </p>
            </div>
          </div>
          {loadingShortcuts ? <LoadingBlock label="Carregando atalhos…" /> : null}
          {!loadingShortcuts && shortcuts.length === 0 ? (
            <EmptyState
              title="Sem atalhos ainda"
              description="Cadastre produtos (foto, NFC-e ou entrada manual) para liberar atalhos de consumo. Nada é inventado fora do estoque."
              actionHref="/"
              actionLabel="Ver estoque"
            />
          ) : null}
          {!loadingShortcuts && shortcuts.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {shortcuts.map((s) => (
                <Button
                  key={s.id}
                  type="button"
                  variant={s.source === "history" ? "default" : "outline"}
                  onClick={() => {
                    setPendingShortcut(s);
                    setDoneMessage(null);
                    setError(null);
                  }}
                >
                  {s.label}
                  <span className="ml-2 text-xs opacity-70">
                    {s.source === "history" ? "histórico" : "semente"}
                  </span>
                </Button>
              ))}
            </div>
          ) : null}

          {pendingShortcut ? (
            <div className="mt-4 max-w-lg space-y-3 border-t border-[var(--line)] pt-4">
              <h3 className="font-semibold">Confirmar atalho</h3>
              <p className="text-sm text-[var(--muted)]">
                {pendingShortcut.label} · disponível {formatQty(pendingShortcut.available)}{" "}
                {pendingShortcut.unit}. A baixa só roda se você confirmar.
              </p>
              <div className="flex flex-wrap gap-2">
                <Button type="button" disabled={quickBusy} onClick={() => void confirmShortcut()}>
                  {quickBusy ? "Registrando…" : "Confirmar consumo"}
                </Button>
                <Button type="button" variant="ghost" onClick={() => setPendingShortcut(null)}>
                  Cancelar
                </Button>
              </div>
            </div>
          ) : null}
        </section>

        <section className="mb-10">
          <h2 className="text-lg font-semibold">Baixa por texto</h2>
          <p className="mt-1 text-sm text-[var(--muted)]">
            Exemplos: {EXAMPLES.slice(0, 2).join(" · ")}
          </p>
          <form onSubmit={onInterpret} className="mt-4 max-w-2xl space-y-3">
            <div className="space-y-2">
              <Label htmlFor="consumo-text">O que foi consumido?</Label>
              <textarea
                id="consumo-text"
                value={text}
                onChange={(e) => setText(e.target.value)}
                rows={3}
                placeholder="Ex.: Comi duas bananas e usei 200 ml de leite"
                className="w-full rounded-md border border-[var(--line)] bg-white/70 px-3 py-2 text-sm outline-none ring-[var(--ring)] focus:ring-2"
              />
            </div>
            <div className="flex flex-wrap gap-2">
              {EXAMPLES.map((ex) => (
                <button
                  key={ex}
                  type="button"
                  className="rounded-md border border-[var(--line)] bg-white/50 px-2 py-1 text-xs text-[var(--muted)] transition hover:border-[var(--brand)] hover:text-[var(--brand)]"
                  onClick={() => setText(ex)}
                >
                  {ex}
                </button>
              ))}
            </div>
            <Button type="submit" disabled={interpreting}>
              {interpreting ? "Interpretando…" : "Interpretar"}
            </Button>
          </form>
        </section>

        {interpreting ? <LoadingBlock label="Parser local analisando o texto…" /> : null}

        {interpretationId && lines.length === 0 ? (
          <EmptyState
            title="Nada reconhecido"
            description="Não encontramos menções de produto no texto. Tente algo como “comi duas bananas”."
          />
        ) : null}

        {lines.length > 0 ? (
          <form onSubmit={onConfirm} className="animate-rise space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-semibold">Revisão</h2>
              {mocked ? <Badge variant="muted">mock local · {provider || "parser"}</Badge> : null}
              <span className="text-sm text-[var(--muted)]">{includedCount} item(ns) incluído(s)</span>
            </div>
            <p className="text-sm text-[var(--muted)]">
              Itens sem correspondência no estoque ficam de fora — o sistema não inventa produtos.
            </p>
            <div className="space-y-3">
              {lines.map((line) => (
                <div
                  key={line.tempId}
                  className="rounded-md border border-[var(--line)] bg-white/60 p-4"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium text-[var(--ink)]">
                        {line.matchedProductName || line.productHint}
                      </p>
                      <p className="text-sm text-[var(--muted)]">“{line.mentionedText}”</p>
                      <div className="mt-2 flex flex-wrap gap-2">
                        <Badge variant={matchVariant(line.matchStatus)}>
                          {matchLabel(line.matchStatus)}
                        </Badge>
                        {line.quantityInferred ? <Badge variant="muted">qtd sugerida</Badge> : null}
                        {line.approx ? <Badge variant="muted">aprox.</Badge> : null}
                        {line.insufficientStock ? (
                          <Badge variant="estimated">estoque insuficiente</Badge>
                        ) : null}
                      </div>
                      {line.notes ? (
                        <p className="mt-2 text-xs text-[var(--muted)]">{line.notes}</p>
                      ) : null}
                    </div>
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={line.included}
                        disabled={line.unmatched && !line.productChoice}
                        onChange={(e) => updateLine(line.tempId, { included: e.target.checked })}
                      />
                      Incluir
                    </label>
                  </div>
                  <div className="mt-3 grid gap-3 sm:grid-cols-2">
                    <div className="space-y-1">
                      <Label>Quantidade ({line.unit})</Label>
                      <Input
                        type="number"
                        min="0.001"
                        step="0.001"
                        value={line.quantityInput}
                        onChange={(e) => updateLine(line.tempId, { quantityInput: e.target.value })}
                        disabled={!line.included}
                      />
                      {!line.unmatched ? (
                        <p className="text-xs text-[var(--muted)]">
                          Disponível: {formatQty(line.availableQuantity)} {line.unit}
                        </p>
                      ) : null}
                    </div>
                    {(line.matchStatus === "ambiguous" || (line.candidates?.length ?? 0) > 1) && (
                      <div className="space-y-1">
                        <Label>Produto no estoque</Label>
                        <select
                          className="w-full rounded-md border border-[var(--line)] bg-white px-3 py-2 text-sm"
                          value={line.productChoice}
                          onChange={(e) =>
                            updateLine(line.tempId, {
                              productChoice: e.target.value,
                              included: Boolean(e.target.value),
                            })
                          }
                        >
                          <option value="">Escolher…</option>
                          {(line.candidates ?? []).map((c) => (
                            <option key={c.productId} value={c.productId}>
                              {c.description} ({Math.round(c.score * 100)}%)
                            </option>
                          ))}
                        </select>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button type="submit" disabled={confirming || includedCount === 0}>
                {confirming ? "Registrando…" : "Confirmar baixa"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setLines([]);
                  setInterpretationId(null);
                }}
              >
                Descartar
              </Button>
            </div>
          </form>
        ) : null}

        <section className="mt-12">
          <div className="mb-3 flex items-end justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold">Consumos recentes</h2>
              <p className="text-sm text-[var(--muted)]">
                Desfazer gera ajuste de inventário — não conta como novo consumo
              </p>
            </div>
            <Button asChild variant="ghost" size="sm">
              <Link href="/movimentacoes">Ver tudo</Link>
            </Button>
          </div>
          {recent.length === 0 ? (
            <p className="text-sm text-[var(--muted)]">Nenhum consumo recente para desfazer.</p>
          ) : (
            <ul className="divide-y divide-[var(--line)]/70 rounded-md border border-[var(--line)] bg-white/55">
              {recent.map((m) => (
                <li key={m.id} className="flex flex-wrap items-center justify-between gap-3 px-4 py-3 text-sm">
                  <div>
                    <p className="font-medium">{m.productName || m.productId}</p>
                    <p className="text-[var(--muted)]">
                      {formatQty(m.quantityDelta)} · saldo {formatQty(m.balanceAfter)}
                    </p>
                  </div>
                  <Button type="button" variant="outline" size="sm" onClick={() => void undoMovement(m.id)}>
                    Desfazer
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </section>
      </main>
    </div>
  );
}
