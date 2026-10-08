import Link from "next/link";
import { TicketForm } from "@/components/tickets/ticket-form";

export default function NewTicketPage() {
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
                        TICKORDO / TICKETS
                    </p>

                    <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900">
                        Create a new ticket
                    </h1>

                    <p className="mt-3 text-sm leading-6 text-slate-600">
                        Describe the customer&apos;s issue. Tickordo will save the ticket so it
                        can be reviewed and analyzed.
                    </p>
                </div>

                <TicketForm />
            </div>
        </main>
    );
}
