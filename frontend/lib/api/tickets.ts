import { apiClient } from "./client";

export type Ticket = {
    id: string;
    title: string;
    description: string;
    status: string;
    created_at: string;
    updated_at: string;
};

export type CreateTicketRequest = {
    title: string;
    description: string;
};

export async function getTickets(): Promise<Ticket[]> {
    return apiClient<Ticket[]>("/tickets");
}

export async function createTicket(
    data: CreateTicketRequest,
): Promise<Ticket> {
    return apiClient<Ticket>("/tickets", {
        method: "POST",
        body: JSON.stringify(data),
    });
}

export async function getTicket(id: string): Promise<Ticket> {
    return apiClient<Ticket>(`/tickets/${encodeURIComponent(id)}`);
}
