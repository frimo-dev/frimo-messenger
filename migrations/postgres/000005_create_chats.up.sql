CREATE TYPE chat_type AS ENUM (
    'DIRECT',
    'GROUP'
);

CREATE TYPE chat_history_visibility AS ENUM (
    'VISIBLE',
    'HIDDEN'
);

CREATE TABLE chats (
    id UUID PRIMARY KEY,

    type chat_type NOT NULL,

    title TEXT,
    description TEXT,

    topics_enabled BOOLEAN NOT NULL DEFAULT FALSE,

    history_visibility chat_history_visibility
        NOT NULL DEFAULT 'VISIBLE',

    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT chats_direct_topics_disabled
        CHECK (
            type <> 'DIRECT'
            OR topics_enabled = FALSE
        ),

    CONSTRAINT chats_direct_title_null
        CHECK (
            type <> 'DIRECT'
            OR title IS NULL
        ),

    CONSTRAINT chats_group_title_required
        CHECK (
            type <> 'GROUP'
            OR (
                title IS NOT NULL
                AND length(btrim(title)) > 0
            )
        ),

    CONSTRAINT chats_direct_history_visible
        CHECK (
            type <> 'DIRECT'
            OR history_visibility = 'VISIBLE'
        )
);