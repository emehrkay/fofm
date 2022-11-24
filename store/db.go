package store

import (
	"github.com/emehrkay/fofm/migration"
)

type Store interface {
	// Connect will connect to the store
	// it is automatcically called by FOFM
	Connect() error

	Close() error

	// CreateStore should do the work of creating
	// the storage for the migrations. It
	// should be a "create if doesnt exist" type
	// operation
	CreateStore() error

	Clear() error

	// LastRun will return the last run migration
	LastRun() (migration.Migration, error)

	LastStatusRun(status string) (migration.Migration, error)

	// List will return a list of all migrations that
	// have been saved to the store
	List() ([]migration.Migration, error)

	// Save should insert a new record
	Save(current migration.Migration, err error) error
}
