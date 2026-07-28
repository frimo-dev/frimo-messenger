CREATE TABLE email_verifications (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash BYTEA NOT NULL UNIQUE,
	expires_at TIMESTAMPTZ NOT NULL,
	used_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
    token_ciphertext BYTEA NOT NULL
);

CREATE INDEX email_verifications_user_id_idx ON email_verifications (user_id)