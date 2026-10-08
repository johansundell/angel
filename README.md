[![Go](https://github.com/johansundell/angel/actions/workflows/go.yml/badge.svg)](https://github.com/johansundell/angel/actions/workflows/go.yml)
# angel

A lightweight care communication daemon that provides daily notes and acknowledgement tracking for home care recipients and caregivers. It includes built-in support for system service management, pure-Go SQLite storage, and Docker deployment.

## API Endpoints

Every response carries an `X-Version` header with the build version. Errors are returned as plain text with the HTTP status text as the body (for example `Not Found`). A request that runs longer than `TIMEOUT` seconds gets **503** with the body `Timeout`.

### Public Endpoints

- **GET /**
  - Entry screen: a numeric PIN entry screen (Swedish, "Ange PIN-kod") for the Caregiver PIN or Master PIN. With a valid session it redirects (**303**) to `/note` or `/admin` instead.

- **POST /pin**
  - Form field `pin`. The Caregiver PIN starts a caregiver session and redirects (**303**) to `/note`; the Master PIN starts a client session and redirects to `/admin`. A wrong PIN shows the entry screen again with an error (**401**).
  - Rate limited: after **5** wrong PINs from one client address within **15 minutes**, that address gets **429** (without the PIN being checked) until the 15 minutes have passed. See `TRUSTED_PROXIES` for running behind a proxy.
  - PIN Alert: the rate limit is per address, so guessing from many addresses is not blocked. Instead, after **20** wrong PINs from any addresses within **24 hours**, the Client dashboard shows a PIN Alert with the number of wrong PINs and when the first and last were entered, asking the Client to change the Caregiver PIN, and the service logs a warning. Attempts refused by the rate limit don't count. The alert is kept in memory and stays until the service restarts, which changing the Caregiver PIN requires. Caregivers never see it. See [ADR-0005](docs/adr/0005-pin-alert-instead-of-global-cap.md).

- **POST /logout**
  - The "Logga ut" button at the top of `/note` and `/admin`. Deletes the session cookie and redirects (**303**) to `/`, so the entry screen shows again and either PIN can be entered on the same phone. Needs no session; without one it just redirects. `GET /logout` gets **405**, so a link prefetch cannot log anyone out.
  - Sessions are stateless, so this ends the session on the device that logs out. A copy of the cookie kept elsewhere stays valid until it expires (`CAREGIVER_SESSION_TIMEOUT` or `CLIENT_SESSION_TIMEOUT`).
  - There is no CSRF token, so another site could log a visitor out with a hidden form. That is only a nuisance (enter the PIN again) and is accepted.

- **GET /note**, **GET /admin**
  - The caregiver view and the client dashboard. They need a session for that role; anyone else is redirected (**303**) to `/`.
  - `/note` shows today's Daily Note (the calendar date in Europe/Stockholm), rendered from Markdown to HTML, in a high-contrast red box when it has the Important Flag, or "Inga särskilda instruktioner idag. Allt är som vanligt!" when there is none. Below it, "Kvitteringar idag" lists the times of today's Acknowledgements, newest first ("Kvitterat kl 08:35"), or "Ingen har kvitterat idag än." when there are none, so a Caregiver can see whether anyone has been there today. With `SHARE_CAREGIVER_NAMES=true` it shows first names too ("Maria kl 08:35"; anonymous ones stay "Kvitterat kl 08:35"). Names are off by default: the Caregiver PIN is shared, so they would tell anyone who knows it who visits and when ([ADR-0006](docs/adr/0006-caregiver-names-opt-in.md)). The list updates when the page is reloaded.
  - Notes are written in Markdown (CommonMark, rendered with [goldmark](https://github.com/yuin/goldmark)). A single line break stays a line break, so plain-text notes look as typed. Raw HTML in a note is left out and `javascript:` links are removed, so a note cannot run scripts on caregivers' phones.
  - `/admin` is the client's editor for today's Daily Note: a multiline text box for the Markdown source, the Important Flag ("Viktigt") checkbox, the time it was last saved and a button to clear it. Between them is "Kvitteringar idag", with a hint saying what Caregivers see on `/note`: "Vårdpersonalen ser tiderna." (only the times), or "Vårdpersonalen ser namn och tider." with `SHARE_CAREGIVER_NAMES=true`, then today's Acknowledgements newest first ("Maria kl 08:35", or "Okänd ängel kl 12:05" without a name), or "Inga kvitteringar registrerade idag än." when there are none. Below it is the same editor for tomorrow's Advance Note ("Morgondagens anteckning").
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

- **GET /healthz**
  - Health check. Pings SQLite and returns an HTML page, or JSON `{"title", "name", "version", "dbStatus"}` when the request's `Accept` header prefers JSON (for example `application/json` or `application/json, text/plain`). Browsers and requests without an `Accept` header get HTML.
  - `dbStatus` is `OK`, or the storage error. The status is **200** when storage answers and **503** when it doesn't, so Docker's `HEALTHCHECK` and load balancers see the service as unhealthy.

- **GET /assets/\***
  - Static files (CSS, images), embedded in the binary or read from disk with `USE_FILE_SYSTEM=true`.

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
- **Database Support**: Pure-Go SQLite storage with WAL mode for Daily Notes and Acknowledgements (see [ADR-0003](docs/adr/0003-pure-go-sqlite-storage.md)).
- **Light and dark mode**: The pages follow the device's light or dark setting (`prefers-color-scheme`), including form controls, scrollbars and the phone's address bar. There is no toggle and nothing is stored, so a shared phone never keeps one visitor's choice. Every colour is a token in the `:root` blocks of `assets/css/main.css`, and a test fails if a colour is used anywhere else.
- **Docker Ready**: Includes `Dockerfile` and `docker-compose.yml` for easy containerization.
- **Asset Management**: Supports embedding assets or serving from the file system.

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

To automate bumping the version, running tests, committing and pushing to `main`, and publishing a release, use [`scripts/bump-release.sh`](scripts/bump-release.sh) or `make bump-release`:

```bash
# Default: bump patch version (e.g. v0.0.13 -> v0.0.14)
make bump-release

# Specify bump level or explicit version:
make bump-release BUMP=minor
make bump-release BUMP=v0.1.0

# Dry-run verification without committing or pushing:
make bump-release DRY_RUN=1
```

The script can also be run directly: `./scripts/bump-release.sh [patch|minor|major|vX.Y.Z] [--dry-run]`.

The service resolves paths relative to **its binary's folder**: the `assets` and `tmpl` folders when `USE_FILE_SYSTEM=true`, a `.env` file (after the current directory), and the default `SQLITE_PATH`. `go run .` builds the binary in a temporary Go folder, which has two effects:

- **`USE_FILE_SYSTEM=true` doesn't work with `go run .`**: the assets and templates aren't found, so pages such as `GET /healthz` return 500 and `/assets/...` returns 404. Use embedded assets (the default), or build first with `go build` or `make build` and run the binary from the repo, as above.
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

The container runs as your user (`make docker-run-local` passes `id -u` and `id -g`), so it can write the database files that you own. Everything else comes from `.env`, except `PORT`: the container always listens on 8080, so pick the host port with `HOST_PORT`.

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

Inside the container the service always listens on **8080** (the image sets `PORT=:8080`), which the image's `EXPOSE` and health check rely on. Choose the port on the host instead: `HOST_PORT=9090 docker compose up`, or `docker run -p 9090:8080 ...`. Don't set `PORT` for the container. The health check calls `GET /healthz`, so the container turns unhealthy when the storage backend is unreachable.

#### Running with HTTPS (Let's Encrypt)

`docker-compose.https.yml` runs Angel on a server of its own (for example a fresh VPS) behind [Caddy](https://caddyserver.com/), which gets a certificate from Let's Encrypt and renews it by itself. Angel has no port on the host: the only way in is through Caddy on ports 80 and 443, and Angel trusts `X-Forwarded-For` only from Caddy's fixed address. Caddy sends the same security headers and one-year HSTS as the [nginx and Apache examples](examples/reverse-proxy/), and keeps no access log of its own.

The stack runs the [published image](#published-image), so the server needs no checkout and builds nothing. Put these three files in one folder on the server:

- [`docker-compose.https.yml`](docker-compose.https.yml)
- [`Caddyfile`](Caddyfile)
- `.env`, made from [`ENV_BASE`](ENV_BASE) (step 2)

Commands below are run in that folder. While the image is private, log in to GHCR on the server first (see [Published image](#published-image)).

**1. DNS and ports.** Point an `A` record for your host name, say `angel.example.com`, at the server, and open ports 80 and 443 (TCP, and UDP 443 for HTTP/3). Let's Encrypt must reach port 80 from the internet, and nothing else on the server may use 80 or 443. Leave out the `AAAA` record: the stack's Docker network is IPv4 only, so Docker would pass IPv6 visitors on from its own address, and they would all share one PIN rate limit.

**2. `.env`.** Copy `ENV_BASE` to `.env` next to the compose file and fill in:

```bash
DOMAIN="angel.example.com"    # required
CAREGIVER_PIN="..."           # exactly 4 digits
MASTER_PIN="..."              # 4-12 digits, not the Caregiver PIN
SESSION_SECRET="..."          # at least 32 characters, e.g. from: openssl rand -base64 33
ACME_EMAIL="you@example.com"  # optional: Let's Encrypt warns here before a certificate expires
ACME_CA="https://acme-staging-v02.api.letsencrypt.org/directory"  # first run only, see step 3
VERSION="v0.0.2"              # optional: the image tag to run; latest when unset
```

Angel gets only the variables it needs, by name: the PINs, `SESSION_SECRET`, the session timeouts, `SHARE_CAREGIVER_NAMES`, and `DEBUG`. `PORT`, `TRUSTED_PROXIES`, `SQLITE_PATH` and `COOKIE_SECURE` are fixed in the compose file, so values for them in `.env` are ignored. The network is `172.31.0.0/24` with Caddy on `172.31.0.10`; if that subnet is in use on the host, set `PROXY_SUBNET_PREFIX` (for example `10.99.0`).

**3. First run against staging.** Start the stack:

```bash
docker compose -f docker-compose.https.yml up -d
```

In a checkout, `make docker-run-https` does the same, and first stops with a message when `.env` is missing. Compose stops when `DOMAIN` is missing, and otherwise pulls `ghcr.io/johansundell/angel` with the tag in `VERSION` (`latest` when unset) if it isn't on the server yet, and starts both containers in the background. `make docker-run-https` pulls every time, and takes the tag only from `.env`: it ignores a `VERSION` on the command line or in the shell, because the `Makefile` `VERSION` is the next release, which isn't published yet. Plain `docker compose` lets a `VERSION` exported in the shell win over `.env`. Caddy waits until Angel is healthy. With `ACME_CA` pointing at staging, Let's Encrypt issues an untrusted test certificate, but mistakes in DNS or ports don't count against its [rate limits](https://letsencrypt.org/docs/rate-limits/). Follow it with `docker compose -f docker-compose.https.yml logs -f caddy` until you see `certificate obtained successfully`. Then remove `ACME_CA` from `.env` (the default is Let's Encrypt itself) and run the same command again. Compose recreates Caddy with the new setting, and Caddy, which keeps certificates apart per CA, gets a real one.

**4. Check it.** Open `https://angel.example.com` on a phone using mobile data, then run [`check.sh`](examples/reverse-proxy/#checking-a-setup) from a checkout or a copy of the script: `CAREGIVER_PIN=<your PIN> ./examples/reverse-proxy/check.sh https://angel.example.com`.

**Upgrading.** Set `VERSION` in `.env` to the new tag, then pull and recreate:

```bash
docker compose -f docker-compose.https.yml pull
docker compose -f docker-compose.https.yml up -d
```

With `VERSION` unset, the same two commands move to the newest `latest`; `make docker-run-https` pulls every time. Pinning a tag keeps the server on a known version until you change it, and going back is the same steps with the old tag. The database stays in the `data` volume. Upgrades between releases may change the compose file or the `Caddyfile` too, so compare them with the release you move to.

**Building from a checkout.** To run your own changes behind Caddy, build Angel instead of pulling it:

```bash
make docker-build-https
```

This adds [`docker-compose.https.build.yml`](docker-compose.https.build.yml), which builds the image from the checkout with the `Makefile` `VERSION` and names it `angel-local`, so it never stands in for a published tag. Run `make docker-run-https` to go back to the published image.

**Trying it on your own machine.** With `DOMAIN=localhost`, Caddy makes a certificate with its own local CA instead of asking Let's Encrypt. If ports 80 and 443 are taken, set `HTTP_PORT` and `HTTPS_PORT` (for example `8081` and `8443`); on a server, leave them alone. Give `check.sh` Caddy's root certificate, since curl doesn't know it:

```bash
docker compose -f docker-compose.https.yml cp caddy:/data/caddy/pki/authorities/local/root.crt ./caddy-root.crt
CURL_CA_BUNDLE=./caddy-root.crt ./examples/reverse-proxy/check.sh https://localhost   # or https://localhost:8443
```

The certificates live in the named volumes `caddy_data` and `caddy_config`, and the SQLite database in `data`. They survive `docker compose -f docker-compose.https.yml down`, restarts, upgrades and rebuilds. **`down -v` deletes them**: the Daily Notes are gone, and Caddy asks Let's Encrypt for new certificates, which it [limits per week](https://letsencrypt.org/docs/rate-limits/).

### Running behind a reverse proxy

On a server of its own, [Running with HTTPS (Let's Encrypt)](#running-with-https-lets-encrypt) is the shortest way. To use a proxy you already run, [`examples/reverse-proxy/`](examples/reverse-proxy/) has nginx and Apache configurations with HTTPS from Let's Encrypt, step-by-step setup instructions, a local demo for each proxy, and `check.sh`, which tests that the PIN rate limit works through the proxy. Behind a proxy, set `PORT` to a local address (for example `127.0.0.1:8080`) and `TRUSTED_PROXIES` to the proxy's address.

For a Cloudflare Tunnel with cloudflared on the same host, `TRUSTED_PROXIES=127.0.0.1` is enough; no Cloudflare-specific setting is needed. cloudflared adds the visitor's address as the last `X-Forwarded-For` entry, and Angel reads that header from the right, skipping trusted addresses.

If a request carries `X-Forwarded-For` or `CF-Connecting-IP` from an address that isn't in `TRUSTED_PROXIES`, Angel logs a warning once with that address. It usually means the proxy's address is missing from `TRUSTED_PROXIES`.

### Deploying releases to a VPS

When Angel runs as a system service on a VPS, the `Deploy` workflow ([`.github/workflows/deploy.yml`](.github/workflows/deploy.yml)) can install each new GitHub release automatically. It connects over SSH and runs [`scripts/deploy.sh`](scripts/deploy.sh) on the server. The script downloads the release, checks its checksum, replaces the binary and restarts the service, and puts the previous binary back if `/healthz` doesn't answer. [docs/deploy-vps.md](docs/deploy-vps.md) walks through the setup step by step.

### Third-party licences

`THIRD_PARTY_LICENSES.txt` holds the licence texts of the Go standard library and of every module compiled into the binary. `make compile` copies it next to each binary, so it is in the release archives, and the Docker image has it at `/app/THIRD_PARTY_LICENSES.txt`. Run `make licenses` after changing dependencies and commit the result.

## Configuration

The application is configured via environment variables. You can set these in a `.env` file in the root directory. Environment variables explicitly set in the system or terminal take precedence over values in the `.env` file.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DEBUG` | bool | `false` | Log the route table at startup and one access log line per request (method, path, status, duration, client IP) through the service log. For troubleshooting; leave it off in normal use. |
| `PORT` | string | `:8080` | The port the server listens on. |
| `USE_FILE_SYSTEM` | bool | `false` | If true, serves assets and templates from the `assets` and `tmpl` folders next to the binary (edit them without rebuilding). If false, uses the embedded copies. Doesn't work with `go run .` (see [Running Locally](#running-locally)). |
| `TIMEOUT` | int | `15` | Request timeout in seconds. |
| `SQLITE_PATH` | string | `<binary dir>/<nameOfService>.db` | Path to the SQLite database file where Daily Notes and Acknowledgements are kept. Set it when using `go run .`, whose binary dir is temporary. |
| `CAREGIVER_PIN` | string | - | **Required.** Shared 4-digit PIN that caregivers enter on the entry screen. |
| `MASTER_PIN` | string | - | **Required.** The client's 4–12 digit PIN for the dashboard; must differ from `CAREGIVER_PIN`. |
| `CAREGIVER_SESSION_TIMEOUT` | duration | `20m` | How long a caregiver session lasts before the entry screen is shown again. Must be between `15m` and `30m`. |
| `CLIENT_SESSION_TIMEOUT` | duration | `8h` | How long a client session (Master PIN) lasts before the Master PIN must be entered again. Must be between `15m` and `24h`. |
| `SESSION_TIMEOUT` | duration | - | **Deprecated**: the old name of `CAREGIVER_SESSION_TIMEOUT`. Used only when `CAREGIVER_SESSION_TIMEOUT` is unset, and the service logs a deprecation warning at startup. Rename it. |
| `SESSION_SECRET` | string | random per start | Key that signs session cookies, at least 32 characters. When unset, a random key is generated at start, so everyone enters the PIN again after a restart. It is never logged. |
| `SHARE_CAREGIVER_NAMES` | bool | `false` | Show Caregivers each other's first names with the times of today's Acknowledgements on `/note`; off, they see only the times. Anyone who knows or guesses the shared Caregiver PIN then sees who visits and when, so turn it on only when that is acceptable ([ADR-0006](docs/adr/0006-caregiver-names-opt-in.md)). Unlike the other switches, a value that isn't a boolean (such as `yes`) stops the service at startup. |
| `COOKIE_SECURE` | bool | `true` | Mark the session cookie `Secure` (sent over HTTPS only). Browsers also accept it on `http://localhost`; set `false` only to test over plain HTTP from another device. |
| `TRUSTED_PROXIES` | string | - | Comma-separated IPs or CIDRs of reverse proxies (for example `127.0.0.1` for cloudflared on the same host). Only these may set the client address through `X-Forwarded-For`, which the PIN rate limit is keyed on. Leave empty when clients connect directly; behind a proxy, set it, or every caregiver shares one rate limit. See [`examples/reverse-proxy/`](examples/reverse-proxy/). |



