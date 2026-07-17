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

CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_transactions_exchange_type
    ON credit_transactions (exchange_id, type)
    WHERE exchange_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS services (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    titre VARCHAR(150) NOT NULL CHECK (BTRIM(titre) <> ''),
    description TEXT NOT NULL DEFAULT '',
    categorie VARCHAR(30) NOT NULL CHECK (categorie IN (
        'Informatique', 'Jardinage', 'Bricolage', 'Cuisine', 'Musique',
        'Langues', 'Sport', 'Tutorat', 'Demenagement', 'Photographie',
        'Animalier', 'Couture', 'Autre'
    )),
    duree_minutes INTEGER NOT NULL CHECK (duree_minutes > 0),
    credits INTEGER NOT NULL CHECK (credits > 0),
    ville VARCHAR(100) NOT NULL DEFAULT '',
    actif BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_services_provider ON services (provider_id);
CREATE INDEX IF NOT EXISTS idx_services_categorie ON services (categorie);
CREATE INDEX IF NOT EXISTS idx_services_ville ON services (LOWER(ville));

CREATE TABLE IF NOT EXISTS reviews (
    id BIGSERIAL PRIMARY KEY,
    exchange_id BIGINT NOT NULL,
    service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note INTEGER NOT NULL CHECK (note BETWEEN 1 AND 5),
    commentaire TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (exchange_id, author_id)
);

CREATE INDEX IF NOT EXISTS idx_reviews_target ON reviews (target_id);
CREATE INDEX IF NOT EXISTS idx_reviews_service ON reviews (service_id);

CREATE TABLE IF NOT EXISTS exchanges (
    id BIGSERIAL PRIMARY KEY,
    service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    requester_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credits_cost INTEGER NOT NULL CHECK (credits_cost > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'accepted', 'completed', 'rejected', 'cancelled')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (requester_id <> owner_id)
);

CREATE INDEX IF NOT EXISTS idx_exchanges_requester ON exchanges (requester_id);
CREATE INDEX IF NOT EXISTS idx_exchanges_owner ON exchanges (owner_id);
CREATE INDEX IF NOT EXISTS idx_exchanges_service ON exchanges (service_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_exchanges_active_service
    ON exchanges (service_id)
    WHERE status IN ('pending', 'accepted');
