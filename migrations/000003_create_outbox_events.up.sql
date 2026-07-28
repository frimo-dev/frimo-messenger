CREATE TABLE outbox_events (
	id UUID PRIMARY KEY,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL,

	created_at TIMESTAMPTZ NOT NULL,
	available_at TIMESTAMPTZ NOT NULL,

	locked_at TIMESTAMPTZ,
	processed_at TIMESTAMPTZ,
	failed_at TIMESTAMPTZ,

	attempts INTEGER NOT NULL DEFAULT 0,
	last_error TEXT,

	CONSTRAINT outbox_events_attempts_non_negative
		CHECK (attempts >= 0),

	CONSTRAINT outbox_events_payload_object
		CHECK (jsonb_typeof(payload) = 'object'),
);

CREATE INDEX outbox_events_pending_idx
	ON outbox_events (available_at, created_at)
	WHERE processed_at IS NULL
	AND failed_at IS NULL;