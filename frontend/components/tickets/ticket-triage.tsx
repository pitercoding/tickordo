"use client";

import { useEffect, useState } from "react";
import { ApiError } from "@/lib/api/client";
import {
    analyzeTicket,
    getLatestTicketTriage,
    type TicketTriage as TicketTriageResult,
} from "@/lib/api/triages";

type TicketTriageProps = {
    ticketId: string;
};

function getTriageErrorMessage(
    error: unknown,
    fallback: string,
): string {
    if (error instanceof ApiError && error.status < 500) {
        return error.message;
    }

    return fallback;
}

export default function TicketTriagePanel({
    ticketId,
}: TicketTriageProps) {
    const [triage, setTriage] = useState<TicketTriageResult | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [isAnalyzing, setIsAnalyzing] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;

        async function loadLatestTriage() {
            try {
                const result = await getLatestTicketTriage(ticketId);

                if (!cancelled) {
                    setTriage(result);
                }
            } catch (err) {
                if (cancelled) {
                    return;
                }

                if (err instanceof ApiError && err.status === 404) {
                    setTriage(null);
                } else {
                    setError(
                        getTriageErrorMessage(
                            err,
                            "Failed to load the AI analysis. Please try again.",
                        ),
                    );
                }
            } finally {
                if (!cancelled) {
                    setIsLoading(false);
                }
            }
        }

        void loadLatestTriage();

        return () => {
            cancelled = true;
        };
    }, [ticketId]);

    async function handleAnalyze() {
        setIsAnalyzing(true);
        setError(null);

        try {
            const result = await analyzeTicket(ticketId);
            setTriage(result);
        } catch (err) {
            setError(
                getTriageErrorMessage(
                    err,
                    "Failed to analyze the ticket. Please try again.",
                ),
            );
        } finally {
            setIsAnalyzing(false);
        }
    }

    return (
        <section className="mt-8 rounded-xl border border-slate-200 bg-white p-6">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <p className="text-sm font-medium text-indigo-600">
                        AI-POWERED TRIAGE
                    </p>
                    <h2 className="mt-1 text-xl font-semibold text-slate-900">
                        Ticket analysis
                    </h2>
                    <p className="mt-2 text-sm text-slate-500">
                        Analyze the ticket to get a suggested category,
                        priority, and next action.
                    </p>
                </div>

                <button
                    type="button"
                    onClick={handleAnalyze}
                    disabled={isLoading || isAnalyzing}
                    className="inline-flex shrink-0 items-center justify-center rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                    {isAnalyzing
                        ? "Analyzing..."
                        : triage
                            ? "Analyze again"
                            : "Analyze with AI"}
                </button>
            </div>

            {isLoading && (
                <p className="mt-6 text-sm text-slate-500">
                    Loading previous analysis...
                </p>
            )}

            {!isLoading && !triage && !error && (
                <div className="mt-6 rounded-lg border border-dashed border-slate-300 p-5 text-center">
                    <p className="text-sm text-slate-600">
                        This ticket has not been analyzed yet.
                    </p>
                </div>
            )}

            {error && (
                <div
                    role="alert"
                    className="mt-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
                >
                    {error}
                </div>
            )}

            {triage && (
                <div className="mt-6 space-y-6 border-t border-slate-100 pt-6">
                    <div className="grid gap-4 sm:grid-cols-3">
                        <div className="rounded-lg bg-slate-50 p-4">
                            <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
                                Category
                            </p>
                            <p className="mt-2 text-sm font-semibold text-slate-900">
                                {triage.category}
                            </p>
                        </div>

                        <div className="rounded-lg bg-slate-50 p-4">
                            <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
                                Priority
                            </p>
                            <p className="mt-2 text-sm font-semibold capitalize text-slate-900">
                                {triage.priority}
                            </p>
                        </div>

                        <div className="rounded-lg bg-slate-50 p-4">
                            <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
                                Sentiment
                            </p>
                            <p className="mt-2 text-sm font-semibold capitalize text-slate-900">
                                {triage.sentiment}
                            </p>
                        </div>
                    </div>

                    <div>
                        <h3 className="text-sm font-semibold text-slate-900">
                            Suggested team
                        </h3>
                        <p className="mt-2 text-sm text-slate-700">
                            {triage.suggested_team}
                        </p>
                    </div>

                    <div>
                        <h3 className="text-sm font-semibold text-slate-900">
                            Summary
                        </h3>
                        <p className="mt-2 text-sm leading-7 text-slate-700">
                            {triage.summary}
                        </p>
                    </div>

                    <div>
                        <h3 className="text-sm font-semibold text-slate-900">
                            Suggested action
                        </h3>
                        <p className="mt-2 text-sm leading-7 text-slate-700">
                            {triage.suggested_action}
                        </p>
                    </div>

                    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-4">
                        <div>
                            <p className="text-xs text-slate-500">
                                Model: {triage.model}
                            </p>
                            <p className="mt-1 text-xs text-slate-500">
                                Analyzed on{" "}
                                {new Date(triage.created_at).toLocaleString(
                                    "en-GB",
                                )}
                            </p>
                        </div>

                        <p className="text-sm font-medium text-slate-600">
                            Confidence:{" "}
                            {Math.round(triage.confidence * 100)}%
                        </p>
                    </div>
                </div>
            )}
        </section>
    );
}
