# Belo Challenge

**Stack:**
- Go
- PostgreSQL

## Requirements
- Docker

## Quick Start

```bash
git clone <repo-url>
cd belo-challenge
docker compose up --build
```

Services:
1. `api`: HTTP server on port `:8080`.
2. `migrate`: applies schema migrations and seeds initial users.
3. `postgres`: persists users and transactions.

Smoke test:

```bash
curl http://localhost:8080/ping
# pong
```

## Swagger

Once the stack is running, browse to <http://localhost:8080/swagger/index.html>.

## API Walkthrough

The application seeds three users. Use their IDs to run the examples below.
The review threshold defaults to `50000.00`, transfers above that amount are created
as `pending` and must be explicitly approved or rejected.

| Name  | ID                                     | Initial balance |
|-------|----------------------------------------|-----------------|
| Alice | `018f3a7b-0000-7000-8000-000000000001` | `100000.00`     |
| Bob   | `018f3a7b-0000-7000-8000-000000000002` | `5000.00`       |
| Carla | `018f3a7b-0000-7000-8000-000000000003` | `50.00`         |

### Create a transaction

Moves balances immediately when `amount` is below the threshold.

```bash
curl -X POST http://localhost:8080/v1/transactions \
  -H 'Content-Type: application/json' \
  -d '{
    "source_id":      "018f3a7b-0000-7000-8000-000000000001",
    "destination_id": "018f3a7b-0000-7000-8000-000000000002",
    "amount":         "1000.00"
  }'
```

### Create a transaction that needs review

Same endpoint with an amount above the threshold. No balances move; the transaction is
stored as `pending`. Save the returned `id`, you'll need it to approve or reject.

```bash
curl -X POST http://localhost:8080/v1/transactions \
  -H 'Content-Type: application/json' \
  -d '{
    "source_id":      "018f3a7b-0000-7000-8000-000000000001",
    "destination_id": "018f3a7b-0000-7000-8000-000000000002",
    "amount":         "60000.00"
  }'
```

### List a user's transactions

Paginated with a cursor based strategy. Pass `cursor=<value>` on subsequent pages.

```bash
curl "http://localhost:8080/v1/transactions?userId=018f3a7b-0000-7000-8000-000000000001"
```

### Approve a pending transaction

Debits source, credits destination, marks the transaction as `approved`.

```bash
curl -X PATCH http://localhost:8080/v1/transactions/<id>/approve
```

### Reject a pending transaction

Marks the transaction as `rejected`. Balances are not touched. Terminal state.

```bash
curl -X PATCH http://localhost:8080/v1/transactions/<id>/reject
```

## Project Structure

```
.
├── cmd/
│   └── web/
│       ├── handler/          web application request handlers
│       └── server/           web application entry point
├── internal/
│   ├── platform/
│   │   ├── database/         foundational database access
│   │   └── web/              foundational web framework
│   ├── transaction/          transaction business domain
│   └── user/                 user business domain
├── migrations/               database schema and seed data
├── docs/api/                 generated API specification
├── docker-compose.yml
├── Dockerfile
└── Makefile
```