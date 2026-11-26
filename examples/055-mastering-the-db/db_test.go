package main

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
)

// setup connects, creates the p055_ tables and drops them when the test
// ends. Tests that use it must not run in parallel: they share the tables.
func setup(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; see compose.yaml")
	}
	ctx := t.Context()
	pool, err := newPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := createSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), dropSQL); err != nil {
			t.Errorf("drop tables: %v", err)
		}
	})
	return pool, dsn
}

func TestStructMapping(t *testing.T) {
	pool, dsn := setup(t)
	want := []User{{ID: 1, Email: "ada@example.com"}, {ID: 2, Email: "linus@example.com"}}

	got, err := activeUsersPgx(t.Context(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("pgx: got %+v, want %+v", got, want)
	}

	db, err := sqlx.ConnectContext(t.Context(), "pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err = activeUsersSqlx(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("sqlx: got %+v, want %+v", got, want)
	}
}

func TestNullHandling(t *testing.T) {
	pool, _ := setup(t)

	tests := []struct {
		email     string
		wantValid bool
		wantBio   string
	}{
		{"ada@example.com", true, "First programmer"},
		{"linus@example.com", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			r, err := scanBio(t.Context(), pool, tt.email)
			if err != nil {
				t.Fatal(err)
			}
			if gotNil := r.pointer == nil; gotNil == tt.wantValid {
				t.Errorf("*string nil = %t, want %t", gotNil, !tt.wantValid)
			}
			if r.null.Valid != tt.wantValid || r.null.V != tt.wantBio {
				t.Errorf("sql.Null = %+v, want {V:%q Valid:%t}", r.null, tt.wantBio, tt.wantValid)
			}
			if r.text.Valid != tt.wantValid || r.text.String != tt.wantBio {
				t.Errorf("pgtype.Text = %+v, want {String:%q Valid:%t}", r.text, tt.wantBio, tt.wantValid)
			}
			if r.coalesce != tt.wantBio {
				t.Errorf("COALESCE = %q, want %q", r.coalesce, tt.wantBio)
			}
			// A plain string works for a value but fails on NULL.
			if gotErr := r.plainErr != nil; gotErr == tt.wantValid {
				t.Errorf("plain string err = %v, want error only for NULL", r.plainErr)
			}
			if r.plainErr != nil && !strings.Contains(r.plainErr.Error(), "cannot scan NULL") {
				t.Errorf("plain string err = %v, want a \"cannot scan NULL\" error", r.plainErr)
			}
		})
	}
}

// TestRun runs the whole demo once, which also proves it cleans up after
// itself (setup is not used, so run must create and drop its own tables).
func TestRun(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; see compose.yaml")
	}
	var out strings.Builder
	if err := run(t.Context(), dsn, &out); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	for _, want := range []string{
		"after Sprintf:  p055_injection_victim exists = false",
		"after $1:       p055_injection_victim exists = true",
		"cleanup: dropped the p055_ tables",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q", want)
		}
	}
}
