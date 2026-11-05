"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { SiteHeader } from "@/components/site-header";
import { EmptyState } from "@/components/empty-state";
import { LoadingBlock } from "@/components/loading-block";
import { ErrorBlock } from "@/components/error-block";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, type Product, type Session } from "@/lib/api";
import { formatQty } from "@/lib/utils";

export default function HomePage() {
  const [session, setSession] = useState<Session | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [adjusting, setAdjusting] = useState<Product | null>(null);
  const [newQty, setNewQty] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [s, p] = await Promise.all([api.session(), api.products()]);
      setSession(s);
      setProducts(p.products);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao carregar estoque");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onAdjust(e: FormEvent) {
    e.preventDefault();
    if (!adjusting) return;
    setSaving(true);
    try {
      await api.adjust(adjusting.id, Number(newQty), "Ajuste de inventário (não é consumo)");
      setAdjusting(null);
      setNewQty("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha no ajuste");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="min-h-screen pantry-grid">
      <SiteHeader household={session?.household.name} />
      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        <section className="animate-rise mb-8 max-w-2xl">
          <h1 className="font-[family-name:var(--font-display)] text-4xl leading-tight text-[var(--brand)] sm:text-5xl">
            Estoque
          </h1>
          <p className="mt-3 text-base text-[var(--muted)] sm:text-lg">
            Saldos confiáveis: a IA só sugere; você confirma. Estimado e confirmado ficam sempre visíveis.
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button asChild size="lg">
              <Link href="/carga-inicial">Fotografar despensa</Link>
            </Button>
            <Button asChild variant="outline" size="lg">
              <Link href="/nfce">Entrada por NFC-e</Link>
            </Button>
            <Button asChild variant="ghost" size="lg">
              <Link href="/entrada">Entrada manual</Link>
            </Button>
          </div>
        </section>

        <section className="animate-rise-delay">
          <div className="mb-4 flex items-end justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold">Produtos</h2>
              <p className="text-sm text-[var(--muted)]">Quantidades atuais da residência</p>
            </div>
            <Button variant="ghost" onClick={() => void load()}>
              Atualizar
            </Button>
          </div>

          {loading ? <LoadingBlock label="Carregando produtos…" /> : null}
          {error ? <ErrorBlock message={error} onRetry={() => void load()} /> : null}

          {!loading && !error && products.length === 0 ? (
            <EmptyState
              title="Nada na despensa ainda"
              description="Comece fotografando a prateleira ou registrando uma entrada manual. Nada entra no saldo sem a sua confirmação."
              actionHref="/carga-inicial"
              actionLabel="Iniciar carga por foto"
            />
          ) : null}

          {!loading && !error && products.length > 0 ? (
            <div className="overflow-x-auto rounded-md border border-[var(--line)] bg-white/55">
              <table className="min-w-full text-left text-sm">
                <thead className="border-b border-[var(--line)] bg-[var(--surface)]/70 text-[var(--muted)]">
                  <tr>
                    <th className="px-4 py-3 font-medium">Produto</th>
                    <th className="px-4 py-3 font-medium">Categoria</th>
                    <th className="px-4 py-3 font-medium">Qtd</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium">Ações</th>
                  </tr>
                </thead>
                <tbody>
                  {products.map((p) => (
                    <tr key={p.id} className="border-b border-[var(--line)]/60 last:border-0">
                      <td className="px-4 py-3 font-medium text-[var(--ink)]">{p.description}</td>
                      <td className="px-4 py-3 text-[var(--muted)]">{p.category}</td>
                      <td className="px-4 py-3 tabular-nums">
                        {formatQty(p.quantity)} {p.unit}
                      </td>
                      <td className="px-4 py-3">
                        <StatusBadge status={p.confirmationStatus} />
                      </td>
                      <td className="px-4 py-3">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setAdjusting(p);
                            setNewQty(String(p.quantity));
                          }}
                        >
                          Ajustar
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}

          {adjusting ? (
            <form
              onSubmit={onAdjust}
              className="mt-6 max-w-md space-y-3 border-t border-[var(--line)] pt-6"
            >
              <h3 className="font-semibold">Ajuste de inventário — {adjusting.description}</h3>
              <p className="text-sm text-[var(--muted)]">
                Este ajuste atualiza o saldo e não é classificado como consumo.
              </p>
              <div className="space-y-2">
                <Label htmlFor="qty">Nova quantidade ({adjusting.unit})</Label>
                <Input
                  id="qty"
                  type="number"
                  min="0"
                  step="0.001"
                  value={newQty}
                  onChange={(e) => setNewQty(e.target.value)}
                  required
                />
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={saving}>
                  {saving ? "Salvando…" : "Confirmar ajuste"}
                </Button>
                <Button type="button" variant="ghost" onClick={() => setAdjusting(null)}>
                  Cancelar
                </Button>
              </div>
            </form>
          ) : null}
        </section>
      </main>
    </div>
  );
}
