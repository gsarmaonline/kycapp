package obsstore_test

import (
	"testing"

	"github.com/gsarmaonline/kyc/internal/obsstore"
)

func TestMigrationFilesEmbedded(t *testing.T) {
	files, err := obsstore.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected embedded obs migrations")
	}
	found := false
	for _, f := range files {
		if f == "000001_init.up.sql" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing 000001_init.up.sql in %#v", files)
	}
}
