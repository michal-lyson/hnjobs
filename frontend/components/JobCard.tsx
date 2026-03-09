"use client";

import { ExternalLink, MapPin, DollarSign, Clock, Wifi } from "lucide-react";
import { useState } from "react";
import type { Job } from "@/lib/api";
import { formatSalary, timeAgo } from "@/lib/api";

interface Props {
  job: Job;
}

export function JobCard({ job }: Props) {
  const [expanded, setExpanded] = useState(false);
  const salary = formatSalary(job.salary_min, job.salary_max, job.salary_currency);
  const lines = job.text.split("\n");
  const preview = lines.slice(0, 3).join(" ").slice(0, 200);
  const hasMore = job.text.length > 200 || lines.length > 3;

  return (
    <article className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-zinc-700 transition-colors">
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h2 className="font-semibold text-zinc-100 text-base truncate">
              {job.company || job.author}
            </h2>
            {job.remote_region && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-medium">
                <Wifi className="w-3 h-3" />
                {job.remote_region === "global" ? "Remote" : `Remote · ${job.remote_region.toUpperCase()}`}
              </span>
            )}
          </div>

          <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-sm text-zinc-400">
            {job.location && (
              <span className="flex items-center gap-1">
                <MapPin className="w-3.5 h-3.5 shrink-0" />
                {job.location}
              </span>
            )}
            {salary && (
              <span className="flex items-center gap-1 text-emerald-400">
                <DollarSign className="w-3.5 h-3.5 shrink-0" />
                {salary}
              </span>
            )}
            <span className="flex items-center gap-1">
              <Clock className="w-3.5 h-3.5 shrink-0" />
              {timeAgo(job.posted_at)}
            </span>
          </div>
        </div>

        <a
          href={job.url}
          target="_blank"
          rel="noopener noreferrer"
          className="shrink-0 p-2 rounded-lg text-zinc-500 hover:text-orange-400 hover:bg-zinc-800 transition-colors"
          title="View on HN"
        >
          <ExternalLink className="w-4 h-4" />
        </a>
      </div>

      <div className="mt-3 text-sm text-zinc-300 leading-relaxed">
        {expanded ? (
          <pre className="whitespace-pre-wrap font-sans">{job.text}</pre>
        ) : (
          <p>{preview}{hasMore && !expanded ? "…" : ""}</p>
        )}
      </div>

      {hasMore && (
        <button
          onClick={() => setExpanded((v) => !v)}
          className="mt-2 text-xs text-orange-400 hover:text-orange-300 transition-colors"
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </article>
  );
}
