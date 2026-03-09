"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Loader2, AlertCircle, Briefcase } from "lucide-react";
import { FilterBar } from "@/components/FilterBar";
import { JobCard } from "@/components/JobCard";
import { Nav } from "@/components/Nav";
import { Pagination } from "@/components/Pagination";
import { fetchJobs } from "@/lib/api";
import type { JobFilters } from "@/lib/api";

const DEFAULT_FILTERS: JobFilters = { page: 1, page_size: 20 };

export default function Home() {
  const [filters, setFilters] = useState<JobFilters>(DEFAULT_FILTERS);

  const { data, isLoading, isError, isFetching } = useQuery({
    queryKey: ["jobs", filters],
    queryFn: () => fetchJobs(filters),
  });

  return (
    <div className="min-h-screen flex flex-col">
      <Nav>
        {isFetching && <Loader2 className="w-4 h-4 text-zinc-500 animate-spin" />}
      </Nav>

      {/* Filters */}
      <FilterBar filters={filters} onChange={setFilters} />

      {/* Content */}
      <main className="flex-1 max-w-6xl mx-auto w-full px-4 py-6">
        {/* Stats */}
        {data && (
          <p className="text-sm text-zinc-500 mb-4">
            {data.total.toLocaleString()} job{data.total !== 1 ? "s" : ""} found
            {data.total_pages > 1 && ` · page ${data.page} of ${data.total_pages}`}
          </p>
        )}

        {isLoading && (
          <div className="flex flex-col items-center justify-center py-24 gap-3 text-zinc-500">
            <Loader2 className="w-8 h-8 animate-spin" />
            <p className="text-sm">Loading jobs…</p>
          </div>
        )}

        {isError && (
          <div className="flex flex-col items-center justify-center py-24 gap-3 text-zinc-500">
            <AlertCircle className="w-8 h-8 text-red-400" />
            <p className="text-sm text-red-400">Failed to load jobs. Is the backend running?</p>
          </div>
        )}

        {data && data.jobs?.length === 0 && (
          <div className="flex flex-col items-center justify-center py-24 gap-3 text-zinc-500">
            <Briefcase className="w-8 h-8" />
            <p className="text-sm">No jobs match your filters.</p>
          </div>
        )}

        {data?.jobs && data.jobs.length > 0 && (
          <div className="flex flex-col gap-4">
            {data.jobs.map((job) => (
              <JobCard key={job.id} job={job} />
            ))}
          </div>
        )}

        {data && (
          <Pagination
            page={data.page}
            totalPages={data.total_pages}
            onPage={(p) => {
              setFilters((f) => ({ ...f, page: p }));
              window.scrollTo({ top: 0, behavior: "smooth" });
            }}
          />
        )}
      </main>
    </div>
  );
}
