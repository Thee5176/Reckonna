-- 005_idempotency.up.sql — idempotency records for POST /command/journal-entries
-- (plan 03 S8b). Composite PK (key, owner_sub) scopes a key to its owner so two
-- clients cannot collide. request_hash lets a replay with a different body be
-- rejected (422 duplicate_idempotency_key). created_at drives 24h TTL cleanup.

CREATE TABLE idempotency_record (
    key             text NOT NULL,
    owner_sub       text NOT NULL,
    request_hash    text NOT NULL,
    response_status int  NOT NULL,
    response_body   jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (key, owner_sub)
);

CREATE INDEX idempotency_record_created_at_idx ON idempotency_record (created_at);
