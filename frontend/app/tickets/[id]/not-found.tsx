import Link from "next/link";

export default function TicketNotFound() {
    return (
        <main className="flex min-h-screen items-center justify-center px-6">
            <div className="w-full max-w-md rounded-xl border border-slate-200 bg-white p-8 text-center shadow-sm">
                <h1 className="text-xl font-semibold text-slate-900">
                    Ticket not found
                </h1>

                <p className="mt-2 text-sm text-slate-500">
                    This ticket doesn&apos;t exist or may have been removed.
                </p>

                <Link
                    href="/"
                    className="mt-6 inline-flex rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700"
                >
                    Back to dashboard
                </Link>
            </div>
        </main>
    );
}