import Link from "next/link";
import { Suspense } from "react";
import { notFound } from "next/navigation";
import { getTicket } from "@/lib/api/tickets";
import { ApiError } from "@/lib/api/client";
import TicketTriagePanel from "@/components/tickets/ticket-triage";

type TicketDetailsPageProps = {
    params: Promise<{ id: string }>;
};

export default function TicketDetailsPage({
    params,
}: TicketDetailsPageProps) {
    return (
        <Suspense fallback={<TicketDetailsSkeleton />}>
            <TicketDetails params={params} />
        </Suspense>
    );
}

async function TicketDetails({
    params,
}: TicketDetailsPageProps) {
    const { id } = await params;

    let ticket;

    try {
        ticket = await getTicket(id);
    } catch (error) {
        if (
            error instanceof ApiError &&
            (error.status === 400 || error.status === 404)
        ) {
            notFound();
        }

        throw error;
    }

    return (
        <main className="min-h-screen px-6 py-10 sm:px-10">
            <div className="mx-auto max-w-3xl">
                <Link
                    href="/"
                    className="text-sm font-medium text-slate-500 transition hover:text-slate-900"
                >
                    ← Back to dashboard
                </Link>

                <div className="mb-8 mt-8">
                    <p className="text-sm font-medium text-slate-500">
                        TICKORDO / TICKETS / DETAILS
                    </p>

                    <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900">
                        {ticket.title}
                    </h1>

                    <p className="mt-3 text-sm text-slate-500">
                        Ticket ID: {ticket.id}
                    </p>
                </div>

                <section className="space-y-6 rounded-xl border border-slate-200 bg-white p-6">
                    <div>
                        <h2 className="text-sm font-medium text-slate-500">
                            Status
                        </h2>
                        <span className="mt-2 inline-flex rounded-full bg-slate-100 px-3 py-1 text-sm font-medium text-slate-700">
                            {ticket.status}
                        </span>
                    </div>

                    <div>
                        <h2 className="text-sm font-medium text-slate-500">
                            Description
                        </h2>
                        <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-slate-700">
                            {ticket.description}
                        </p>
                    </div>

                    <div className="grid gap-5 border-t border-slate-100 pt-5 sm:grid-cols-2">
                        <div>
                            <h2 className="text-sm font-medium text-slate-500">
                                Created at
                            </h2>
                            <p className="mt-2 text-sm text-slate-700">
                                {new Date(ticket.created_at).toLocaleString("en-GB")}
                            </p>
                        </div>

                        <div>
                            <h2 className="text-sm font-medium text-slate-500">
                                Last updated
                            </h2>
                            <p className="mt-2 text-sm text-slate-700">
                                {new Date(ticket.updated_at).toLocaleString("en-GB")}
                            </p>
                        </div>
                    </div>
                </section>

                <TicketTriagePanel ticketId={ticket.id} />

                <div className="mt-6">
                    <Link
                        href="/"
                        className="inline-flex rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700"
                    >
                        Back to dashboard
                    </Link>
                </div>
            </div>
        </main>
    );
}

function TicketDetailsSkeleton() {
    return (
        <main className="min-h-screen px-6 py-10 sm:px-10">
            <div className="mx-auto max-w-3xl animate-pulse">
                <div className="h-4 w-36 rounded bg-slate-200" />

                <div className="mb-8 mt-8">
                    <div className="h-3 w-48 rounded bg-slate-200" />
                    <div className="mt-3 h-8 w-3/4 rounded bg-slate-200" />
                    <div className="mt-3 h-4 w-64 rounded bg-slate-100" />
                </div>

                <div className="space-y-6 rounded-xl border border-slate-200 bg-white p-6">
                    <div className="h-4 w-20 rounded bg-slate-200" />
                    <div className="h-6 w-24 rounded-full bg-slate-100" />

                    <div>
                        <div className="h-4 w-28 rounded bg-slate-200" />
                        <div className="mt-3 h-4 w-full rounded bg-slate-100" />
                        <div className="mt-2 h-4 w-2/3 rounded bg-slate-100" />
                    </div>

                    <div className="border-t border-slate-100 pt-5">
                        <div className="h-4 w-32 rounded bg-slate-200" />
                        <div className="mt-3 h-4 w-40 rounded bg-slate-100" />
                    </div>
                </div>
            </div>
        </main>
    );
}
