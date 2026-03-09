"use client";

import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { Loader2, AlertCircle } from "lucide-react";
import { Nav } from "@/components/Nav";
import { fetchTrends } from "@/lib/api";
import type { TrendEntry } from "@/lib/api";

const COLORS = [
  "#f97316", "#3b82f6", "#22c55e", "#a855f7", "#ec4899",
  "#eab308", "#06b6d4", "#f43f5e", "#84cc16", "#8b5cf6",
  "#14b8a6", "#f59e0b", "#6366f1", "#10b981", "#ef4444",
  "#0ea5e9", "#d946ef", "#64748b", "#fb923c", "#34d399",
];

interface Tooltip {
  x: number;
  y: number;
  keyword: string;
  color: string;
  month: string;
  count: number;
}

interface ChartProps {
  trends: TrendEntry[];
  months: string[];
  hovered: string | null;
  selected: Set<string>;
  onHover: (kw: string | null) => void;
  onSelect: (kw: string) => void;
}

function TrendsChart({ trends, months, hovered, selected, onHover, onSelect }: ChartProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [tooltip, setTooltip] = useState<Tooltip | null>(null);

  if (months.length === 0) return null;

  const W = 800, H = 280;
  const PL = 40, PR = 16, PT = 16, PB = 48;
  const chartW = W - PL - PR;
  const chartH = H - PT - PB;

  const maxCount = Math.max(1, ...trends.flatMap((t) => t.points.map((p) => p.count)));
  const getX = (i: number) =>
    months.length === 1 ? PL + chartW / 2 : PL + (i / (months.length - 1)) * chartW;
  const getY = (count: number) => PT + chartH - (count / maxCount) * chartH;

  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => ({
    y: PT + chartH - f * chartH,
    label: Math.round(f * maxCount),
  }));

  const isActive = (kw: string) => {
    if (hovered) return hovered === kw;
    if (selected.size > 0) return selected.has(kw);
    return true;
  };
  const lineOpacity = (kw: string) =>
    !hovered && selected.size === 0 ? 1 : isActive(kw) ? 1 : 0.07;
  const lineWidth = (kw: string) => (isActive(kw) && (hovered === kw || selected.has(kw)) ? 3 : 1.5);

  const handleMouseMove = (
    e: React.MouseEvent<SVGPolylineElement>,
    trend: TrendEntry,
    color: string,
    byMonth: Record<string, number>
  ) => {
    const svg = svgRef.current;
    if (!svg) return;
    const rect = svg.getBoundingClientRect();
    const relX = ((e.clientX - rect.left) / rect.width) * W;
    const idx = Math.max(
      0,
      Math.min(months.length - 1, Math.round(((relX - PL) / chartW) * (months.length - 1)))
    );
    setTooltip({
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
      keyword: trend.keyword,
      color,
      month: months[idx],
      count: byMonth[months[idx]] ?? 0,
    });
  };

  return (
    <div className="relative">
      {tooltip && (
        <div
          className="absolute z-10 pointer-events-none bg-zinc-800 border border-zinc-700 rounded-lg px-3 py-2 text-xs shadow-xl"
          style={{
            left: tooltip.x + 14,
            top: Math.max(0, tooltip.y - 36),
            transform: tooltip.x > 600 ? "translateX(-110%)" : undefined,
          }}
        >
          <p className="font-semibold mb-0.5" style={{ color: tooltip.color }}>
            {tooltip.keyword}
          </p>
          <p className="text-zinc-400">
            {tooltip.month}:{" "}
            <span className="text-zinc-100 font-medium">{tooltip.count}</span>
          </p>
        </div>
      )}

      <svg
        ref={svgRef}
        viewBox={`0 0 ${W} ${H}`}
        className="w-full"
        style={{ height: 280 }}
        onMouseLeave={() => {
          setTooltip(null);
          onHover(null);
        }}
      >
        {/* Grid */}
        {yTicks.map((t) => (
          <g key={t.label}>
            <line x1={PL} y1={t.y} x2={W - PR} y2={t.y} stroke="#3f3f46" strokeWidth="1" />
            <text x={PL - 6} y={t.y + 4} textAnchor="end" fontSize="10" fill="#71717a">
              {t.label}
            </text>
          </g>
        ))}

        {/* Lines — inactive first so active renders on top */}
        {[...trends].reverse().map((trend, ri) => {
          const ti = trends.length - 1 - ri;
          const color = COLORS[ti % COLORS.length];
          const byMonth = Object.fromEntries(trend.points.map((p) => [p.month, p.count]));
          const pts = months.map((m, i) => `${getX(i)},${getY(byMonth[m] ?? 0)}`).join(" ");
          const active = isActive(trend.keyword);

          return (
            <g
              key={trend.keyword}
              style={{ opacity: lineOpacity(trend.keyword), transition: "opacity 0.12s" }}
            >
              {/* Visible line */}
              <polyline
                points={pts}
                fill="none"
                stroke={color}
                strokeWidth={lineWidth(trend.keyword)}
                strokeLinejoin="round"
                strokeLinecap="round"
                style={{ transition: "stroke-width 0.12s" }}
              />
              {/* Dot at each data point when active */}
              {active &&
                months.map((m, i) => {
                  const count = byMonth[m] ?? 0;
                  if (count === 0) return null;
                  return (
                    <circle
                      key={m}
                      cx={getX(i)}
                      cy={getY(count)}
                      r={hovered === trend.keyword || selected.has(trend.keyword) ? 3.5 : 0}
                      fill={color}
                      style={{ transition: "r 0.12s" }}
                    />
                  );
                })}
              {/* Wide invisible hit area */}
              <polyline
                points={pts}
                fill="none"
                stroke="transparent"
                strokeWidth="16"
                className="cursor-pointer"
                onMouseEnter={() => onHover(trend.keyword)}
                onMouseMove={(e) => handleMouseMove(e, trend, color, byMonth)}
                onClick={() => onSelect(trend.keyword)}
              />
            </g>
          );
        })}

        {/* X axis labels — show at most 8 to avoid overlap */}
        {(() => {
          const step = Math.ceil(months.length / 8);
          return months.map((m, i) => {
            if (i % step !== 0 && i !== months.length - 1) return null;
            return (
              <text key={m} x={getX(i)} y={H - 8} textAnchor="middle" fontSize="10" fill="#71717a">
                {m.replace(/(\w{3})\w* (\d{4})/, "$1 $2")}
              </text>
            );
          });
        })()}
      </svg>
    </div>
  );
}

export default function TrendsPage() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["trends"],
    queryFn: fetchTrends,
    staleTime: 5 * 60 * 1000,
  });

  const [hovered, setHovered] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const toggleSelected = (kw: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(kw) ? next.delete(kw) : next.add(kw);
      return next;
    });
  };

  const maxTotal = data?.trends[0]?.total ?? 1;

  const isActive = (kw: string) => {
    if (hovered) return hovered === kw;
    if (selected.size > 0) return selected.has(kw);
    return true;
  };

  return (
    <div className="min-h-screen flex flex-col">
      <Nav />

      <main className="flex-1 max-w-6xl mx-auto w-full px-4 py-8">
        <div className="mb-8">
          <h2 className="text-2xl font-bold text-zinc-100">Job Trends</h2>
          <p className="text-sm text-zinc-500 mt-1">
            20 most mentioned technologies across all "Who is Hiring?" threads
          </p>
        </div>

        {isLoading && (
          <div className="flex items-center justify-center py-24 gap-3 text-zinc-500">
            <Loader2 className="w-6 h-6 animate-spin" />
            <span className="text-sm">Crunching the numbers…</span>
          </div>
        )}

        {isError && (
          <div className="flex items-center justify-center py-24 gap-3 text-zinc-500">
            <AlertCircle className="w-6 h-6 text-red-400" />
            <span className="text-sm text-red-400">Failed to load trends.</span>
          </div>
        )}

        {data && (
          <>
            {/* Line chart */}
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 mb-8">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-medium text-zinc-400">Mentions over time</h3>
                {selected.size > 0 && (
                  <button
                    onClick={() => setSelected(new Set())}
                    className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
                  >
                    Clear selection
                  </button>
                )}
              </div>

              <TrendsChart
                trends={data.trends}
                months={data.months}
                hovered={hovered}
                selected={selected}
                onHover={setHovered}
                onSelect={toggleSelected}
              />

              {/* Legend */}
              <div className="flex flex-wrap gap-x-4 gap-y-2 mt-5">
                {data.trends.map((t, i) => {
                  const color = COLORS[i % COLORS.length];
                  const active = isActive(t.keyword);
                  const pinned = selected.has(t.keyword);
                  return (
                    <button
                      key={t.keyword}
                      onClick={() => toggleSelected(t.keyword)}
                      onMouseEnter={() => setHovered(t.keyword)}
                      onMouseLeave={() => setHovered(null)}
                      className="flex items-center gap-1.5 text-xs transition-opacity"
                      style={{ opacity: active ? 1 : 0.35 }}
                    >
                      <span
                        className="inline-block w-4 rounded"
                        style={{
                          height: pinned ? 3 : 2,
                          backgroundColor: color,
                          transition: "height 0.12s",
                        }}
                      />
                      <span style={{ color: active ? color : "#71717a", transition: "color 0.12s" }}>
                        {t.keyword}
                      </span>
                    </button>
                  );
                })}
              </div>
              <p className="text-xs text-zinc-600 mt-3">
                Hover to highlight · Click to pin
              </p>
            </div>

            {/* Bar chart */}
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
              <h3 className="text-sm font-medium text-zinc-400 mb-4">Overall ranking</h3>
              <div className="flex flex-col gap-2.5">
                {data.trends.map((t, i) => {
                  const color = COLORS[i % COLORS.length];
                  const active = isActive(t.keyword);
                  const pinned = selected.has(t.keyword);
                  return (
                    <div
                      key={t.keyword}
                      className="flex items-center gap-3 cursor-pointer group"
                      style={{ opacity: active ? 1 : 0.3, transition: "opacity 0.12s" }}
                      onMouseEnter={() => setHovered(t.keyword)}
                      onMouseLeave={() => setHovered(null)}
                      onClick={() => toggleSelected(t.keyword)}
                    >
                      <span className="w-5 text-xs text-zinc-600 text-right shrink-0">{i + 1}</span>
                      <span
                        className="w-28 text-sm shrink-0 truncate transition-colors"
                        style={{ color: active ? "#e4e4e7" : "#71717a" }}
                      >
                        {t.keyword}
                      </span>
                      <div className="flex-1 h-5 bg-zinc-800 rounded overflow-hidden">
                        <div
                          className="h-full rounded transition-all duration-300"
                          style={{
                            width: `${(t.total / maxTotal) * 100}%`,
                            backgroundColor: color,
                            opacity: pinned ? 1 : 0.75,
                          }}
                        />
                      </div>
                      <span className="w-12 text-xs text-zinc-500 text-right shrink-0">
                        {t.total.toLocaleString()}
                      </span>
                    </div>
                  );
                })}
              </div>
              <p className="text-xs text-zinc-600 mt-4">Hover to highlight · Click to pin</p>
            </div>
          </>
        )}
      </main>
    </div>
  );
}
