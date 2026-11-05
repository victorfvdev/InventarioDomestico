"use client";

import { useCallback, useEffect, useState } from "react";
import { SiteHeader } from "@/components/site-header";
import { EmptyState } from "@/components/empty-state";
import { LoadingBlock } from "@/components/loading-block";
import { ErrorBlock } from "@/components/error-block";
import { Badge } from "@/components/ui/badge";
import { api, type Movement, type Session } from "@/lib/api";
import { formatQty } from "@/lib/utils";

const typeLabel: Record<string, string> = {
  manual_entry: "Entrada manual",
  inventory_adjust: "Ajuste de inventário",
  initial_load: "Carga inicial",
  purchase_entry: "Compra",
  consumption: "Consumo",
  loss: "Perda",
};

export default function MovimentacoesPage() {
  const [session, setSession] = useState<Session | null>(null);
  const [movements, setMovements] = useState<Movement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [s, m] = await Promise.all([api.session(), api.movements(40)]);
      setSession(s);
      setMovements(m.movements);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha ao carregar histórico");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="min-h-screen pantry-grid">
      <SiteHeader household={session?.household.name} />
      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        <section className="animate-rise mb-8 max-w-2xl">
          <h1 className="font-[family-name:var(--font-display)] text-4xl text-[var(--brand)]">
            Movimentações
          </h1>
          <p className="mt-3 text-[var(--muted)]">
            Histórico imutável. Ajustes de inventário aparecem com flag explícita de que não são consumo.
          </p>
        </section>

        {loading ? <LoadingBlock /> : null}
        {error ? <ErrorBlock message={error} onRetry={() => void load()} /> : null}

        {!loading && !error && movements.length === 0 ? (
          <EmptyState
            title="Sem movimentações"
            description="Quando você confirmar a carga inicial ou registrar entradas, o histórico aparece aqui."
            actionHref="/carga-inicial"
            actionLabel="Fazer carga inicial"
          />
        ) : null}

        {!loading && !error && movements.length > 0 ? (
          <ul className="animate-rise-delay divide-y divide-[var(--line)] rounded-md border border-[var(--line)] bg-white/55">
            {movements.map((m) => (
              <li key={m.id} className="flex flex-col gap-2 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="font-medium">{m.productName || m.productId}</p>
                  <p className="text-sm text-[var(--muted)]">
                    {typeLabel[m.movementType] || m.movementType} · origem {m.origin}
                  </p>
                  {m.notes ? <p className="mt-1 text-sm text-[var(--muted)]">{m.notes}</p> : null}
                </div>
                <div className="flex flex-wrap items-center gap-2 sm:justify-end">
                  <span className="tabular-nums font-semibold">
                    {Number(m.quantityDelta) > 0 ? "+" : ""}
                    {formatQty(m.quantityDelta)} → {formatQty(m.balanceAfter)}
                  </span>
                  <Badge variant={m.isConsumption ? "estimated" : "muted"}>
                    {m.isConsumption ? "consumo" : "não é consumo"}
                  </Badge>
                </div>
              </li>
            ))}
          </ul>
        ) : null}
      </main>
    </div>
  );
}
