CREATE TABLE ticket_triages (
    id UUID PRIMARY KEY,
    ticket_id UUID NOT NULL REFERENCES tickets(id),
    category VARCHAR(50) NOT NULL,
    priority VARCHAR(20) NOT NULL,
    sentiment VARCHAR(30) NOT NULL,
    suggested_team VARCHAR(100) NOT NULL,
    summary TEXT NOT NULL,
    suggested_action TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_triages_ticket_id
ON ticket_triages(ticket_id);