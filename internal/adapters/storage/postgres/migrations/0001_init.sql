-- +goose Up
CREATE TABLE users (
    id         UUID PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    pass_hash  BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE apps (
    id     INTEGER PRIMARY KEY,
    name   TEXT NOT NULL,
    secret TEXT NOT NULL
);

CREATE TABLE user_roles (
    user_id UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id  INTEGER NOT NULL REFERENCES apps(id)  ON DELETE CASCADE,
    role    TEXT    NOT NULL,
    PRIMARY KEY (user_id, app_id, role)
);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY,
    user_id    UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id     INTEGER NOT NULL REFERENCES apps(id)  ON DELETE CASCADE,
    token_hash BYTEA   NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens(user_id);

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE user_roles;
DROP TABLE apps;
DROP TABLE users;
