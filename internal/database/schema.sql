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

-- One exchange has at most one entry of each type (one spend, one earn, one
-- refund), so a debit, earning, or refund can never be recorded twice for the
-- same exchange and operation. Welcome credits use a NULL exchange_id and are
-- excluded by the partial predicate, so several users may still receive them.
CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_transactions_exchange_type
    ON credit_transactions (exchange_id, type)
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

-- Person 3 scope: exchange lifecycle. credits_cost is a snapshot of the
-- service price taken when the request is created, so the amount blocked on
-- acceptance is exactly the amount released on completion or refunded on
-- cancellation even if the provider later edits the service price. It is
-- internal bookkeeping and is not part of the public Exchange JSON.
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

-- A service may hold at most one live reservation at a time. This partial
-- unique index makes two simultaneous requests for the same service race at
-- the database level: the second INSERT fails with a unique violation, which
-- the store maps to 409 Conflict. No Go mutex is involved.
CREATE UNIQUE INDEX IF NOT EXISTS idx_exchanges_active_service
    ON exchanges (service_id)
    WHERE status IN ('pending', 'accepted');
