# Sacrop Customer API — Go

Go port of the Express API in the parent directory. It reads customer data
from the local MySQL database imported from the WordPress/WooCommerce SQL
dump. It does not call the live website.

Endpoints, query parameters, status codes and JSON response shapes match the
Node version.

## Requirements

- Go 1.22 or newer (`brew install go`)
- MySQL with the Sacrop WordPress database imported

The database tables in the provided dump use the `wp5u_` prefix, including
`wp5u_wc_customer_lookup`, `wp5u_wc_order_stats`, and `wp5u_usermeta`.

## Configuration

Create a `.env` file in this directory (one is already present; `.env.example`
is the committable template):

```dotenv
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=root
DB_PASSWORD=your_mysql_password
DB_DATABASE=sacrop
DB_TIMEZONE=UTC
PORT=3000
```

Real environment variables take precedence over `.env`. Start MySQL before
starting the API; the connection is verified at startup and the process exits
with a non-zero status if it fails.

## Install and run

```sh
go mod tidy          # downloads dependencies, writes go.sum
make swag-install    # installs the swag generator (once)
go run ./cmd/server
```

Or with the Makefile: `make tidy && make run`. To produce a binary:

```sh
make build && ./bin/server
```

The server listens at `http://localhost:3000` by default and shuts down
gracefully on SIGINT/SIGTERM.

## API documentation

Swagger UI is served at **http://localhost:3000/swagger/index.html** (`/swagger`
redirects there). The UI assets are embedded in the binary, so the page works
without internet access.

The spec itself is generated from the `@`-annotations in the source by
[swaggo/swag](https://github.com/swaggo/swag) and committed under `docs/`:

| File | Purpose |
| --- | --- |
| `docs/docs.go` | Registered with the UI at startup |
| `docs/swagger.json` | Also served at `/swagger/doc.json` |
| `docs/swagger.yaml` | For import into Postman, Insomnia, etc. |

Regenerate after changing an annotation:

```sh
make docs
```

`make run` and `make build` do this for you. The output is Swagger 2.0
(OpenAPI 2.0), which is what swag v1 emits.

## Endpoints

### `GET /api/customers`

Customer profiles with completed/processing order count and total spend, one
row per unique email address.

```text
http://localhost:3000/api/customers?limit=1000&offset=0
http://localhost:3000/api/customers?limit=all
```

`total` is the number of unique customers in the database (12,033 in the
supplied dump), not the size of the current page.

### `GET /api/customers/profiles`

Customer profile data without order information.

```text
http://localhost:3000/api/customers/profiles?limit=1000&offset=0
http://localhost:3000/api/customers/profiles?limit=all
```

### `GET /api/customers/:id`

One customer by WooCommerce customer ID, including order summary.

```text
http://localhost:3000/api/customers/1471
```

### `GET /health`

Basic API health response. This confirms the HTTP server is responding; it
does not independently verify the database after startup.

## Pagination

Both list endpoints accept:

- `limit`: rows per response; defaults to `50`, minimum `1`, maximum `50000`.
  Both endpoints also accept `limit=all`, which returns every row.
- `offset`: number of rows to skip; defaults to `0`.

The response includes `total`, `limit`, and `offset` fields.

- `/api/customers` → `total` is the count of unique customers across the whole
  database, so paging with `offset` walks the full set without repeating or
  skipping anyone.
- `/api/customers/profiles` → `total` is the full row count of
  `wp5u_wc_customer_lookup` (12,578), which includes duplicate emails.

The two differ because `wp5u_wc_customer_lookup` holds 12,578 rows, of which 12
have no email and the rest collapse to 12,033 unique addresses.

## Layout

```
cmd/server/main.go          entry point, routing, graceful shutdown
internal/config             environment + .env loading
internal/database           MySQL pool and startup ping
internal/httpx              JSON helpers, CORS, recover, 404
internal/customer/model.go  response structs and null handling
internal/customer/repository.go  SQL queries
internal/customer/handler.go     HTTP handlers and pagination
```

## Timestamps

`DB_TIMEZONE` controls the zone `DATETIME` columns are read in; responses are
always rendered as UTC ISO-8601 (`2006-01-02T15:04:05.000Z`).

- `UTC` (default) treats stored values as UTC, which is how WordPress writes
  them.
- `Local` reproduces the Node version exactly: `mysql2` interprets `DATETIME`
  in the server's local zone, shifting the rendered value by the local UTC
  offset.
- Any IANA name (e.g. `Asia/Dhaka`) also works.

With `DB_TIMEZONE=Local`, responses from this server are byte-identical to the
Node server's for every endpoint, including JSON key order.

## Differences from the Node version

`/api/customers` de-duplicates differently, and deliberately:

- The Node version selects a page of rows and then de-duplicates by email
  *within that page*, so `total` is the page size after de-duplication (never
  more than `limit`), the same customer can appear on two different pages, and
  no request can reach past the 10,000-row cap.
- This version de-duplicates in SQL with `ROW_NUMBER() OVER (PARTITION BY
  LOWER(TRIM(email)))`, keeping the row with the most orders (ties broken by
  the highest `customer_id`) — the same row the Node code would have kept had
  it seen every candidate. `limit`/`offset` then page the de-duplicated set,
  and `total` is the true number of unique customers.

Requires MySQL 8.0 or newer for the CTE and window function.

Other differences:

- The `limit` cap is `50000` rather than `10000`, and `limit=all` is accepted.
- `?limit=20abc` falls back to the default here, where `parseInt` in Node
  would read `20`. Well-formed numbers behave identically.
- `/api/customers/:id` requires an integer; Node accepted any numeric string
  including floats.
