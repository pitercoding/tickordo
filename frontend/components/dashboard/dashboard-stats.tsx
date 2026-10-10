import { getDashboardStats } from "@/lib/api/dashboard";

export async function DashboardStatsCards() {
    let stats;

    try {
        stats = await getDashboardStats();
    } catch {
        // Show an inline alert so a stats failure doesn't take down the whole page.
        return (
            <section
                className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
                role="alert"
            >
                Unable to load dashboard statistics. Please try again later.
            </section>
        );
    }

    const cards = [
        {
            label: "Open tickets",
            value: stats.open_tickets,
        },
        {
            label: "High priority",
            value: stats.high_priority,
        },
        {
            label: "AI analyzed",
            value: stats.ai_analyzed,
        },
    ];

    return (
        <section className="grid gap-4 sm:grid-cols-3">
            {cards.map((card) => (
                <div
                    key={card.label}
                    className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm"
                >
                    <p className="text-sm font-medium text-slate-500">
                        {card.label}
                    </p>

                    <p className="mt-2 text-3xl font-semibold tracking-tight text-slate-900">
                        {card.value}
                    </p>
                </div>
            ))}
        </section>
    );
}

export function DashboardStatsSkeleton() {
    return (
        <section
            className="grid gap-4 sm:grid-cols-3"
            aria-label="Loading dashboard statistics"
        >
            {["open", "priority", "analyzed"].map((item) => (
                <div
                    key={item}
                    className="animate-pulse rounded-xl border border-slate-200 bg-white p-5 shadow-sm"
                >
                    <div className="h-4 w-28 rounded bg-slate-200" />
                    <div className="mt-3 h-9 w-16 rounded bg-slate-200" />
                </div>
            ))}
        </section>
    );
}
