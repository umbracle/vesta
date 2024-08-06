
CREATE TABLE IF NOT EXISTS events (
    id TEXT,
    deployment_id TEXT NOT NULL REFERENCES deployments (id),
    task TEXT NOT NULL,
    type TEXT NOT NULL
);
