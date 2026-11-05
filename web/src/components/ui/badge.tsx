import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-semibold tracking-wide",
  {
    variants: {
      variant: {
        confirmed: "bg-[var(--ok-soft)] text-[var(--ok)]",
        estimated: "bg-[var(--warn-soft)] text-[var(--warn)]",
        muted: "bg-[var(--surface-strong)] text-[var(--muted)]",
      },
    },
    defaultVariants: { variant: "muted" },
  },
);

export function Badge({
  className,
  variant,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}
