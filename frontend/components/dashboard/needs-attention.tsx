import Link from "next/link";

import { getTickets, type Ticket, type TicketStatus } from "@/lib/api/tickets";

import { ScrollToSectionLink } from "./scroll-to-section-link";

const MAX_TICKETS = 5;

// Only these priorities show up in the section; a lower rank is more urgent.
const urgencyRank: Record<string, number> = {
    critical: 0,
    high: 1,
};

const priorityStyles: Record<string, string> = {
    critical: "bg-red-100 text-red-700",
    high: "bg-amber-100 text-amber-800",
};

const priorityLabels: Record<string, string> = {
    critical: "Critical",
    high: "High",
};

const statusLabels: Record<TicketStatus, string> = {
    open: "Open",
    in_progress: "In progress",
    resolved: "Resolved",
};

function isUrgent(ticket: Ticket): boolean {
    return (
        ticket.status !== "resolved" &&
        ticket.latest_triage_priority !== null &&
        ticket.latest_triage_priority in urgencyRank
    );
}

// Most urgent priority first; newest first within the same priority.
function sortByUrgency(a: Ticket, b: Ticket): number {
    const priorityDifference =
        urgencyRank[a.latest_triage_priority ?? ""] -
        urgencyRank[b.latest_triage_priority ?? ""];

    if (priorityDifference !== 0) {
        return priorityDifference;
    }

    return (
        new Date(b.created_at).getTime() -
        new Date(a.created_at).getTime()
    );
}

export async function NeedsAttention() {
    let tickets;

    try {
        tickets = await getTickets();
    } catch {
        // Show an inline alert so this section doesn't take down the whole page.
        return (
            <section
                className="mt-8 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
                role="alert"
            >
                Unable to load tickets that need attention. Please try again later.
            </section>
        );
    }

    const urgentTickets = tickets
        .filter(isUrgent)
        .sort(sortByUrgency)
        .slice(0, MAX_TICKETS);

    return (
        <section className="mt-8">
            <div className="mb-4 flex items-center justify-between gap-4">
                <div>
                    <h2 className="text-lg font-semibold text-slate-900">
                        Needs attention
                    </h2>
                    <p className="mt-1 text-sm text-slate-500">
                        Unresolved critical and high priority tickets, most urgent first.
                    </p>
                </div>

                <ScrollToSectionLink
                    targetId="recent-tickets"
                    className="shrink-0 text-sm font-medium text-slate-700 transition hover:text-slate-900"
                >
                    View all
                </ScrollToSectionLink>
            </div>

            <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
                {urgentTickets.length === 0 ? (
                    <div className="px-5 py-8 text-center">
                        <p className="font-medium text-slate-900">
                            No urgent tickets
                        </p>
                        <p className="mt-1 text-sm text-slate-500">
                            There are no unresolved high or critical priority tickets.
                        </p>
                    </div>
                ) : (
                    <ul className="divide-y divide-slate-100">
                        {urgentTickets.map((ticket) => {
                            const priority = ticket.latest_triage_priority ?? "";

                            return (
                                <li key={ticket.id}>
                                    <Link
                                        href={`/tickets/${ticket.id}`}
                                        className="flex flex-col gap-3 px-5 py-4 transition hover:bg-slate-50 sm:flex-row sm:items-center sm:justify-between"
                                    >
                                        <div className="min-w-0">
                                            <p className="truncate font-medium text-slate-900">
                                                {ticket.title}
                                            </p>
                                            <p className="mt-1 text-sm text-slate-500">
                                                {ticket.latest_triage_category ?? "Uncategorized"}
                                                {" · "}
                                                {statusLabels[ticket.status]}
                                            </p>
                                        </div>

                                        <span
                                            className={`w-fit shrink-0 rounded-full px-2.5 py-1 text-xs font-medium ${priorityStyles[priority]}`}
                                        >
                                            {priorityLabels[priority]}
                                        </span>
                                    </Link>
                                </li>
                            );
                        })}
                    </ul>
                )}
            </div>
        </section>
    );
}

export function NeedsAttentionSkeleton() {
    return (
        <section
            className="mt-8 animate-pulse"
            aria-label="Loading tickets that need attention"
        >
            <div className="mb-4">
                <div className="h-6 w-40 rounded bg-slate-200" />
                <div className="mt-2 h-4 w-72 max-w-full rounded bg-slate-100" />
            </div>

            <div className="divide-y divide-slate-100 overflow-hidden rounded-xl border border-slate-200 bg-white">
                {Array.from({ length: 3 }).map((_, index) => (
                    <div
                        key={index}
                        className="flex items-center justify-between gap-4 px-5 py-4"
                    >
                        <div className="min-w-0 flex-1">
                            <div className="h-4 w-48 max-w-full rounded bg-slate-200" />
                            <div className="mt-2 h-3 w-32 rounded bg-slate-100" />
                        </div>
                        <div className="h-6 w-16 rounded-full bg-slate-100" />
                    </div>
                ))}
            </div>
        </section>
    );
}
