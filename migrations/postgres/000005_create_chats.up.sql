CREATE TYPE chat_type AS ENUM (
    'DIRECT',
    'GROUP',
    'SAVED'
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
        ),

    CONSTRAINT chats_saved_owner_required
        CHECK (
            type <> 'SAVED'
            OR created_by IS NOT NULL
        ),

    CONSTRAINT chats_saved_topics_enabled
        CHECK (
            type <> 'SAVED'
            OR topics_enabled = TRUE
        ),

    CONSTRAINT chats_saved_history_visible
        CHECK (
            type <> 'SAVED'
            OR history_visibility = 'VISIBLE'
        )
);

CREATE UNIQUE INDEX chats_saved_owner_unique_idx
    ON chats (created_by)
    WHERE type = 'SAVED';
