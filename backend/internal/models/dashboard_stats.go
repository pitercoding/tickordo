package models

// DashboardStats holds the aggregated counters shown on the dashboard.
type DashboardStats struct {
	OpenTickets  int64 `json:"open_tickets"`
	HighPriority int64 `json:"high_priority"`
	AIAnalyzed   int64 `json:"ai_analyzed"`
}
