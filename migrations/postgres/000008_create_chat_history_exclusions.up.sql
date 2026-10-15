CREATE TYPE chat_history_exclusion_kind AS ENUM (
    'SELF_ABSENCE',
    'REVOCABLE'
);

CREATE TABLE chat_history_exclusions (
    id UUID PRIMARY KEY,

    chat_id UUID NOT NULL,
    user_id UUID NOT NULL,

    kind chat_history_exclusion_kind NOT NULL,

    from_boundary_id UUID,
    to_boundary_id UUID,

    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT chat_history_exclusions_member_fk
        FOREIGN KEY (chat_id, user_id)
        REFERENCES chat_members(chat_id, user_id),

    CONSTRAINT chat_history_exclusions_boundaries_check
        CHECK (
            from_boundary_id IS NOT NULL
            OR to_boundary_id IS NOT NULL
        ),

    CONSTRAINT chat_history_exclusions_order_check
        CHECK (
            from_boundary_id IS NULL
            OR to_boundary_id IS NULL
            OR from_boundary_id <= to_boundary_id
        ),

    CONSTRAINT chat_history_exclusions_self_absence_check
        CHECK (
            kind <> 'SELF_ABSENCE'
            OR from_boundary_id IS NOT NULL
        )
);

CREATE INDEX chat_history_exclusions_member_idx
    ON chat_history_exclusions (chat_id, user_id);

CREATE UNIQUE INDEX chat_history_exclusions_one_open_idx
    ON chat_history_exclusions (chat_id, user_id)
    WHERE to_boundary_id IS NULL;
