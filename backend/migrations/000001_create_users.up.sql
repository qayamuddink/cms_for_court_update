-- Migration: 000001_create_users.up.sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    password_hash TEXT,
    auth_provider VARCHAR(32) NOT NULL DEFAULT 'local',
    full_name VARCHAR(255) NOT NULL,
    display_role VARCHAR(32) NOT NULL DEFAULT 'public',
    system_role VARCHAR(32) NOT NULL DEFAULT 'reader',
    bar_council_number VARCHAR(64),
    phone VARCHAR(32),
    email_verified_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

CREATE UNIQUE INDEX idx_users_email_lower ON users (LOWER(email));
CREATE UNIQUE INDEX idx_users_phone_unique ON users (phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_users_system_role ON users (system_role);
