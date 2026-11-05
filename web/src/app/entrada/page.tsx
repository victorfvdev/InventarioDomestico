"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { SiteHeader } from "@/components/site-header";
import { ErrorBlock } from "@/components/error-block";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, type Session } from "@/lib/api";

export default function EntradaPage() {
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("geral");
  const [unit, setUnit] = useState("un");
  const [quantity, setQuantity] = useState("1");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  useEffect(() => {
    api.session().then(setSession).catch(() => undefined);
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setOk(null);
    try {
      const result = await api.manualEntry({
        description,
        category,
        unit,
        quantity: Number(quantity),
        notes,
      });
      setOk(`${result.product.description} atualizado — saldo ${result.product.quantity} ${result.product.unit}`);
      setDescription("");
      setQuantity("1");
      setNotes("");
      setTimeout(() => router.push("/"), 700);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Falha na entrada");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="min-h-screen pantry-grid">
      <SiteHeader household={session?.household.name} />
      <main className="mx-auto max-w-xl px-4 py-8 sm:px-6">
        <section className="animate-rise mb-8">
          <h1 className="font-[family-name:var(--font-display)] text-4xl text-[var(--brand)]">
            Entrada manual
          </h1>
          <p className="mt-3 text-[var(--muted)]">
            Registra entrada com origem rastreável e marca o saldo como confirmado.
          </p>
        </section>

        <form onSubmit={onSubmit} className="animate-rise-delay space-y-4">
          <div className="space-y-2">
            <Label htmlFor="description">Produto</Label>
            <Input
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Ex.: Café moído 500g"
              required
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="category">Categoria</Label>
              <Input id="category" value={category} onChange={(e) => setCategory(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="unit">Unidade</Label>
              <Input id="unit" value={unit} onChange={(e) => setUnit(e.target.value)} />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="quantity">Quantidade</Label>
            <Input
              id="quantity"
              type="number"
              min="0"
              step="0.001"
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="notes">Observações</Label>
            <Input id="notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
          </div>
          {error ? <ErrorBlock message={error} /> : null}
          {ok ? <p className="text-sm text-[var(--ok)]">{ok}</p> : null}
          <Button type="submit" disabled={saving} size="lg">
            {saving ? "Salvando…" : "Registrar entrada"}
          </Button>
        </form>
      </main>
    </div>
  );
}
