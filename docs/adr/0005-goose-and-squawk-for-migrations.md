# goose and squawk for migrations, not Atlas

The architecture first named Atlas for schema-as-code migrations and `atlas migrate lint`. We use
plain, forward-only SQL migrations applied by goose from files embedded in the binary, and lint them
with squawk in CI. Embedding keeps the deploy to one binary with a `migrate` subcommand and no extra
CLI at runtime; squawk is free, Postgres-specific, and catches the same lock, timeout and
destructive-change risks without an account. Migrations are forward-only: no down sections.
