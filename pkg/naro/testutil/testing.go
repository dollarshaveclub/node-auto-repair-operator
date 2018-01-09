package testutil

import (
	"os"
	"testing"

	"io/ioutil"

	"github.com/coreos/bbolt"
	"github.com/stretchr/testify/assert"
)

// DB returns a *bolt.DB for use in testing along with a function
// that'll cleanup the database.
func DB(t *testing.T) (*bolt.DB, func()) {
	f, err := ioutil.TempFile("/tmp", "node-auto-repair-operator-test-")
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	db, err := bolt.Open(f.Name(), 0600, nil)
	assert.NoError(t, err)

	cleanup := func() {
		assert.NoError(t, db.Close())
		assert.NoError(t, os.Remove(f.Name()))
	}

	return db, cleanup
}
