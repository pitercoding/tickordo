import { apiClient } from "./client";

// The backend gives OpenAI up to 60 seconds per analysis, so the default
// API timeout would abort triages the server is still able to finish.
const ANALYZE_TIMEOUT_MS = 75_000;

export type TicketTriage = {
    id: string;
    ticket_id: string;
    category: string;
    priority: string;
    sentiment: string;
    suggested_team: string;
    summary: string;
    suggested_action: string;
    confidence: number;
    model: string;
    created_at: string;
};

export async function getTicketTriages(
    ticketId: string,
): Promise<TicketTriage[]> {
    return apiClient<TicketTriage[]>(
        `/tickets/${encodeURIComponent(ticketId)}/triages`,
    );
}

export async function analyzeTicket(
    ticketId: string,
): Promise<TicketTriage> {
    return apiClient<TicketTriage>(
        `/tickets/${encodeURIComponent(ticketId)}/triage`,
        {
            method: "POST",
            timeoutMs: ANALYZE_TIMEOUT_MS,
        },
    );
}
