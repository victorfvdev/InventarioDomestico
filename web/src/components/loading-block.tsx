export function LoadingBlock({ label = "Carregando…" }: { label?: string }) {
  return (
    <div className="flex items-center gap-3 py-10 text-[var(--muted)]" role="status">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--brand)] border-r-transparent" />
      <span>{label}</span>
    </div>
  );
}
