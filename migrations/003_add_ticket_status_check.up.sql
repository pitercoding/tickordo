ALTER TABLE tickets
ADD CONSTRAINT tickets_status_check
CHECK (status IN ('open', 'in_progress', 'resolved'));
