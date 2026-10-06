# angel

A robust Go-based service template designed for quick bootstrapping of web services. It includes built-in support for system service management, request logging to SQLite, MySQL or FileMaker, authentication, and Docker deployment.

## API Endpoints

Every response carries an `X-Version` header with the build version. Errors are returned as plain text with the HTTP status text as the body (for example `Not Found`). A request that runs longer than `TIMEOUT` seconds gets **503** with the body `Timeout`.

### Public Endpoints

- **GET /**
  - Entry screen: a numeric PIN entry screen (Swedish, "Ange PIN-kod") for the Caregiver PIN or Master PIN. With a valid session it redirects (**303**) to `/note` or `/admin` instead.

- **POST /pin**
  - Form field `pin`. The Caregiver PIN starts a caregiver session and redirects (**303**) to `/note`; the Master PIN starts a client session and redirects to `/admin`. A wrong PIN shows the entry screen again with an error (**401**).
  - Rate limited: after **5** wrong PINs from one client address within **15 minutes**, that address gets **429** (without the PIN being checked) until the 15 minutes have passed. See `TRUSTED_PROXIES` for running behind a proxy.
  - PIN Alert: the rate limit is per address, so guessing from many addresses is not blocked. Instead, after **20** wrong PINs from any addresses within **24 hours**, the Client dashboard shows a PIN Alert with the number of wrong PINs and when the first and last were entered, asking the Client to change the Caregiver PIN, and the service logs a warning. Attempts refused by the rate limit don't count. The alert is kept in memory and stays until the service restarts, which changing the Caregiver PIN requires. Caregivers never see it. See [ADR-0005](docs/adr/0005-pin-alert-instead-of-global-cap.md).
  - Never request-logged, so PINs are not stored.

- **POST /logout**
  - The "Logga ut" button at the top of `/note` and `/admin`. Deletes the session cookie and redirects (**303**) to `/`, so the entry screen shows again and either PIN can be entered on the same phone. Needs no session; without one it just redirects. `GET /logout` gets **405**, so a link prefetch cannot log anyone out.
  - Sessions are stateless, so this ends the session on the device that logs out. A copy of the cookie kept elsewhere stays valid until it expires (`CAREGIVER_SESSION_TIMEOUT` or `CLIENT_SESSION_TIMEOUT`).
  - There is no CSRF token, so another site could log a visitor out with a hidden form. That is only a nuisance (enter the PIN again) and is accepted.

- **GET /note**, **GET /admin**
  - The caregiver view and the client dashboard. They need a session for that role; anyone else is redirected (**303**) to `/`.
  - `/note` shows today's Daily Note (the calendar date in Europe/Stockholm), rendered from Markdown to HTML, in a high-contrast red box when it has the Important Flag, or "Inga särskilda instruktioner idag. Allt är som vanligt!" when there is none.
  - Notes are written in Markdown (CommonMark, rendered with [goldmark](https://github.com/yuin/goldmark)). A single line break stays a line break, so plain-text notes look as typed. Raw HTML in a note is left out and `javascript:` links are removed, so a note cannot run scripts on caregivers' phones.
  - `/admin` is the client's editor for today's Daily Note: a multiline text box for the Markdown source, the Important Flag ("Viktigt") checkbox, the time it was last saved and a button to clear it. Between them is "Kvitteringar idag", today's Acknowledgements newest first ("Maria kl 08:35", or "Okänd ängel kl 12:05" without a name), or "Inga kvitteringar registrerade idag än." when there are none. Below it is the same editor for tomorrow's Advance Note ("Morgondagens anteckning").
  - The feed refreshes every 30 seconds. When a refresh finds that the client session has expired, a banner at the top says "Sessionen har gått ut – kopiera din text innan du loggar in igen" and the save and clear buttons are disabled, so unsaved text is not lost to the entry screen. The text boxes stay editable for copying. Drafts are not kept in browser storage, because the device may be shared.
  - Sessions are a signed, HTTP-only `angel_session` cookie (`SameSite=Lax`, `Secure` unless `COOKIE_SECURE=false`) that carries the role and expiry. A caregiver session lasts `CAREGIVER_SESSION_TIMEOUT` and a client session `CLIENT_SESSION_TIMEOUT`. The expiry is fixed at login and never extended, so the feed refresh on `/admin` cannot keep a session alive. The server checks the expiry too, so an old cookie is useless once it has expired.

- **POST /note/ack**
  - The "Kvittera" button on `/note`. Records an Acknowledgement of today's Daily Note with the time and the optional form field `name` (the caregiver's first name, trimmed and capped at 40 characters; blank means anonymous). Every submission is a new Acknowledgement, so each visit during the day records its own.
  - Redirects (**303**) to `/note`, which shows "Kvitterat av Maria kl 08:35" (or "Kvitterat kl 08:35" without a name). The confirmation travels in a one-time `angel_ack` cookie (HTTP-only, `Path=/note`, one minute) that `/note` clears once shown, so the next caregiver on a shared phone is not told the note is already acknowledged. It only appears for an Acknowledgement made today.
  - Needs a caregiver session; anyone else, including the client, is redirected (**303**) to `/`.

- **GET /admin/acks**
  - Only the PIN Alert (when it has triggered) and the "Kvitteringar idag" list from `/admin`, as an HTML fragment; each part is marked with the `data-target` of the element it replaces. The dashboard fetches it every 30 seconds, and when the tab becomes visible again, so new Acknowledgements appear without reloading the page and losing unsaved editor text. It shows only today's Acknowledgements (the calendar date in Europe/Stockholm), so the list starts empty at midnight.
  - Needs a client session; anyone else is redirected (**303**) to `/`. When the dashboard's fetch gets that redirect, the session has ended: the list is replaced with "Sessionen har gått ut. Logga in igen för att se nya kvitteringar." and polling stops.

- **POST /admin/note**, **POST /admin/note/clear**
  - The editor on `/admin`. `/admin/note` saves today's Daily Note from the form fields `text` (line endings normalized, trimmed, at most 5000 characters, else **400**) and `important` (any value sets the Important Flag). Saving blank text clears the note. `/admin/note/clear` removes today's note, so caregivers see the empty state again.
  - Both also need the form field `date`: the day (`YYYY-MM-DD`) the editor was loaded for. If it is not today, for example because the dashboard was opened before midnight and submitted after it, nothing changes and the response is **409**, so a stale page cannot overwrite or delete the note that has just rolled over.
  - Both redirect (**303**) to `/admin`. They need a client session; anyone else, including caregivers, is redirected (**303**) to `/` and nothing changes.

- **POST /admin/advance**, **POST /admin/advance/clear**
  - The Advance Note editor on `/admin`. They work like `/admin/note` and `/admin/note/clear`, but for tomorrow's calendar date in Europe/Stockholm, so `date` must be tomorrow. An Advance Note is stored as tomorrow's Daily Note, so caregivers cannot see it today; at midnight (the Rollover) it becomes today's note on `/note` and in the dashboard's today editor, with no publish step or background job.

- **GET /health**
  - Health check. Pings the storage backend and returns an HTML page, or JSON `{"title", "name", "version", "dbStatus"}` when the request's `Accept` header prefers JSON (for example `application/json` or `application/json, text/plain`). Browsers and requests without an `Accept` header get HTML.
  - `dbStatus` is `OK`, or the storage error. The status is **200** when storage answers and **503** when it doesn't, so Docker's `HEALTHCHECK` and load balancers see the service as unhealthy.

- **GET /assets/\***
  - Static files (CSS, images), embedded in the binary or read from disk with `USE_FILE_SYSTEM=true`.

### Protected Endpoints

These endpoints require an `Authorization` header with the configured `AUTH_TOKEN` (e.g., `Authorization: Bearer <token>` or `Authorization: <token>`). A missing or wrong token gets **401**.

- **GET /logs/:from/:to**
  - Retrieve usage logs within a date range, oldest first (by time, then ID), one page at a time:

    ```json
    {
      "entries": [{"id": 1, "status": 200, "method": "GET", "endpoint": "...", "created_at": "2026-09-30T10:15:00Z", "...": "..."}],
      "next": "/logs/2026-09-30/2026-09-30?limit=1000&offset=1000"
    }
    ```

  - `?limit=` sets the page size (default **1000**, at most **10000**) and `?offset=` skips that many entries (default 0). Other values get **400**. Follow `next` until it is `null` to read the whole range; an empty range returns `{"entries": [], "next": null}`.
  - Logs are written in the background, so the newest entries can take up to about a second to appear.
  - `:from` and `:to` are dates in `YYYY-MM-DD` format (anything else gets **400**) and mean whole **UTC** days: from `:from` 00:00Z up to, not including, the day after `:to`. Timestamps are stored in UTC.

### Request logging

No route is logged at the moment. A route with `UseLogger: true` in `router/routes.go` is logged to the storage backend: method, endpoint (including query strings), status, error, and the full request and response bodies. **Warning:** Because the full endpoint with query strings is logged, avoid passing sensitive data (like tokens) in URL parameters to prevent them from being stored in the database. Bodies over **1 MiB** on these routes are rejected with **413** and not logged. In `GET /logs`, a body that is valid JSON appears as JSON; any other body (plain text or truncated data) appears as a JSON string, and an empty body as `{}`. For requests that end in an error, the response body is written after logging, so the entry has `"response": null` with the status and error message in their own fields.

Entries are written in the background (see Features), so logging never slows a request, but entries **can be lost**: when more than 1,000 are waiting (a slow or unreachable database), new ones are dropped; a batch that keeps failing is dropped after about 60 seconds of retries; and at shutdown, entries not written within 5 seconds are dropped. Each of these is logged as a warning or error with the number of entries.

A logged public route lets anyone who can reach the service add rows to the log table, so think twice before setting `UseLogger` on one. Never set it on `POST /pin`, or PINs end up in the log table.

### Authentication token

If `AUTH_TOKEN` is not set, the service generates a random token when it starts and logs it as a warning (`AUTH_TOKEN is not set; using temporary token for this run: ...`). The token changes on every restart and appears in the service log, so set `AUTH_TOKEN` for any real deployment.

## Service Management

The application can be installed as a system service.

```bash
# Install the service
./angel -service install

# Start the service
./angel -service start

# Stop the service
./angel -service stop

# Uninstall the service
./angel -service uninstall
```

## Features

- **Web Server**: Built with [Gin](https://github.com/gin-gonic/gin) for high performance.
- **Service Management**: Can be installed and managed as a system service (Windows Service, Systemd, etc.) using [kardianos/service](https://github.com/kardianos/service).
- **Database Support**: Request logs in SQLite, MySQL or FileMaker (see [FileMaker storage](#filemaker-storage)).
- **Authentication**: Simple token-based authentication for protected routes.
- **Docker Ready**: Includes `Dockerfile` and `docker-compose.yml` for easy containerization.
- **Asset Management**: Supports embedding assets or serving from the file system.
- **Logging**: Request logging to database. Entries are written in the background in batches, so a slow database never slows down requests; `GET /logs` can be up to about a second behind, and pending entries are written when the service stops.

## Getting Started

### Prerequisites

- [Go](https://golang.org/dl/) 1.27.1 or higher
- [Make](https://www.gnu.org/software/make/) (optional, for build scripts)
- [Docker](https://www.docker.com/) (optional, for containerized run)

### Installation

Clone the repository:

```bash
git clone https://github.com/johansundell/angel.git
cd angel
```

### Creating a new service from the template

Rename the Go module and the service. The service name is used for the binary, the system service, the default SQLite file, the Docker image and the compose service. Run this in the fresh clone (GNU `sed`; on macOS use `sed -i ''`):

```bash
NEW_MODULE=github.com/acme/billing-service   # Go module path of the new service
NEW_NAME=billing-service                     # service, binary and Docker name
NEW_ACCOUNT=acme                             # GitHub account for `make release` and the image

OLD_MODULE=github.com/johansundell/angel
git grep -lz "$OLD_MODULE" | xargs -0 sed -i "s#$OLD_MODULE#$NEW_MODULE#g"
git grep -lz angel -- ':!.agents' | xargs -0 sed -i "s#angel#$NEW_NAME#g"
sed -i "s#^GHACCOUNT := .*#GHACCOUNT := $NEW_ACCOUNT#" Makefile

go build ./... && go test ./...
```

The first command rewrites the module path in `go.mod`, the imports and the docs; the second renames the service everywhere else (`main.go`'s `nameOfService`, the `Makefile`, `Dockerfile`, `docker-compose.yml`, `.gitignore` and the READMEs). `.agents/skills` is left alone, because the skill describes the template. Then start a fresh history if you like (`rm -rf .git && git init`), and set the version in the `Makefile`.

### Running Locally

You can run the service directly using Go:

```bash
go run .
```

Or build it using Make:

```bash
make build
./angel
```

The cross-platform targets (`make compile`, `make dist`, `make release`) need `gox` and `github-release`. Install them once with `make deps`; they go into `$(go env GOPATH)/bin`, which must be on your `PATH`.

The service resolves paths relative to **its binary's folder**: the `assets` and `tmpl` folders when `USE_FILE_SYSTEM=true`, a `.env` file (after the current directory), and the default `SQLITE_PATH`. `go run .` builds the binary in a temporary Go folder, which has two effects:

- **`USE_FILE_SYSTEM=true` doesn't work with `go run .`**: the assets and templates aren't found, so pages such as `GET /health` return 500 and `/assets/...` returns 404. Use embedded assets (the default), or build first with `go build` or `make build` and run the binary from the repo, as above.
- **The default SQLite file lands in that temporary folder** and is gone after the next build. With `go run .`, set `SQLITE_PATH`, for example `SQLITE_PATH=./angel.db go run .` (`*.db` is gitignored).

### Running with Docker

To run the service using Docker Compose:

```bash
make docker-run
```

This runs `docker compose up --build` while automatically injecting the correct `VERSION` from the `Makefile`.

This will start the service on port 8080, with the SQLite database (including its WAL and SHM files) in the named Docker volume `data`, mounted at `/app/data`. The volume keeps the data across `docker compose down` and rebuilds; **`docker compose down -v` deletes it**.

The service runs as a non-root user, and a named volume gets the right ownership automatically. A host folder like `./data` usually belongs to your own user and makes SQLite fail with `permission denied`; if you need one, `chown` it to the container user first (`docker compose run --rm --entrypoint id angel` shows the uid and gid).

To copy the database out, for a backup or to inspect it:

```bash
docker compose cp angel:/app/data/angel.db ./angel.db
```

#### With your local .env and database

To try the container with the same settings and SQLite database as a local run:

```bash
make docker-run-local
```

This uses `docker-compose.local.yml` instead of `docker-compose.yml`. It mounts your `./.env` read-only at `/app/.env` and your `./data` folder at `/app/data`, and uses the database `./data/angel.db`. To use that database for local runs too, set `SQLITE_PATH=./data/angel.db` in `.env`; the container replaces that with its own path to the same file. `make docker-run-local` stops when `.env` is missing and creates the data folder as your user; run with plain `docker compose`, Docker would create both as empty root-owned folders. Set `LOCAL_DATA_DIR` to use another folder. Don't run the container and a local binary on the same database at the same time.

The container runs as your user (`make docker-run-local` passes `id -u` and `id -g`), so it can write the database files that you own. Everything else comes from `.env`, except `PORT`: the container always listens on 8080, so pick the host port with `HOST_PORT`. For a MySQL server on your machine, set `MYSQL_HOST=host.docker.internal`, because `127.0.0.1` in the container is the container itself, and let the server accept connections from the Docker network.

Both compose files run the same `angel` service, so starting one replaces a container started from the other. Stop it with `docker compose -f docker-compose.local.yml down`.

#### Published image

Releases are published to the GitHub Container Registry as `ghcr.io/johansundell/angel`. Pushing a `v*` git tag (`make release` creates one) runs the `Docker` workflow ([`.github/workflows/docker.yml`](.github/workflows/docker.yml)): it runs the tests, builds the image with the tag as its version, and pushes `ghcr.io/johansundell/angel:<tag>` and `:latest`. The tag is what the service reports in the `X-Version` header. A server can then run the image without a checkout:

```bash
docker pull ghcr.io/johansundell/angel:latest
```

GHCR makes a new package private. After the first publish, make it public once under the package's **Package settings → Change visibility** on GitHub, so `docker pull` works without logging in. While it is private, log in first with a personal access token (classic) that has the `read:packages` scope:

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
```

`make release` leaves the image to the workflow. `make docker-push` builds the image with the `Makefile` `VERSION` and pushes the same two tags from your machine, which needs a login with a token that has `write:packages`. In a service made from this template, `GHACCOUNT` and the service name in the `Makefile` set the image name, and the workflow publishes to `ghcr.io/<owner>/<repo>`.

#### Port and health check

Inside the container the service always listens on **8080** (the image sets `PORT=:8080`), which the image's `EXPOSE` and health check rely on. Choose the port on the host instead: `HOST_PORT=9090 docker compose up`, or `docker run -p 9090:8080 ...`. Don't set `PORT` for the container. The health check calls `GET /health`, so the container turns unhealthy when the storage backend is unreachable.

### Running behind a reverse proxy

[`examples/reverse-proxy/`](examples/reverse-proxy/) has nginx and Apache configurations with HTTPS from Let's Encrypt, step-by-step setup instructions, a local demo for each proxy, and `check.sh`, which tests that the PIN rate limit works through the proxy. Behind a proxy, set `PORT` to a local address (for example `127.0.0.1:8080`) and `TRUSTED_PROXIES` to the proxy's address.

For a Cloudflare Tunnel with cloudflared on the same host, `TRUSTED_PROXIES=127.0.0.1` is enough; no Cloudflare-specific setting is needed. cloudflared adds the visitor's address as the last `X-Forwarded-For` entry, and Angel reads that header from the right, skipping trusted addresses.

If a request carries `X-Forwarded-For` or `CF-Connecting-IP` from an address that isn't in `TRUSTED_PROXIES`, Angel logs a warning once with that address. It usually means the proxy's address is missing from `TRUSTED_PROXIES`.

### Third-party licences

`THIRD_PARTY_LICENSES.txt` holds the licence texts of the Go standard library and of every module compiled into the binary. `make compile` copies it next to each binary, so it is in the release archives, and the Docker image has it at `/app/THIRD_PARTY_LICENSES.txt`. Run `make licenses` after changing dependencies and commit the result.

## Configuration

The application is configured via environment variables. You can set these in a `.env` file in the root directory. Environment variables explicitly set in the system or terminal take precedence over values in the `.env` file.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DEBUG` | bool | `false` | Log the route table at startup, one access log line per request (method, path, status, duration, client IP), and each written batch of request log entries, through the service log. For troubleshooting; leave it off in normal use. |
| `PORT` | string | `:8080` | The port the server listens on. |
| `USE_FILE_SYSTEM` | bool | `false` | If true, serves assets and templates from the `assets` and `tmpl` folders next to the binary (edit them without rebuilding). If false, uses the embedded copies. Doesn't work with `go run .` (see [Running Locally](#running-locally)). |
| `TIMEOUT` | int | `15` | Request timeout in seconds. |
| `STORAGE` | string | `sqlite` | Storage backend for request logs: `sqlite`, `mysql` or `filemaker`. |
| `SQLITE_PATH` | string | `<binary dir>/<nameOfService>.db` | Path to the SQLite database file. Daily Notes are always kept here; with `STORAGE=sqlite` the request log is too. Set it when using `go run .`, whose binary dir is temporary. |
| `AUTH_TOKEN` | string | random per start | Token required for protected endpoints. When unset, a temporary token is generated and logged (see [Authentication token](#authentication-token)). |
| `CAREGIVER_PIN` | string | - | **Required.** Shared 4-digit PIN that caregivers enter on the entry screen. |
| `MASTER_PIN` | string | - | **Required.** The client's 4–12 digit PIN for the dashboard; must differ from `CAREGIVER_PIN`. |
| `CAREGIVER_SESSION_TIMEOUT` | duration | `20m` | How long a caregiver session lasts before the entry screen is shown again. Must be between `15m` and `30m`. |
| `CLIENT_SESSION_TIMEOUT` | duration | `8h` | How long a client session (Master PIN) lasts before the Master PIN must be entered again. Must be between `15m` and `24h`. |
| `SESSION_TIMEOUT` | duration | - | **Deprecated**: the old name of `CAREGIVER_SESSION_TIMEOUT`. Used only when `CAREGIVER_SESSION_TIMEOUT` is unset, and the service logs a deprecation warning at startup. Rename it. |
| `SESSION_SECRET` | string | random per start | Key that signs session cookies, at least 32 characters. When unset, a random key is generated at start, so everyone enters the PIN again after a restart. It is never logged. |
| `COOKIE_SECURE` | bool | `true` | Mark the session cookie `Secure` (sent over HTTPS only). Browsers also accept it on `http://localhost`; set `false` only to test over plain HTTP from another device. |
| `TRUSTED_PROXIES` | string | - | Comma-separated IPs or CIDRs of reverse proxies (for example `127.0.0.1` for cloudflared on the same host). Only these may set the client address through `X-Forwarded-For`, which the PIN rate limit is keyed on. Leave empty when clients connect directly; behind a proxy, set it, or every caregiver shares one rate limit. See [`examples/reverse-proxy/`](examples/reverse-proxy/). |
| `MYSQL_USERNAME` | string | - | MySQL username (required when `STORAGE=mysql`, as are `MYSQL_HOST` and `MYSQL_DATABASE`). |
| `MYSQL_PASSWORD` | string | - | MySQL password. |
| `MYSQL_HOST` | string | - | MySQL host address. |
| `MYSQL_PORT` | string | `3306` | MySQL port. |
| `MYSQL_DATABASE` | string | - | MySQL database name. |
| `FMS_HOST` | string | - | FileMaker Server URL; must start with `https://` (required when `STORAGE=filemaker`, as are `FMS_DATABASE`, `FMS_USERNAME` and `FMS_PASSWORD`). |
| `FMS_DATABASE` | string | - | FileMaker file (database) name. |
| `FMS_USERNAME` | string | - | FileMaker account with the `fmodata` extended privilege. |
| `FMS_PASSWORD` | string | - | Password for that account. |
| `FMS_TIMEOUT` | duration | `10s` | Timeout for each FileMaker request, and for the startup check. |
| `FMS_LOG_TABLE` | string | `Logs` | FileMaker table for request logs. |
| `FMS_CA_FILE` | string | - | PEM file with extra CA certificates to trust, for a server with a private CA. |
| `FMS_INSECURE_SKIP_VERIFY` | bool | `false` | Skip certificate verification. Logs a warning on every start; prefer `FMS_CA_FILE`. |

## FileMaker storage

With `STORAGE=filemaker`, request logs are written to a FileMaker table through the FileMaker OData API. The service never creates or changes the table: a FileMaker developer sets it up once, and at startup the service checks that it can read the table and every field. A missing table or field, a wrong password or an unreachable server stops startup with FileMaker's error.

Create the table (default name `Logs`, see `FMS_LOG_TABLE`) with these fields:

| Field | Type | Notes |
|-------|------|-------|
| `ID` | Number | Auto-enter serial number, unique. Returned as `id` by `GET /logs`. |
| `Status` | Number | |
| `Method` | Text | |
| `Error` | Text | |
| `Endpoint` | Text | |
| `CreatedAt` | Timestamp | Stored in **UTC** without a time zone. Add a calculation field if people browsing the file want local time. |
| `Request` | Text | Turn indexing off (bodies up to 1 MiB). |
| `Response` | Text | Turn indexing off. |

The account needs the `fmodata` extended privilege and create and view access to the table.

**Connection.** `FMS_HOST` must use `https://`, because the account's password is sent with every request. Certificates are verified; for a server with a private CA, point `FMS_CA_FILE` at the CA's PEM file. `FMS_INSECURE_SKIP_VERIFY=true` turns verification off and logs a warning on every start.

**Writes.** Log entries are sent in batches as one OData `$batch` request each, asking FileMaker not to send the records back (`Prefer: return=minimal`) to save the server's data-transfer allowance. A rejected batch (a 4xx response, for example a changed field) is dropped and logged at once; network errors and 5xx responses are retried.

**Testing against a real server.** `go test -tags filemaker ./store` runs an integration test when `FMS_TEST_HOST`, `FMS_TEST_DATABASE`, `FMS_TEST_USERNAME` and `FMS_TEST_PASSWORD` (and optionally `FMS_TEST_CA_FILE`) are set. It uses its own variables so it never touches a service's database, and creates and deletes a temporary `LogsTest_<timestamp>` table; the account needs schema privileges for that.


