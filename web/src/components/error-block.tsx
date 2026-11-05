import { Button } from "@/components/ui/button";

export function ErrorBlock({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="rounded-md border border-[var(--danger)]/30 bg-[var(--danger-soft)] px-4 py-5 text-[var(--danger)]">
      <p className="font-semibold">Algo deu errado</p>
      <p className="mt-1 text-sm opacity-90">{message}</p>
      {onRetry ? (
        <Button variant="outline" className="mt-4 border-[var(--danger)]/40" onClick={onRetry}>
          Tentar de novo
        </Button>
      ) : null}
    </div>
  );
}
