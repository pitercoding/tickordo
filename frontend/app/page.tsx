import { Suspense } from "react";
import Link from "next/link";

import {
  TicketsTable,
  TicketsTableSkeleton,
} from "@/components/tickets/tickets-table";

import {
  DashboardStatsCards,
  DashboardStatsSkeleton,
} from "@/components/dashboard/dashboard-stats";

export default function Home() {
  return (
    <main className="min-h-screen">
      <div className="mx-auto max-w-7xl px-6 py-8 lg:px-8">
        <header className="flex items-center justify-between border-b border-slate-200 pb-6">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
              Tickordo
            </h1>
            <p className="mt-1 text-sm text-slate-500">
              AI-powered ticket triage
            </p>
          </div>

          <Link
            href="/tickets/new"
            className="inline-flex items-center gap-2 rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700"
          >
            <span aria-hidden="true">+</span>
            New ticket
          </Link>
        </header>

        <section className="py-8">
          <h2 className="text-2xl font-semibold tracking-tight text-slate-900">
            Good morning
          </h2>
          <p className="mt-1 text-slate-500">
            Monitor and prioritize your support tickets.
          </p>
        </section>

        <Suspense fallback={<DashboardStatsSkeleton />}>
          <DashboardStatsCards />
        </Suspense>

        <section className="mt-8">
          <div className="mb-4 flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold text-slate-900">
                Recent tickets
              </h2>
              <p className="mt-1 text-sm text-slate-500">
                Latest support requests and their current triage status.
              </p>
            </div>

            <button className="text-sm font-medium text-slate-700 hover:text-slate-900">
              View all
            </button>
          </div>

          <Suspense fallback={<TicketsTableSkeleton />}>
            <TicketsTable />
          </Suspense>
        </section>
      </div>
    </main>
  );
}
