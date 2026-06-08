package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func Connect(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) Migrate() error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id             BIGSERIAL PRIMARY KEY,
    email          TEXT UNIQUE NOT NULL,
    password_hash  TEXT NOT NULL,
    first_name     TEXT NOT NULL DEFAULT '',
    last_name      TEXT NOT NULL DEFAULT '',
    phone          TEXT NOT NULL DEFAULT '',
    address        TEXT NOT NULL DEFAULT '',
    role           TEXT NOT NULL DEFAULT 'customer',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_verifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS password_resets (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT NOT NULL,
    token      TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notify_list (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS products (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT UNIQUE NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    price        NUMERIC(10,2) NOT NULL,
    discount_pct INT NOT NULL DEFAULT 0,
    stock        INT NOT NULL DEFAULT 0,
    image_url    TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT '',
    notes        TEXT[] NOT NULL DEFAULT '{}',
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    is_new       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT REFERENCES users(id),
    guest_email    TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending',
    total          NUMERIC(10,2) NOT NULL DEFAULT 0,
    payment_method TEXT NOT NULL DEFAULT '',
    ship_name      TEXT NOT NULL DEFAULT '',
    ship_line1     TEXT NOT NULL DEFAULT '',
    ship_line2     TEXT NOT NULL DEFAULT '',
    ship_city      TEXT NOT NULL DEFAULT '',
    ship_country   TEXT NOT NULL DEFAULT '',
    ship_postal    TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS order_items (
    id         BIGSERIAL PRIMARY KEY,
    order_id   BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id),
    quantity   INT NOT NULL,
    unit_price NUMERIC(10,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS tickets (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    subject    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ticket_messages (
    id         BIGSERIAL PRIMARY KEY,
    ticket_id  BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    sender     TEXT NOT NULL,
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_active  ON products(active);
CREATE INDEX IF NOT EXISTS idx_orders_user      ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_user     ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_ticket_msgs      ON ticket_messages(ticket_id);

ALTER TABLE users    ADD COLUMN IF NOT EXISTS phone   TEXT NOT NULL DEFAULT '';
ALTER TABLE users    ADD COLUMN IF NOT EXISTS address TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS discount_pct INT NOT NULL DEFAULT 0;
ALTER TABLE orders   ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT '';
ALTER TABLE orders   ADD COLUMN IF NOT EXISTS guest_email    TEXT NOT NULL DEFAULT '';
ALTER TABLE orders   ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE products ADD COLUMN IF NOT EXISTS is_new BOOLEAN NOT NULL DEFAULT FALSE;
`
