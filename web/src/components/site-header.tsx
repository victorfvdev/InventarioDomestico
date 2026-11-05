"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const links = [
  { href: "/", label: "Estoque" },
  { href: "/carga-inicial", label: "Carga por foto" },
  { href: "/nfce", label: "NFC-e MT" },
  { href: "/entrada", label: "Entrada manual" },
  { href: "/movimentacoes", label: "Movimentações" },
];

export function SiteHeader({ household }: { household?: string }) {
  const pathname = usePathname();

  return (
    <header className="border-b border-[var(--line)]/70 bg-[var(--paper)]/80 backdrop-blur-md">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-4 sm:flex-row sm:items-end sm:justify-between sm:px-6">
        <div>
          <p className="font-[family-name:var(--font-display)] text-3xl leading-none tracking-tight text-[var(--brand)] sm:text-4xl">
            Estoque
          </p>
          <p className="mt-1 text-sm text-[var(--muted)]">
            {household ? `${household} · inventário doméstico` : "Inventário doméstico com confirmação humana"}
          </p>
        </div>
        <nav className="flex flex-wrap gap-1">
          {links.map((link) => {
            const active = pathname === link.href;
            return (
              <Link
                key={link.href}
                href={link.href}
                className={cn(
                  "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "bg-[var(--brand)] text-[var(--brand-fg)]"
                    : "text-[var(--ink)] hover:bg-[var(--surface)]",
                )}
              >
                {link.label}
              </Link>
            );
          })}
        </nav>
      </div>
    </header>
  );
}
