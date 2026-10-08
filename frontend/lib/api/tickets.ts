import { apiClient } from "./client";

export type Ticket = {
    id: string;
    title: string;
    description: string;
    status: string;
    created_at: string;
    updated_at: string;
};

export async function getTickets(): Promise<Ticket[]> {
    return apiClient<Ticket[]>("/tickets");
}
