CREATE TABLE users (
	id UUID PRIMARY KEY,
	email TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	email_verified_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX users_email_unique_idx ON users (lower(email));