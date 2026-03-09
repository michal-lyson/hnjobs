"use client";

import { Search, MapPin, Banknote, Calendar, Wifi } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { JobFilters } from "@/lib/api";

interface Props {
  filters: JobFilters;
  onChange: (f: JobFilters) => void;
}

const SALARY_OPTIONS = [
  { label: "Any salary", value: 0 },
  { label: "50K+", value: 50000 },
  { label: "100K+", value: 100000 },
  { label: "150K+", value: 150000 },
  { label: "200K+", value: 200000 },
];

const DATE_OPTIONS = [
  { label: "Any time", value: "" },
  { label: "Last 7 days", value: daysAgo(7) },
  { label: "Last 30 days", value: daysAgo(30) },
  { label: "Last 90 days", value: daysAgo(90) },
];

function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().split("T")[0];
}

export function FilterBar({ filters, onChange }: Props) {
  const [keywordsInput, setKeywordsInput] = useState(filters.keywords ?? "");
  const [locationInput, setLocationInput] = useState(filters.location ?? "");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const submit = useCallback(
    (partial: Partial<JobFilters>) => {
      onChange({ ...filters, ...partial, page: 1 });
    },
    [filters, onChange]
  );

  // Debounced keyword search — fires 400ms after the user stops typing
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      if (keywordsInput !== (filters.keywords ?? "")) {
        submit({ keywords: keywordsInput || undefined });
      }
    }, 400);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [keywordsInput]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleLocationKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") submit({ location: locationInput });
  };

  return (
    <div className="sticky top-0 z-10 bg-zinc-900/95 backdrop-blur border-b border-zinc-800 py-4">
      <div className="max-w-6xl mx-auto px-4 flex flex-wrap gap-3 items-center">
        {/* Keywords */}
        <div className="relative flex-1 min-w-[200px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400" />
          <input
            type="text"
            placeholder="Keywords, skills, role..."
            value={keywordsInput}
            onChange={(e) => setKeywordsInput(e.target.value)}
            className="w-full pl-9 pr-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-orange-500 transition-colors"
          />
        </div>

        {/* Location */}
        <div className="relative min-w-[160px]">
          <MapPin className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400" />
          <input
            type="text"
            placeholder="Location..."
            value={locationInput}
            onChange={(e) => setLocationInput(e.target.value)}
            onKeyDown={handleLocationKey}
            onBlur={() => submit({ location: locationInput })}
            className="w-full pl-9 pr-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-orange-500 transition-colors"
          />
        </div>

        {/* Remote region */}
        <div className="relative">
          <Wifi className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400 pointer-events-none" />
          <select
            value={filters.remote_region ?? ""}
            onChange={(e) =>
              submit({ remote_region: (e.target.value as JobFilters["remote_region"]) || undefined })
            }
            className={`pl-9 pr-8 py-2 bg-zinc-800 border rounded-lg text-sm focus:outline-none appearance-none cursor-pointer transition-colors ${
              filters.remote_region
                ? "border-orange-500 text-orange-400"
                : "border-zinc-700 text-zinc-400 focus:border-orange-500"
            }`}
          >
            <option value="">All work types</option>
            <option value="any">Remote (any)</option>
            <option value="global">Remote — Global</option>
            <option value="eu">Remote — EU</option>
            <option value="us">Remote — US</option>
          </select>
        </div>

        {/* Salary */}
        <div className="relative">
          <Banknote className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400 pointer-events-none" />
          <select
            value={filters.salary_min ?? 0}
            onChange={(e) =>
              submit({
                salary_min: Number(e.target.value) || undefined,
              })
            }
            className="pl-9 pr-8 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-100 focus:outline-none focus:border-orange-500 appearance-none cursor-pointer transition-colors"
          >
            {SALARY_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </div>

        {/* Date */}
        <div className="relative">
          <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400 pointer-events-none" />
          <select
            value={filters.date_from ?? ""}
            onChange={(e) => submit({ date_from: e.target.value || undefined })}
            className="pl-9 pr-8 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-100 focus:outline-none focus:border-orange-500 appearance-none cursor-pointer transition-colors"
          >
            {DATE_OPTIONS.map((o) => (
              <option key={o.label} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </div>

        {/* Clear */}
        {(filters.keywords ||
          filters.location ||
          filters.remote_region ||
          filters.salary_min ||
          filters.date_from) && (
          <button
            onClick={() =>
              onChange({ page: 1, page_size: filters.page_size })
            }
            className="px-3 py-2 text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
          >
            Clear
          </button>
        )}
      </div>
    </div>
  );
}
