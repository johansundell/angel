# Retire Template Stores and Request Logging

## Context

Angel was scaffolded from the Square Moon Go daemon template (`template-service`). That template included generic request logging to a database, pluggable storage backends (SQLite, MySQL, and FileMaker via OData), and token-based authentication for diagnostic log endpoints (`GET /logs/:from/:to`).

As Angel evolved into a specialized care communication daemon:
- Daily Notes and Acknowledgements have always been stored exclusively in SQLite ([ADR-0003](0003-pure-go-sqlite-storage.md)).
- Production care routes are protected by PIN-based session cookies or public, and `POST /pin` must never log request bodies.
- Template endpoints (`/ping`, `/pong`) were removed in #13, leaving no production routes configured for request logging.
- The MySQL driver (`go-sql-driver/mysql`), FileMaker OData client (`fmsodata`), background log queue (`logqueue`), `request_logs` schema, and `AUTH_TOKEN` middleware were unused scaffolding that added maintenance burden and dependency bloat.

## Decision

1. **Retire Unused Stores & OData Client**: Remove MySQL and FileMaker storage implementations, together with the `fmsodata` package and the `go-sql-driver/mysql` dependency.
2. **Retire Request Logging Stack**: Remove the `logqueue` package, `types.UsageLog`, `request_logs` SQLite table, and the `GET /logs/:from/:to` endpoint.
3. **Consolidate Store Interface**: Unify `store.Store` and `store.NoteStore` into a single `store.Store` interface implemented directly by `store.SQLiteStore`.
4. **Simplify Storage Configuration**: Drop the `STORAGE` environment variable. Storage is configured exclusively via `SQLITE_PATH`.
5. **Retire Bearer Auth**: Drop `AUTH_TOKEN`, `ensureAuthToken()`, and `router.AuthMiddleware`. Authentication is handled exclusively by PIN sessions.

## Consequences

- **Reduced Surface Area**: Removed dead code across storage, queueing, routing, and configuration.
- **Lighter Dependencies**: Removed external MySQL driver and its transitive dependencies from `go.mod` and third-party licence manifests.
- **Operational Clarity**: Configuration is simpler; Docker compose files and documentation only reference variables and features actively used by Angel.
