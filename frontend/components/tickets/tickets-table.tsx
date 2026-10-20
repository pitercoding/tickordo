import { unstable_rethrow } from "next/navigation";

import { getTickets } from "@/lib/api/tickets";
import TicketsTableClient from "./tickets-table-client";

export async function TicketsTable() {
  let tickets;

  try {
    tickets = await getTickets();
  } catch (error) {
    // Let Next.js handle its own control-flow errors, such as the rejection
    // of fetches that are still pending when a prerender ends.
    unstable_rethrow(error);

    // Keep the cause in the server logs; the inline alert only says it failed.
    console.error("Failed to load tickets:", error);

    // Show an inline alert so a list failure doesn't take down the whole page.
    return (
      <div
        className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
        role="alert"
      >
        Unable to load tickets. Please try again later.
      </div>
    );
  }

  return <TicketsTableClient tickets={tickets} />;
}

export function TicketsTableSkeleton() {
  // Mirrors the client layout (filters + sort + table) to avoid a layout shift on load.
  return (
    <div className="animate-pulse space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="h-10 flex-1 rounded-lg bg-slate-100" />
        <div className="h-10 rounded-lg bg-slate-100 sm:w-44" />
        <div className="h-10 rounded-lg bg-slate-100 sm:w-44" />
      </div>

      <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
        <div className="h-10 border-b border-slate-200 bg-slate-50" />

        <div className="divide-y divide-slate-100">
          {Array.from({ length: 5 }).map((_, index) => (
            <div key={index} className="grid grid-cols-3 gap-4 px-5 py-4">
              <div>
                <div className="h-4 w-40 rounded bg-slate-200" />
                <div className="mt-2 h-3 w-24 rounded bg-slate-100" />
              </div>
              <div className="h-4 w-64 max-w-full rounded bg-slate-100" />
              <div className="h-5 w-16 rounded-full bg-slate-100" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
