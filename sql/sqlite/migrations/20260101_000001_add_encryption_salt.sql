-- Add encryption_salt column for proper key derivation
-- Run this migration before upgrading to the new version

ALTER TABLE licenses ADD COLUMN encryption_salt TEXT;

-- Generate salt for existing licenses (SQLite)
-- Note: This should be run manually or via migration tool
-- UPDATE licenses SET encryption_salt = hex(randomblob(32)) WHERE encryption_salt IS NULL;
