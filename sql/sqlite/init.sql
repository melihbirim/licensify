-- SQLite initialization schema
-- This file creates all necessary tables for Licensify

CREATE TABLE IF NOT EXISTS licenses (
	license_id TEXT PRIMARY KEY,
	customer_name TEXT NOT NULL,
	customer_email TEXT NOT NULL,
	tier TEXT NOT NULL DEFAULT 'free',
	expires_at TEXT NOT NULL,
	daily_limit INTEGER NOT NULL,
	monthly_limit INTEGER NOT NULL,
	max_activations INTEGER NOT NULL,
	active INTEGER DEFAULT 1,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS activations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	license_id TEXT NOT NULL,
	hardware_id TEXT NOT NULL,
	activated_at TEXT DEFAULT CURRENT_TIMESTAMP,
	last_check_in TEXT DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (license_id) REFERENCES licenses(license_id)
);

CREATE TABLE IF NOT EXISTS verification_codes (
	email TEXT PRIMARY KEY,
	code TEXT NOT NULL,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS daily_usage (
	license_id TEXT NOT NULL,
	date TEXT NOT NULL,
	count INTEGER DEFAULT 0,
	PRIMARY KEY (license_id, date)
);

CREATE TABLE IF NOT EXISTS check_ins (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	license_id TEXT NOT NULL,
	last_check_in TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS proxy_keys (
	license_id TEXT PRIMARY KEY,
	openai_key TEXT,
	anthropic_key TEXT,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
