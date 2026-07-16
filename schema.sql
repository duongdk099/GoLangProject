CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    pseudo VARCHAR(50) NOT NULL CHECK (BTRIM(pseudo) <> ''),
    bio TEXT NOT NULL DEFAULT '',
    ville VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS skills (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nom VARCHAR(100) NOT NULL CHECK (BTRIM(nom) <> ''),
    niveau VARCHAR(20) NOT NULL CHECK (
        niveau IN ('débutant', 'intermédiaire', 'expert')
    ),
    PRIMARY KEY (user_id, nom)
);

CREATE INDEX IF NOT EXISTS idx_skills_nom_lower ON skills (LOWER(nom));

-- exchange_id deliberately remains nullable and has no foreign key until the
-- exchanges table owned by Person 3 is added.
CREATE TABLE IF NOT EXISTS credit_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exchange_id BIGINT NULL,
    montant INTEGER NOT NULL CHECK (montant <> 0),
    type VARCHAR(20) NOT NULL CHECK (type IN ('earn', 'spend', 'refund')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_transactions_user
    ON credit_transactions (user_id);

CREATE INDEX IF NOT EXISTS idx_credit_transactions_exchange
    ON credit_transactions (exchange_id)
    WHERE exchange_id IS NOT NULL;
