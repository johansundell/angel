# Pure-Go SQLite for Local Storage

## Context
Angel needs to persist daily notes, acknowledgements, and metadata across service restarts. We considered flat JSON files versus an embedded SQLite database.

## Decision
We will use SQLite with a pure-Go driver (`modernc.org/sqlite`) as the persistence engine, storing data in a single local database file.

## Reasons
- **Zero CGO requirements**: Compiles cross-platform without external C compiler toolchains.
- **Relational integrity & query safety**: Provides atomic transactions, safe concurrent writes, and clean date-based queries for rollover and history.
- **Operational simplicity**: Stored as a single self-contained database file that is trivial to back up.
