import { Nav } from "@/components/Nav";
import { ExternalLink } from "lucide-react";

export const metadata = {
  title: "About — HN Jobs",
};

export default function AboutPage() {
  return (
    <div className="min-h-screen flex flex-col">
      <Nav />

      <main className="flex-1 max-w-2xl mx-auto w-full px-4 py-12">
        <h2 className="text-2xl font-bold text-zinc-100 mb-2">About HN Jobs</h2>
        <p className="text-sm text-zinc-500 mb-10">Why this exists and how it works</p>

        <div className="flex flex-col gap-8 text-sm text-zinc-300 leading-relaxed">
          <section>
            <h3 className="text-base font-semibold text-zinc-100 mb-2">The problem</h3>
            <p>
              Every month, Hacker News hosts an "Ask HN: Who is Hiring?" thread. These threads
              contain hundreds of job posts from companies around the world — ranging from early
              startups to large tech companies — and they are genuinely one of the best places
              to find interesting engineering roles.
            </p>
            <p className="mt-3">
              The problem is that they are nearly impossible to search. Each thread is a flat
              list of comments. There is no filtering, no keyword search, no way to narrow by
              remote region or salary. You end up scrolling through hundreds of posts hoping
              to spot something relevant.
            </p>
          </section>

          <section>
            <h3 className="text-base font-semibold text-zinc-100 mb-2">What this does</h3>
            <p>
              This app scrapes all recent "Who is Hiring?" threads from the{" "}
              <a
                href="https://hacker-news.firebaseio.com"
                target="_blank"
                rel="noopener noreferrer"
                className="text-orange-400 hover:text-orange-300 inline-flex items-center gap-1"
              >
                HN Firebase API <ExternalLink className="w-3 h-3" />
              </a>{" "}
              and the{" "}
              <a
                href="https://hn.algolia.com"
                target="_blank"
                rel="noopener noreferrer"
                className="text-orange-400 hover:text-orange-300 inline-flex items-center gap-1"
              >
                Algolia HN search API <ExternalLink className="w-3 h-3" />
              </a>
              , parses each job post, and indexes it into SQLite with full-text search.
            </p>
            <ul className="mt-3 flex flex-col gap-2 list-disc list-inside text-zinc-400">
              <li>Search by multiple keywords — ranked by relevance using FTS5 BM25</li>
              <li>Filter by remote region — Global, EU, or US</li>
              <li>Filter by salary range (multi-currency)</li>
              <li>Filter by location and date</li>
              <li>Trend analysis — see which technologies are growing or fading</li>
            </ul>
          </section>

          <section>
            <h3 className="text-base font-semibold text-zinc-100 mb-2">Stack</h3>
            <div className="grid grid-cols-2 gap-2 text-zinc-400">
              <div className="bg-zinc-900 border border-zinc-800 rounded-lg px-4 py-3">
                <p className="text-xs text-zinc-500 mb-1">Backend</p>
                <p>Go · Chi · SQLite FTS5</p>
              </div>
              <div className="bg-zinc-900 border border-zinc-800 rounded-lg px-4 py-3">
                <p className="text-xs text-zinc-500 mb-1">Frontend</p>
                <p>Next.js · React · Tailwind</p>
              </div>
              <div className="bg-zinc-900 border border-zinc-800 rounded-lg px-4 py-3">
                <p className="text-xs text-zinc-500 mb-1">Data</p>
                <p>HN Firebase API · Algolia</p>
              </div>
              <div className="bg-zinc-900 border border-zinc-800 rounded-lg px-4 py-3">
                <p className="text-xs text-zinc-500 mb-1">Infrastructure</p>
                <p>Docker · robfig/cron</p>
              </div>
            </div>
          </section>

          <section>
            <h3 className="text-base font-semibold text-zinc-100 mb-2">Caveats</h3>
            <p className="text-zinc-400">
              Salary, location, and remote region are extracted with regex heuristics — they
              are best-effort and will miss or misclassify some posts. The data is only as good
              as what people write in their comments. This is not affiliated with Hacker News
              or Y Combinator.
            </p>
          </section>

          <section className="border-t border-zinc-800 pt-6 text-zinc-500">
            <p>
              Built by{" "}
              <a
                href="https://michallyson.pl/"
                target="_blank"
                rel="noopener noreferrer"
                className="text-orange-400 hover:text-orange-300 inline-flex items-center gap-1 transition-colors"
              >
                Michał Łysoń <ExternalLink className="w-3 h-3" />
              </a>
            </p>
          </section>
        </div>
      </main>
    </div>
  );
}
