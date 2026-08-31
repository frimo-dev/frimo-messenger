CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY,
    type TEXT NOT NULL,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL,

    CHECK (type IN ('direct', 'group', 'channel'))
);

CREATE TABLE IF NOT EXISTS chat_members (
    chat_id UUID NOT NULL REFERENCES chats(id),
    user_id UUID NOT NULL REFERENCES users(id),

    joined_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,

    PRIMARY KEY (chat_id, user_id),

    CHECK (status IN ('active', 'left'))
);
CREATE INDEX IF NOT EXISTS chat_members_user_id_idx
    ON chat_members (user_id, chat_id);

CREATE TABLE IF NOT EXISTS direct_chats (
    chat_id UUID PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,

    user_low_id UUID NOT NULL REFERENCES users(id),
    user_high_id UUID NOT NULL REFERENCES users(id),

    CHECK (user_low_id < user_high_id),

    UNIQUE (user_low_id, user_high_id)
);