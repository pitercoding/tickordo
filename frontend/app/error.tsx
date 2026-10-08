"use client";

export default function Error({
    retry,
}: {
    retry: () => void;
}) {
    return (
        <main className="flex min-h-screen items-center justify-center px-6">
            <div className="w-full max-w-md rounded-xl border border-slate-200 bg-white p-8 text-center shadow-sm">
                <h1 className="text-xl font-semibold text-slate-900">
                    Something went wrong
                </h1>

                <p className="mt-2 text-sm text-slate-500">
                    We couldn&apos;t load the Tickordo dashboard. Please try again.
                </p>

                <button
                    onClick={() => retry()}
                    className="mt-6 rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700"
                >
                    Try again
                </button>
            </div>
        </main>
    );
};
