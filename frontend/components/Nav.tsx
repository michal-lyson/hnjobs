"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export function Nav({ children }: { children?: React.ReactNode }) {
  const pathname = usePathname();

  const link = (href: string, label: string) => (
    <Link
      href={href}
      className={`text-sm transition-colors ${
        pathname === href
          ? "text-orange-400 font-medium"
          : "text-zinc-400 hover:text-zinc-200"
      }`}
    >
      {label}
    </Link>
  );

  return (
    <header className="bg-zinc-900 border-b border-zinc-800">
      <div className="max-w-6xl mx-auto px-4 py-4 flex items-center gap-6">
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-orange-500 text-white font-bold text-sm">
            HN
          </div>
          <div>
            <span className="text-lg font-semibold text-zinc-100">HN Jobs</span>
            <p className="text-xs text-zinc-500">Who is hiring? — aggregated</p>
          </div>
        </div>
        <nav className="flex items-center gap-5">
          {link("/", "Jobs")}
          {link("/trends", "Trends")}
          {link("/about", "About")}
        </nav>
        {children && <div className="ml-auto">{children}</div>}
      </div>
    </header>
  );
}
