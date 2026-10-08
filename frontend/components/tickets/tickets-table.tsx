import { getTickets } from "@/lib/api/tickets";

export async function TicketsTable() {
  const tickets = await getTickets();

  return (
    <TicketsTableShell>
      {tickets.map((ticket) => (
        <tr key={ticket.id} className="transition hover:bg-slate-50">
          <td className="px-5 py-4">
            <p className="text-sm font-medium text-slate-900">{ticket.title}</p>
            <p className="mt-1 text-xs text-slate-500">{ticket.id}</p>
          </td>

          <td className="px-5 py-4 text-sm text-slate-600">
            {ticket.description}
          </td>

          <td className="px-5 py-4">
            <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700">
              {ticket.status}
            </span>
          </td>
        </tr>
      ))}
    </TicketsTableShell>
  );
}

export function TicketsTableSkeleton() {
  return (
    <TicketsTableShell>
      {Array.from({ length: 5 }).map((_, index) => (
        <tr key={index} className="animate-pulse">
          <td className="px-5 py-4">
            <div className="h-4 w-40 rounded bg-slate-200" />
            <div className="mt-2 h-3 w-24 rounded bg-slate-100" />
          </td>

          <td className="px-5 py-4">
            <div className="h-4 w-64 rounded bg-slate-100" />
          </td>

          <td className="px-5 py-4">
            <div className="h-5 w-16 rounded-full bg-slate-100" />
          </td>
        </tr>
      ))}
    </TicketsTableShell>
  );
}

function TicketsTableShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div className="overflow-x-auto">
        <table className="w-full min-w-175">
          <thead className="border-b border-slate-200 bg-slate-50">
            <tr>
              <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                Ticket
              </th>
              <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                Description
              </th>
              <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                Status
              </th>
            </tr>
          </thead>

          <tbody className="divide-y divide-slate-100">{children}</tbody>
        </table>
      </div>
    </div>
  );
}
