"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "@/lib/api/client";
import { createTicket } from "@/lib/api/tickets";

// Mirror the limits enforced by POST /tickets in the backend.
const MAX_TITLE_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 5000;

export function TicketForm() {
    const router = useRouter();

    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [error, setError] = useState("");

    async function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();

        const trimmedTitle = title.trim();
        const trimmedDescription = description.trim();

        // `required` accepts whitespace-only values, so check the trimmed ones.
        if (!trimmedTitle || !trimmedDescription) {
            setError("Title and description are required.");
            return;
        }

        setError("");
        setIsSubmitting(true);

        try {
            await createTicket({
                title: trimmedTitle,
                description: trimmedDescription,
            });

            // Keep the form disabled while navigating to avoid duplicate submits.
            router.push("/");
            router.refresh();
        } catch (err) {
            // 4xx errors carry a validation message from the API worth showing.
            if (err instanceof ApiError && err.status < 500) {
                setError(err.message);
            } else {
                setError(
                    "We couldn't create the ticket. Please check your connection and try again.",
                );
            }

            setIsSubmitting(false);
        }
    }

    return (
        <form
            onSubmit={handleSubmit}
            className="space-y-6 rounded-xl border border-slate-200 bg-white p-6"
        >
            <div>
                <label
                    htmlFor="title"
                    className="mb-2 block text-sm font-medium text-slate-700"
                >
                    Title
                </label>

                <input
                    id="title"
                    name="title"
                    type="text"
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    maxLength={MAX_TITLE_LENGTH}
                    required
                    disabled={isSubmitting}
                    placeholder="e.g. Unable to access my account"
                    className="w-full rounded-lg border border-slate-300 px-4 py-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200 disabled:bg-slate-50"
                />
            </div>

            <div>
                <label
                    htmlFor="description"
                    className="mb-2 block text-sm font-medium text-slate-700"
                >
                    Description
                </label>

                <textarea
                    id="description"
                    name="description"
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    maxLength={MAX_DESCRIPTION_LENGTH}
                    required
                    rows={6}
                    disabled={isSubmitting}
                    placeholder="Describe the problem and what you have already tried..."
                    className="w-full resize-y rounded-lg border border-slate-300 px-4 py-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200 disabled:bg-slate-50"
                />
            </div>

            {error && (
                <p role="alert" className="text-sm text-red-600">
                    {error}
                </p>
            )}

            <div className="flex items-center justify-end gap-3">
                <button
                    type="button"
                    onClick={() => router.push("/")}
                    disabled={isSubmitting}
                    className="rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                    Cancel
                </button>

                <button
                    type="submit"
                    disabled={isSubmitting}
                    className="rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                    {isSubmitting ? "Creating..." : "Create ticket"}
                </button>
            </div>
        </form>
    );
}
