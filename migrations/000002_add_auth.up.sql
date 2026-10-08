ALTER TABLE users
    MODIFY COLUMN password_hash VARCHAR(255) NULL,
    ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE' AFTER password_hash,
    ADD COLUMN email_verified_at DATETIME(3) NULL AFTER status;

CREATE TABLE auth_identities (
    id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    provider VARCHAR(24) NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    provider_email VARCHAR(255) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_auth_identity_provider_subject (provider, provider_subject),
    KEY idx_auth_identities_user (user_id),
    CONSTRAINT fk_auth_identities_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE auth_sessions (
    id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    refresh_token_hash VARCHAR(128) NOT NULL,
    device_id VARCHAR(255) NULL,
    device_name VARCHAR(255) NULL,
    platform VARCHAR(32) NULL,
    expires_at DATETIME(3) NOT NULL,
    last_used_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    revoke_reason VARCHAR(64) NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_auth_sessions_user (user_id),
    KEY idx_auth_sessions_expiry (expires_at),
    CONSTRAINT fk_auth_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE password_resets (
    id CHAR(36) NOT NULL,
    email VARCHAR(255) NOT NULL,
    otp_hash VARCHAR(128) NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    expires_at DATETIME(3) NOT NULL,
    verified_at DATETIME(3) NULL,
    reset_ticket_hash VARCHAR(128) NULL,
    reset_ticket_expires_at DATETIME(3) NULL,
    consumed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_password_resets_email_created (email, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE terms_acceptances (
    id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    terms_version VARCHAR(64) NOT NULL,
    accepted_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_terms_acceptance_user_version (user_id, terms_version),
    CONSTRAINT fk_terms_acceptances_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
