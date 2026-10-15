CREATE TABLE auth_sessions
(
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users (id),

    device_name  TEXT NOT NULL,

    created_ip   INET,
    last_ip      INET,

    created_at   TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX auth_sessions_user_id_idx
    ON auth_sessions (user_id);


CREATE TABLE refresh_tokens
(
    id         UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES auth_sessions (id),

    token_hash BYTEA NOT NULL UNIQUE,

    created_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    request_id UUID,

    CHECK (
        (used_at IS NULL AND request_id IS NULL)
            OR
        (used_at IS NOT NULL AND request_id IS NOT NULL)
    )
);

CREATE INDEX refresh_tokens_session_id_idx
    ON refresh_tokens (session_id);
