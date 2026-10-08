DROP TABLE IF EXISTS terms_acceptances;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS auth_identities;
ALTER TABLE users
    DROP COLUMN email_verified_at,
    DROP COLUMN status,
    MODIFY COLUMN password_hash VARCHAR(255) NOT NULL;
