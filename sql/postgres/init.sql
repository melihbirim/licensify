-- PostgreSQL initialization schema
-- This file creates all necessary tables for Licensify

CREATE TABLE IF NOT EXISTS licenses (
	license_id TEXT PRIMARY KEY,
	customer_name TEXT NOT NULL,
	customer_email TEXT NOT NULL,
	tier TEXT NOT NULL DEFAULT 'free',
	expires_at TIMESTAMP NOT NULL,
	daily_limit INTEGER NOT NULL,
	monthly_limit INTEGER NOT NULL,
	max_activations INTEGER NOT NULL,
	active BOOLEAN DEFAULT true,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS activations (
	id SERIAL PRIMARY KEY,
	license_id TEXT NOT NULL REFERENCES licenses(license_id),
	hardware_id TEXT NOT NULL,
	activated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	last_check_in TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS verification_codes (
	email TEXT PRIMARY KEY,
	code TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS daily_usage (
	license_id TEXT NOT NULL,
	date DATE NOT NULL,
	count INTEGER DEFAULT 0,
	PRIMARY KEY (license_id, date)
);

CREATE TABLE IF NOT EXISTS check_ins (
	id SERIAL PRIMARY KEY,
	license_id TEXT NOT NULL,
	last_check_in TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS proxy_keys (
	license_id TEXT PRIMARY KEY,
	openai_key TEXT,
	anthropic_key TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
