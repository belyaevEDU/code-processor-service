-- +migrate Up
CREATE TYPE task_status AS ENUM ('in_progress', 'ready');

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    login         text NOT NULL UNIQUE,
    password_hash text NOT NULL
);

CREATE TABLE tasks (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users (id),
    status      task_status NOT NULL DEFAULT 'in_progress',
    translator  text NOT NULL,
    result      jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

-- +migrate Down
DROP TABLE tasks;
DROP TABLE users;
DROP TYPE task_status;
