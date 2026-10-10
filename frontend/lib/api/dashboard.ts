import { apiClient } from "./client";

export type DashboardStats = {
    open_tickets: number;
    high_priority: number;
    ai_analyzed: number;
};

export async function getDashboardStats(): Promise<DashboardStats> {
    return apiClient<DashboardStats>("/dashboard/stats");
}
