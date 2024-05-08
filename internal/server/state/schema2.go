package state

var schemaDB = `
CREATE TABLE IF NOT EXISTS deployments (
	name TEXT PRIMARY KEY,
	spec TEXT
);

CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	deployment TEXT,
	timestamp INTEGER,
	event TEXT
);
`
