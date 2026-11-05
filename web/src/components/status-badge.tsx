import { Badge } from "@/components/ui/badge";
import type { ConfirmationStatus } from "@/lib/api";

export function StatusBadge({ status }: { status: ConfirmationStatus }) {
  if (status === "confirmed") {
    return <Badge variant="confirmed">Confirmado</Badge>;
  }
  return <Badge variant="estimated">Estimado</Badge>;
}
