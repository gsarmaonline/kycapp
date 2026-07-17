package ids

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// New returns a new ULID string.
func New() string {
	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader).String()
}
