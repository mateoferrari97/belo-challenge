CREATE TABLE users (
    id          UUID          PRIMARY KEY,
    name        TEXT          NOT NULL,
    email       TEXT          NOT NULL UNIQUE,
    balance     NUMERIC(20,2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE transactions (
    id              UUID          PRIMARY KEY,
    source_id       UUID          NOT NULL REFERENCES users(id),
    destination_id  UUID          NOT NULL REFERENCES users(id),
    amount          NUMERIC(20,2) NOT NULL CHECK (amount > 0),
    status          TEXT          NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CHECK (source_id <> destination_id)
);

CREATE TABLE transactions_log (
    id              UUID          PRIMARY KEY,
    transaction_id  UUID          NOT NULL REFERENCES transactions(id),
    user_id         UUID          NOT NULL REFERENCES users(id),
    direction       TEXT          NOT NULL,
    amount          NUMERIC(20,2) NOT NULL CHECK (amount > 0),
    balance_before  NUMERIC(20,2) NOT NULL CHECK (balance_before >= 0),
    balance_after   NUMERIC(20,2) NOT NULL CHECK (balance_after >= 0),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
