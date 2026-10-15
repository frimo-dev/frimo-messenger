CREATE TYPE chat_member_role AS ENUM (
    'OWNER',
    'ADMIN',
    'MEMBER'
);

CREATE TYPE chat_member_status AS ENUM (
    'ACTIVE',
    'LEFT',
    'REMOVED'
);

CREATE TABLE chat_members (
    chat_id UUID NOT NULL REFERENCES chats(id),
    user_id UUID NOT NULL REFERENCES users(id),

    role chat_member_role NOT NULL DEFAULT 'MEMBER',
    status chat_member_status NOT NULL DEFAULT 'ACTIVE',

    joined_at TIMESTAMPTZ NOT NULL,
    left_at TIMESTAMPTZ,

    PRIMARY KEY (chat_id, user_id),

    CONSTRAINT chat_members_status_dates_check
        CHECK (
            (status = 'ACTIVE' AND left_at IS NULL)
            OR
            (status IN ('LEFT', 'REMOVED') AND left_at IS NOT NULL)
        ),

    CONSTRAINT chat_members_left_after_joined_check
        CHECK (
            left_at IS NULL
            OR left_at >= joined_at
        ),

    CONSTRAINT chat_members_inactive_role_check
        CHECK (
            status = 'ACTIVE'
            OR role = 'MEMBER'
        )
);

CREATE UNIQUE INDEX chat_members_one_owner_per_chat_idx
    ON chat_members (chat_id)
    WHERE role = 'OWNER'
      AND status = 'ACTIVE';

CREATE INDEX chat_members_active_user_idx
    ON chat_members (user_id, chat_id)
    WHERE status = 'ACTIVE';