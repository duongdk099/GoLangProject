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

-- Person 2 scope: service advertisements.
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

-- Person 2 scope: reviews left on completed exchanges.
-- exchange_id deliberately has no foreign key until the exchanges table
-- owned by Person 3 is added; service_id is a denormalized snapshot of the
-- reviewed exchange's service, captured at creation time through the
-- ExchangeLookup contract, so that service reviews can be listed without a
-- live join against the not-yet-existing exchanges table.
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
