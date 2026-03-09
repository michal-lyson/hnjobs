const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface Job {
  id: number;
  hn_item_id: number;
  thread_id: number;
  author: string;
  text: string;
  company: string;
  location: string;
  remote_region: "" | "us" | "eu" | "global";
  salary_min?: number;
  salary_max?: number;
  salary_currency: string;
  posted_at: string;
  created_at: string;
  url: string;
}

export interface JobsResponse {
  jobs: Job[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface JobFilters {
  keywords?: string;
  location?: string;
  remote_region?: "" | "any" | "us" | "eu" | "global";
  salary_min?: number;
  date_from?: string;
  page?: number;
  page_size?: number;
}

export async function fetchJobs(filters: JobFilters): Promise<JobsResponse> {
  const params = new URLSearchParams();
  if (filters.keywords) params.set("keywords", filters.keywords);
  if (filters.location) params.set("location", filters.location);
  if (filters.remote_region) params.set("remote_region", filters.remote_region);
  if (filters.salary_min) params.set("salary_min", String(filters.salary_min));
  if (filters.date_from) params.set("date_from", filters.date_from);
  if (filters.page) params.set("page", String(filters.page));
  if (filters.page_size) params.set("page_size", String(filters.page_size));

  const res = await fetch(`${API_BASE}/api/jobs?${params}`);
  if (!res.ok) throw new Error("Failed to fetch jobs");
  return res.json();
}

export interface TrendPoint {
  month: string;
  count: number;
}

export interface TrendEntry {
  keyword: string;
  total: number;
  points: TrendPoint[];
}

export interface TrendsResponse {
  trends: TrendEntry[];
  months: string[];
}

export async function fetchTrends(): Promise<TrendsResponse> {
  const res = await fetch(`${API_BASE}/api/trends`);
  if (!res.ok) throw new Error("Failed to fetch trends");
  return res.json();
}

export function formatSalary(min?: number, max?: number, currency?: string): string {
  if (!min && !max) return "";
  const fmt = (n: number) => n >= 1000 ? `${Math.round(n / 1000)}K` : `${n}`;
  const suffix = currency ? ` ${currency}` : "";
  if (min && max) return `${fmt(min)} – ${fmt(max)}${suffix}`;
  if (min) return `${fmt(min)}+${suffix}`;
  return `${fmt(max!)}${suffix}`;
}

export function timeAgo(dateStr: string): string {
  const d = new Date(dateStr);
  const diff = Date.now() - d.getTime();
  const days = Math.floor(diff / 86400000);
  if (days === 0) return "today";
  if (days === 1) return "yesterday";
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  return `${months}mo ago`;
}
