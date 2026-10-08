const tickets = [
  {
    id: "#TK-1042",
    title: "Unable to access Microsoft account",
    category: "Account Access",
    priority: "High",
    status: "Open",
  },
  {
    id: "#TK-1041",
    title: "Billing information needs to be updated",
    category: "Billing",
    priority: "Medium",
    status: "Open",
  },
  {
    id: "#TK-1040",
    title: "Password reset email not received",
    category: "Account Access",
    priority: "High",
    status: "Analyzed",
  },
];

const priorityStyles = {
  High: "bg-red-50 text-red-700",
  Medium: "bg-amber-50 text-amber-700",
};

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

          <button className="rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700">
            + New ticket
          </button>
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

          <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
            <div className="overflow-x-auto">
              <table className="w-full min-w-175">
                <thead className="border-b border-slate-200 bg-slate-50">
                  <tr>
                    <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                      Ticket
                    </th>
                    <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                      Category
                    </th>
                    <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                      Priority
                    </th>
                    <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                      Status
                    </th>
                  </tr>
                </thead>

                <tbody className="divide-y divide-slate-100">
                  {tickets.map((ticket) => (
                    <tr
                      key={ticket.id}
                      className="transition hover:bg-slate-50"
                    >
                      <td className="px-5 py-4">
                        <p className="text-sm font-medium text-slate-900">
                          {ticket.title}
                        </p>
                        <p className="mt-1 text-xs text-slate-500">
                          {ticket.id}
                        </p>
                      </td>

                      <td className="px-5 py-4 text-sm text-slate-600">
                        {ticket.category}
                      </td>

                      <td className="px-5 py-4">
                        <span
                          className={`rounded-full px-2.5 py-1 text-xs font-medium ${priorityStyles[
                            ticket.priority as keyof typeof priorityStyles
                            ]
                            }`}
                        >
                          {ticket.priority}
                        </span>
                      </td>

                      <td className="px-5 py-4">
                        <span className="text-sm text-slate-600">
                          {ticket.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
