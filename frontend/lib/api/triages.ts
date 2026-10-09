
import { apiClient } from "./client";

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
        },
    );
}

export async function getLatestTicketTriage(
    ticketId: string,
): Promise<TicketTriage> {
    return apiClient<TicketTriage>(
        `/tickets/${encodeURIComponent(ticketId)}/triage`,
    );
}
