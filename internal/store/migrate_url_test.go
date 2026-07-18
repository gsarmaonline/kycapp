package store

import "testing"

func TestToMigrateURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			"postgresql://postgres:secret@postgres.railway.internal:5432/railway",
			"pgx5://postgres:secret@postgres.railway.internal:5432/railway",
		},
		{
			"postgres://postgres:secret@localhost:5432/kyc?sslmode=disable",
			"pgx5://postgres:secret@localhost:5432/kyc?sslmode=disable",
		},
		{
			`  "postgresql://u:p@h/db"  `,
			"pgx5://u:p@h/db",
		},
		{
			"pgx5://u:p@h/db",
			"pgx5://u:p@h/db",
		},
	}
	for _, tc := range cases {
		if got := toMigrateURL(tc.in); got != tc.want {
			t.Fatalf("toMigrateURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
