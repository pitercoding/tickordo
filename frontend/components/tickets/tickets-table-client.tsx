"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import type { Ticket, TicketStatus } from "@/lib/api/tickets";

type TicketsTableClientProps = {
    tickets: Ticket[];
};

type StatusFilter = "all" | TicketStatus;

const PAGE_SIZE = 10;

const statusLabels: Record<TicketStatus, string> = {
    open: "Open",
    in_progress: "In progress",
    resolved: "Resolved",
};

const statusOptions: { value: StatusFilter; label: string }[] = [
    { value: "all", label: "All statuses" },
    { value: "open", label: statusLabels.open },
    { value: "in_progress", label: statusLabels.in_progress },
    { value: "resolved", label: statusLabels.resolved },
];

// Badge colors for the priorities the AI analyzer can return.
const priorityStyles: Record<string, string> = {
    low: "bg-slate-100 text-slate-700",
    medium: "bg-blue-100 text-blue-700",
    high: "bg-amber-100 text-amber-800",
    critical: "bg-red-100 text-red-700",
};

type SortOption = "newest" | "oldest" | "title-asc" | "title-desc";

const sortOptions: { value: SortOption; label: string }[] = [
    { value: "newest", label: "Newest first" },
    { value: "oldest", label: "Oldest first" },
    { value: "title-asc", label: "Title A–Z" },
    { value: "title-desc", label: "Title Z–A" },
];

export default function TicketsTableClient({
    tickets,
}: TicketsTableClientProps) {
    const [search, setSearch] = useState("");
    const [statusFilter, setStatusFilter] =
        useState<StatusFilter>("all");
    const [currentPage, setCurrentPage] = useState(1);
    const [sortBy, setSortBy] = useState<SortOption>("newest");

    const filteredTickets = useMemo(() => {
        const normalizedSearch = search.trim().toLowerCase();

        return tickets.filter((ticket) => {
            const matchesTitle = ticket.title
                .toLowerCase()
                .includes(normalizedSearch);

            const matchesStatus =
                statusFilter === "all" ||
                ticket.status === statusFilter;

            return matchesTitle && matchesStatus;
        });
    }, [tickets, search, statusFilter]);

    const sortedTickets = useMemo(() => {
        return [...filteredTickets].sort((a, b) => {
            switch (sortBy) {
                case "newest":
                    return (
                        new Date(b.created_at).getTime() -
                        new Date(a.created_at).getTime()
                    );

                case "oldest":
                    return (
                        new Date(a.created_at).getTime() -
                        new Date(b.created_at).getTime()
                    );

                case "title-asc":
                    return a.title.localeCompare(b.title);

                case "title-desc":
                    return b.title.localeCompare(a.title);
            }
        });
    }, [filteredTickets, sortBy]);

    const totalPages = Math.ceil(sortedTickets.length / PAGE_SIZE);

    // Keep the page in range if the ticket list shrinks (e.g. after a refresh).
    const safeCurrentPage = Math.min(currentPage, Math.max(totalPages, 1));

    const startIndex = (safeCurrentPage - 1) * PAGE_SIZE;

    const paginatedTickets = sortedTickets.slice(
        startIndex,
        startIndex + PAGE_SIZE,
    );

    const firstTicketNumber =
        sortedTickets.length === 0 ? 0 : startIndex + 1;

    const lastTicketNumber = Math.min(
        startIndex + PAGE_SIZE,
        sortedTickets.length,
    );

    const hasTickets = tickets.length > 0;

    return (
        <div className="space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row">
                <div className="flex-1">
                    <label
                        htmlFor="ticket-search"
                        className="sr-only"
                    >
                        Search tickets by title
                    </label>
                    <input
                        id="ticket-search"
                        type="search"
                        placeholder="Search tickets by title..."
                        value={search}
                        onChange={(event) => {
                            setSearch(event.target.value);
                            setCurrentPage(1);
                        }}
                        className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-700 outline-none placeholder:text-slate-400 focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    />
                </div>

                <div>
                    <label
                        htmlFor="ticket-status-filter"
                        className="sr-only"
                    >
                        Filter tickets by status
                    </label>
                    <select
                        id="ticket-status-filter"
                        value={statusFilter}
                        onChange={(event) => {
                            setStatusFilter(
                                event.target.value as StatusFilter,
                            );
                            setCurrentPage(1);
                        }}
                        className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-700 outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200 sm:min-w-44"
                    >
                        {statusOptions.map((option) => (
                            <option
                                key={option.value}
                                value={option.value}
                            >
                                {option.label}
                            </option>
                        ))}
                    </select>
                </div>

                <div>
                    <label
                        htmlFor="ticket-sort"
                        className="sr-only"
                    >
                        Sort tickets
                    </label>
                    <select
                        id="ticket-sort"
                        value={sortBy}
                        onChange={(event) => {
                            setSortBy(event.target.value as SortOption);
                            setCurrentPage(1);
                        }}
                        className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-700 outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200 sm:min-w-44"
                    >
                        {sortOptions.map((option) => (
                            <option
                                key={option.value}
                                value={option.value}
                            >
                                {option.label}
                            </option>
                        ))}
                    </select>
                </div>
            </div>

            {filteredTickets.length === 0 ? (
                <div className="rounded-xl border border-dashed border-slate-300 bg-white px-6 py-10 text-center">
                    {/* Distinguish an empty backlog from filters that match nothing. */}
                    {hasTickets ? (
                        <>
                            <p className="text-sm font-medium text-slate-900">
                                No tickets found
                            </p>
                            <p className="mt-1 text-sm text-slate-500">
                                Try changing your search or status filter.
                            </p>
                            <button
                                type="button"
                                onClick={() => {
                                    setSearch("");
                                    setStatusFilter("all");
                                    setCurrentPage(1);
                                }}
                                className="mt-4 text-sm font-medium text-blue-700 transition hover:text-blue-800"
                            >
                                Clear filters
                            </button>
                        </>
                    ) : (
                        <>
                            <p className="text-sm font-medium text-slate-900">
                                No tickets yet
                            </p>
                            <p className="mt-1 text-sm text-slate-500">
                                Create a ticket to start triaging requests.
                            </p>
                        </>
                    )}
                </div>
            ) : (
                <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
                    <div className="overflow-x-auto">
                        <table className="w-full min-w-225">
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
                                    <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                                        Category
                                    </th>
                                    <th className="px-5 py-3 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
                                        Priority
                                    </th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100">
                                {paginatedTickets.map((ticket) => (
                                    <tr
                                        key={ticket.id}
                                        className="transition hover:bg-slate-50"
                                    >
                                        <td className="px-5 py-4">
                                            <Link
                                                href={`/tickets/${ticket.id}`}
                                                className="text-sm font-medium text-slate-900 transition hover:text-blue-700"
                                            >
                                                {ticket.title}
                                            </Link>
                                            <p className="mt-1 text-xs text-slate-500">
                                                {ticket.id}
                                            </p>
                                        </td>
                                        <td className="px-5 py-4 text-sm text-slate-600">
                                            {ticket.description}
                                        </td>
                                        <td className="px-5 py-4">
                                            <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700">
                                                {statusLabels[ticket.status]}
                                            </span>
                                        </td>
                                        <td className="px-5 py-4 text-sm text-slate-600">
                                            {ticket.latest_triage_category ?? (
                                                <span className="whitespace-nowrap text-slate-400">
                                                    Not analyzed
                                                </span>
                                            )}
                                        </td>
                                        <td className="px-5 py-4">
                                            {ticket.latest_triage_priority ? (
                                                <span
                                                    className={`rounded-full px-2.5 py-1 text-xs font-medium capitalize ${
                                                        priorityStyles[ticket.latest_triage_priority] ??
                                                        priorityStyles.low
                                                    }`}
                                                >
                                                    {ticket.latest_triage_priority}
                                                </span>
                                            ) : (
                                                <span className="whitespace-nowrap text-sm text-slate-400">
                                                    Not analyzed
                                                </span>
                                            )}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <p
                    className="text-xs text-slate-500"
                    aria-live="polite"
                >
                    Showing {firstTicketNumber}–{lastTicketNumber} of{" "}
                    {sortedTickets.length} filtered tickets
                    {" "}({tickets.length} total)
                </p>

                {totalPages > 1 && (
                    <nav
                        aria-label="Ticket pagination"
                        className="flex items-center gap-3"
                    >
                        <button
                            type="button"
                            onClick={() =>
                                setCurrentPage(safeCurrentPage - 1)
                            }
                            disabled={safeCurrentPage === 1}
                            className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                            Previous
                        </button>

                        <span className="text-sm text-slate-600">
                            Page {safeCurrentPage} of {totalPages}
                        </span>

                        <button
                            type="button"
                            onClick={() =>
                                setCurrentPage(safeCurrentPage + 1)
                            }
                            disabled={safeCurrentPage === totalPages}
                            className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                            Next
                        </button>
                    </nav>
                )}
            </div>
        </div>
    );
}
