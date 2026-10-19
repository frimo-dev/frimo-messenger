CREATE TABLE users (
	id UUID PRIMARY KEY,
	email TEXT NOT NULL,
    username TEXT,
    name TEXT NOT NULL,
    surname TEXT,
	password_hash TEXT NOT NULL,
	email_verified_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX users_email_unique_idx ON users (lower(email));

CREATE UNIQUE INDEX users_username_unique_idx
    ON users (username)
    WHERE username IS NOT NULL;