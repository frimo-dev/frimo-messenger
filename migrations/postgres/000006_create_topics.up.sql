CREATE TYPE topic_type AS ENUM (
    'GENERAL',
    'REGULAR'
);

CREATE TABLE topics
(
    id         UUID PRIMARY KEY,
    chat_id    UUID NOT NULL REFERENCES chats (id),
    type       topic_type NOT NULL,
    title      TEXT NOT NULL,

    created_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL,

    closed_at  TIMESTAMPTZ,

    CONSTRAINT topics_title_not_empty
        CHECK (length(btrim(title)) > 0),

    CONSTRAINT topics_closed_after_created
        CHECK (
            closed_at IS NULL
                OR closed_at >= created_at
            )
);

CREATE UNIQUE INDEX topics_one_general_per_chat_idx
    ON topics (chat_id)
    WHERE type = 'GENERAL';

CREATE INDEX topics_chat_id_idx
    ON topics (chat_id);