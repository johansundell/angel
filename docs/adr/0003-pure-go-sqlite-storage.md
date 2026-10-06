# Pure-Go SQLite for Local Storage

## Context
Angel needs to persist daily notes, acknowledgements, and metadata across service restarts. We considered flat JSON files versus an embedded SQLite database.

## Decision
We will use SQLite with a pure-Go driver (`github.com/ncruces/go-sqlite3`) as the persistence engine, storing data in a single local database file.

## Reasons
- **Zero CGO requirements**: Compiles cross-platform without external C compiler toolchains.
- **Relational integrity & query safety**: Provides atomic transactions, safe concurrent writes, and clean date-based queries for rollover and history.
- **Operational simplicity**: Stored as a single self-contained database file that is trivial to back up.

## Driver
This ADR first named `modernc.org/sqlite`, but the code has always used `github.com/ncruces/go-sqlite3`, which Angel inherited from the Square Moon service template (template-service). Both drivers are CGO-free, so the reasons above hold for either. We kept ncruces instead of switching, so Angel stays on the template's driver and can take template fixes without changes (#12).

ncruces ships SQLite compiled to WASM and machine-translated to Go (`github.com/ncruces/go-sqlite3-wasm`), so the binary contains plain Go code and needs no C toolchain or WASM runtime.
