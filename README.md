# BarterSwap - API d'échange de compétences

BarterSwap est une API REST Go où des crédits-temps s'échangent contre des
services entre particuliers (une heure rendue = une heure reçue). Le dépôt
couvre l'ensemble du sujet : comptes et compétences (Person 1), annonces de
services, avis et statistiques (Person 2), cycle de vie des échanges et
journal de crédits (Person 3).

## Prérequis

- Go 1.22 ou supérieur
- PostgreSQL
- Docker Compose (optionnel, le plus rapide pour la base locale)

Le pilote PostgreSQL est la seule dépendance externe. L'API utilise
uniquement `net/http`, `encoding/json`, `context` et `database/sql` de la
stdlib — pas d'ORM, pas de framework HTTP.

## Installation

```bash
docker compose up -d postgres
go mod tidy
go run ./cmd/server
```

Le serveur écoute par défaut sur `http://localhost:8080` et applique le
schéma embarqué (`internal/database/schema.sql`) au démarrage.

| Variable | Défaut |
| --- | --- |
| `PORT` | `8080` |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable` |

## Authentification

Un header simple porte l'identité de l'utilisateur authentifié :

```text
X-User-ID: 1
```

Les lectures publiques (profils, services, avis, stats) ne le nécessitent
pas ; toute écriture (création, modification, action sur un échange) le
requiert.

## Collection Postman

`BarterSwap.postman_collection.json` (à la racine) couvre **tous** les
endpoints et rejoue automatiquement le scénario complet du sujet : création
de deux utilisateurs, compétence, service, demande d'échange, conflit `409`,
accept/reject/complete/cancel avec vérification des soldes de crédits, avis,
statistiques, et tous les cas d'erreur (`400`/`401`/`403`/`404`/`409`).

Import : Postman → **Import** → sélectionner le fichier. La variable de
collection `baseUrl` pointe vers `http://localhost:8080` ; les autres
variables (`aliceId`, `serviceId`, `exchangeId`...) se remplissent seules au
fil des requêtes. Lancez le dossier "Users" avant "Services", puis
"Exchanges", "Reviews" et "Statistics" — ou utilisez le **Collection
Runner** pour tout exécuter d'un coup.

Exécution en ligne de commande (Newman) :

```bash
npx newman run BarterSwap.postman_collection.json
```

## Endpoints

| Méthode | Chemin | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/healthz` | Non | Vérification de santé |
| `POST` | `/api/users` | Non | Créer un compte (10 crédits de bienvenue) |
| `GET` | `/api/users/{id}` | Non | Profil public, compétences, solde |
| `PUT` | `/api/users/{id}` | Propriétaire | Modifier son profil |
| `GET` | `/api/users/{id}/skills` | Non | Compétences d'un utilisateur |
| `PUT` | `/api/users/{id}/skills` | Propriétaire | Remplacer ses compétences |
| `GET` | `/api/services` | Non | Lister, filtrable par `categorie`, `ville`, `search` |
| `POST` | `/api/services` | Oui | Publier un service lié à une compétence |
| `GET` | `/api/services/{id}` | Non | Détail d'un service |
| `PUT` | `/api/services/{id}` | Propriétaire | Modifier son annonce |
| `DELETE` | `/api/services/{id}` | Propriétaire | Supprimer son annonce |
| `POST` | `/api/exchanges` | Oui | Demander un échange pour un service |
| `GET` | `/api/exchanges` | Oui | Lister ses échanges, filtrable par `status` |
| `GET` | `/api/exchanges/{id}` | Participant | Détail d'un échange |
| `PUT` | `/api/exchanges/{id}/accept` | Propriétaire | Accepter (bloque les crédits du demandeur) |
| `PUT` | `/api/exchanges/{id}/reject` | Propriétaire | Refuser une demande |
| `PUT` | `/api/exchanges/{id}/complete` | Demandeur | Confirmer (libère les crédits à l'offreur) |
| `PUT` | `/api/exchanges/{id}/cancel` | Participant | Annuler ; rembourse si déjà accepté |
| `POST` | `/api/exchanges/{id}/review` | Oui | Noter un échange terminé |
| `GET` | `/api/users/{id}/reviews` | Non | Avis reçus par un utilisateur |
| `GET` | `/api/services/{id}/reviews` | Non | Avis sur un service |
| `GET` | `/api/users/{id}/stats` | Non | Statistiques agrégées |

Niveaux de compétence : `débutant`, `intermédiaire`, `expert`. Catégories de
service : `Informatique`, `Jardinage`, `Bricolage`, `Cuisine`, `Musique`,
`Langues`, `Sport`, `Tutorat`, `Demenagement`, `Photographie`, `Animalier`,
`Couture`, `Autre`.

## Exemples curl

### Créer un utilisateur

```bash
curl -i -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"pseudo":"Alice","bio":"Développeuse Go","ville":"Paris"}'
```

### Publier un service

La catégorie doit correspondre à une compétence de l'utilisateur.

```bash
curl -i -X POST http://localhost:8080/api/services \
  -H "Content-Type: application/json" -H "X-User-ID: 1" \
  -d '{"titre":"Cours de Go","categorie":"Informatique","duree_minutes":120,"credits":2,"ville":"Paris"}'
```

### Cycle de vie complet d'un échange

Demandeur = user 2, offreur = user 1.

```bash
curl -i -X POST http://localhost:8080/api/exchanges -H "Content-Type: application/json" -H "X-User-ID: 2" -d '{"service_id":1}'
curl -i -X PUT http://localhost:8080/api/exchanges/1/accept   -H "X-User-ID: 1"   # bloque les crédits
curl -i -X PUT http://localhost:8080/api/exchanges/1/complete -H "X-User-ID: 2"   # les libère à l'offreur
```

### Noter l'échange terminé

```bash
curl -i -X POST http://localhost:8080/api/exchanges/1/review \
  -H "Content-Type: application/json" -H "X-User-ID: 2" \
  -d '{"note":5,"commentaire":"Ponctuel et de bon conseil"}'
```

Tous les autres cas (filtres, reject, cancel, erreurs 400/403/404/409...)
sont dans la [collection Postman](#collection-postman).

## Cycle de vie d'un échange et journal de crédits

```text
pending -> accepted -> completed
   |          |
   v          v
rejected   cancelled   (un échange pending peut aussi être annulé)
```

- **Create** : le demandeur vient de l'authentification ; l'offreur et le
  prix viennent du service, jamais du client. Impossible de se demander à
  soi-même, service inactif refusé, crédits insuffisants refusés, et un
  service ne peut avoir qu'un `pending`/`accepted` à la fois (sinon `409`).
- **Accept** (propriétaire) : bloque le prix chez le demandeur (`spend`).
- **Reject** (propriétaire) : rien n'était bloqué, rien n'est remboursé.
- **Complete** (**demandeur**, celui qui a reçu le service) : libère les
  crédits bloqués vers l'offreur (`earn`).
- **Cancel** (participant) : simple annulation si `pending` ; remboursement
  (`refund`) si `accepted`.

Toute transition qui déplace des crédits s'exécute dans **une transaction
SQL unique** qui verrouille la ligne (`FOR UPDATE`), revérifie l'état, écrit
le mouvement de crédit puis met à jour le statut : tout commit ou rollback
ensemble. **Aucun mutex Go** — la concurrence est gérée par PostgreSQL :

- un index unique partiel sur `exchanges(service_id)` fait échouer la
  seconde réservation simultanée d'un même service → `409` ;
- un index unique partiel sur `credit_transactions(exchange_id, type)`
  empêche qu'un débit/crédit/remboursement soit enregistré deux fois ;
- des gardes de statut (`WHERE status = <attendu>`) plus le verrou de ligne
  empêchent qu'une transition répétée déplace des crédits une seconde fois.

`internal/credits` est une petite bibliothèque (pas un feature HTTP) qui
porte le journal append-only ; un solde est la somme des entrées d'un
utilisateur, jamais une colonne mutable. Le crédit de bienvenue utilise le
même contrat (`earn`, `exchange_id` `NULL`).

## Format des erreurs

```json
{"error": {"code": "validation_error", "message": "validation error: pseudo is required"}}
```

Les sentinelles (`httpapi.ErrValidation`, `ErrUnauthorized`, `ErrForbidden`,
`ErrNotFound`, `ErrConflict`) se traduisent en `400`, `401`, `403`, `404`,
`409` ou `500`.

## Tests

```bash
go test -v -cover ./...
go vet ./...
```

Les tests Postgres (stores réels, transactions) sont optionnels et exclus
du run par défaut :

```bash
RUN_POSTGRES_INTEGRATION=1 \
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable' \
go test ./... -run Integration -p 1 -v
```

Lancez les packages d'intégration en série avec `-p 1` : le test d'intégration
de chaque package applique le schéma, et `CREATE TABLE IF NOT EXISTS` de
PostgreSQL n'est pas sûr en exécution concurrente (les migrations parallèles
entrent en collision sur `pg_type`). Chaque test crée un petit nombre de
lignes temporaires et les supprime avant de terminer. Utilisez une base de
développement, jamais une base de production.

Couverture combinée (tests unitaires + intégration) : **~79%**, au-dessus du
seuil de 60% demandé. Les instructions restantes non couvertes sont
essentiellement des gardes défensives d'échec base de données
(`return ... err` sur un `Commit`/`Scan`/verrou qui n'échoue pas sur une
connexion saine), inatteignables sans mock du pilote SQL.

## Architecture

Le dépôt suit la structure `cmd/` + `internal/` + `pkg/` vue en cours
(Module 8) : un point d'entrée fin, un package Go par feature sous
`internal/` (non importable hors du module), et un package réutilisable
sous `pkg/`.

```text
cmd/server/main.go     # câble config, base de données, et chaque feature
internal/
├── config/, database/, testutil/   # infrastructure commune
├── users/       # Person 1 — comptes, compétences, crédits de bienvenue
├── services/    # Person 2 — annonces de services
├── reviews/     # Person 2 — avis sur échanges terminés
├── stats/       # Person 2 — tableau de bord agrégé
├── exchanges/   # Person 3 — cycle de vie des échanges (transactionnel)
└── credits/     # Person 3 — journal de crédits append-only (bibliothèque)
pkg/httpapi/      # HTTP réutilisable : erreurs, JSON, middlewares, auth, routing
```

Chaque feature suit le même découpage que `internal/users` :

```text
HTTP request -> middleware (pkg/httpapi) -> handler.go -> service.go -> store_postgres.go -> PostgreSQL
```

`pkg/httpapi` ne dépend d'aucun type métier BarterSwap — seulement de
`net/http` — ce qui le rend réutilisable par toutes les features sans cycle
d'import.

### Dépendances entre features

Chaque dépendance croisée est une petite interface déclarée côté
consommateur, satisfaite implicitement par le type concret du fournisseur
(même principe de duck typing que le cours, appliqué au travers d'une
frontière de package) :

- `services.SkillChecker` / `stats.UserExistenceChecker` sont satisfaites
  par `*users.Service` sans import (signatures en types de base uniquement).
- `reviews.ServiceExistenceChecker` et `exchanges.ServiceLookup` sont
  satisfaites par `*services.UseCases` (import du type `Service` uniquement,
  à sens unique).
- `reviews.ExchangeLookup` et `stats.ExchangeStatsProvider` sont satisfaites
  par `*exchanges.UseCases`.
- `exchanges.RequesterChecker` est satisfaite par `*users.Service`.

Seul `cmd/server/main.go` importe toutes les features pour les câbler
ensemble ; aucune feature n'importe le type concret d'une autre en dehors de
ces contrats d'interface.
