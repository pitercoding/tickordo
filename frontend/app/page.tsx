import { Suspense } from "react";
import Link from "next/link";

import {
  TicketsTable,
  TicketsTableSkeleton,
} from "@/components/tickets/tickets-table";

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

        <section className="grid gap-4 sm:grid-cols-3">
          <div className="rounded-xl border border-slate-200 bg-white p-5">
            <p className="text-sm font-medium text-slate-500">Open tickets</p>
            <p className="mt-2 text-3xl font-semibold text-slate-900">24</p>
          </div>

          <div className="rounded-xl border border-slate-200 bg-white p-5">
            <p className="text-sm font-medium text-slate-500">
              High priority
            </p>
            <p className="mt-2 text-3xl font-semibold text-slate-900">7</p>
          </div>

          <div className="rounded-xl border border-slate-200 bg-white p-5">
            <p className="text-sm font-medium text-slate-500">
              AI analyzed
            </p>
            <p className="mt-2 text-3xl font-semibold text-slate-900">18</p>
          </div>
        </section>

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
