"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "@/lib/api/client";
import {
    updateTicketStatus,
    type TicketStatus,
} from "@/lib/api/tickets";

type TicketStatusSelectorProps = {
    ticketId: string;
    initialStatus: TicketStatus;
};

const statusOptions: { value: TicketStatus; label: string }[] = [
    { value: "open", label: "Open" },
    { value: "in_progress", label: "In progress" },
    { value: "resolved", label: "Resolved" },
];

export default function TicketStatusSelector({
    ticketId,
    initialStatus,
}: TicketStatusSelectorProps) {
    const router = useRouter();

    const [currentStatus, setCurrentStatus] =
        useState<TicketStatus>(initialStatus);
    const [selectedStatus, setSelectedStatus] =
        useState<TicketStatus>(initialStatus);
    const [isSaving, setIsSaving] = useState(false);
    const [message, setMessage] = useState("");
    const [error, setError] = useState("");

    async function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();

        if (selectedStatus === currentStatus || isSaving) {
            return;
        }

        setIsSaving(true);
        setMessage("");
        setError("");

        try {
            await updateTicketStatus(ticketId, selectedStatus);

            setCurrentStatus(selectedStatus);
            setMessage("Ticket status updated successfully.");

            // Re-render the server page so fields like "Last updated" stay in sync.
            router.refresh();
        } catch (err) {
            // 4xx errors carry a validation message from the API worth showing.
            if (err instanceof ApiError && err.status < 500) {
                setError(err.message);
            } else {
                setError(
                    "Unable to update the ticket status. Please try again.",
                );
            }
        } finally {
            setIsSaving(false);
        }
    }

    return (
        <div>
            <h2 className="text-sm font-medium text-slate-500">
                Status
            </h2>

            <form
                onSubmit={handleSubmit}
                className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-center"
            >
                <label htmlFor="ticket-status" className="sr-only">
                    Select ticket status
                </label>

                <select
                    id="ticket-status"
                    value={selectedStatus}
                    onChange={(event) => {
                        setSelectedStatus(
                            event.target.value as TicketStatus,
                        );
                        setMessage("");
                        setError("");
                    }}
                    disabled={isSaving}
                    className="rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-700 outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200 disabled:cursor-not-allowed disabled:opacity-60"
                >
                    {statusOptions.map((option) => (
                        <option key={option.value} value={option.value}>
                            {option.label}
                        </option>
                    ))}
                </select>

                <button
                    type="submit"
                    disabled={isSaving || selectedStatus === currentStatus}
                    className="rounded-lg bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                    {isSaving ? "Saving..." : "Save status"}
                </button>
            </form>

            {message && (
                <p className="mt-3 text-sm text-green-700" role="status">
                    {message}
                </p>
            )}

            {error && (
                <p className="mt-3 text-sm text-red-700" role="alert">
                    {error}
                </p>
            )}
        </div>
    );
}
