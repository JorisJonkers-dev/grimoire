package pgtest

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// ErrInjected is the error a Faulty database returns.
var ErrInjected = errors.New("injected")

// Faulty passes calls through to a real database and fails the FailAt-th one.
type Faulty struct {
	DB      queries.DBTX
	Calls   int
	FailAt  int
	counter *Faulty
}

type failedRow struct{}

func (failedRow) Scan(...any) error { return ErrInjected }

func (f *Faulty) fail() bool {
	if f.counter != nil {
		return f.counter.fail()
	}
	f.Calls++
	return f.Calls == f.FailAt
}

// Exec implements queries.DBTX.
func (f *Faulty) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.fail() {
		return pgconn.CommandTag{}, ErrInjected
	}
	return f.DB.Exec(ctx, sql, args...)
}

// Query implements queries.DBTX.
func (f *Faulty) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.fail() {
		return nil, ErrInjected
	}
	return f.DB.Query(ctx, sql, args...)
}

// QueryRow implements queries.DBTX.
func (f *Faulty) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.fail() {
		return failedRow{}
	}
	return f.DB.QueryRow(ctx, sql, args...)
}

// On is a view of f over another database, sharing f's call count; use it when one operation spans
// the pool and a transaction.
func (f *Faulty) On(db queries.DBTX) queries.DBTX {
	return &Faulty{DB: db, FailAt: -1, counter: f}
}

// EveryFault runs fn failing each database call in turn, until a run makes fewer calls than the
// fault index. Every faulted run must surface ErrInjected, and the clean run must succeed.
func EveryFault(t *testing.T, fn func(f *Faulty) error) {
	t.Helper()
	for n := 1; ; n++ {
		f := &Faulty{FailAt: n}
		err := fn(f)
		if f.Calls < n {
			if err != nil {
				t.Fatalf("clean run failed: %v", err)
			}
			return
		}
		if !errors.Is(err, ErrInjected) {
			t.Fatalf("fault at call %d swallowed: %v", n, err)
		}
	}
}
