package fofm

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

type Store interface {
	// Connect will connect to the store
	// it is automatcically called by FOFM
	Connect() error

	// Close will close any connections related to the store
	Close() error

	// CreateStore should do the work of creating
	// the storage for the migrations. It
	// should be a "create if doesnt exist" type
	// operation
	CreateStore() error

	// ClearStore should reset the store to an empty state
	ClearStore() error

	// LastRun will return the last run migration
	LastRun() (*Migration, error)

	// LastStatusRun will get the last run with the
	// provided status
	LastStatusRun(status string) (*Migration, error)

	// LastRunByName will get the last run with the
	// provided name
	LastRunByName(name string) (*Migration, error)

	// GetAllByName will return all migrations with the
	// provided name
	GetAllByName(name string) (MigrationSet, error)

	// List will return a list of all migrations that
	// have been saved to the store
	List() (MigrationSet, error)

	// Save should insert a new record
	Save(current Migration, err error) error
}

// Locker is an optional interface for a Store. If the store has a lock, fofm holds it
// while it runs migrations so that only one copy of an app is migrating at a time. The
// store is still usable without one, but then it is up to you to make sure that two
// copies of an app do not migrate at the same time
type Locker interface {
	// Lock waits until it has the lock or the context is done. The returned function
	// releases the lock. A lock that is never released, because its owner crashed, must
	// eventually expire so that it does not block migrations forever
	Lock(ctx context.Context) (unlock func() error, err error)
}

// NoResultsError should be used in place of a
// store's no results error
type NoResultsError struct {
	_             struct{}
	OriginalError error
}

func (re NoResultsError) Error() string {
	return re.OriginalError.Error()
}

const (
	functionalMigrationTableName = "function_migrations"
	selectFields                 = "id, name, direction, status, error, timestamp, created"

	// DefaultLockTTL is how long a lock is held without being renewed before another
	// process may take it over
	DefaultLockTTL = 5 * time.Minute

	// how long a connection waits on a database that another process has locked
	busyTimeout = 5 * time.Second
)

// how often a waiting Lock call checks if the lock is free
var lockPollInterval = 50 * time.Millisecond

func NewSQLiteWithTableName(filepath, tablename string) (*SQLite, error) {
	ins, err := sql.Open("sqlite", filepath)
	if err != nil {
		return nil, err
	}

	// a :memory: database only exists on the connection that made it, so a pool with
	// more than one connection would hand out empty databases. a single connection also
	// stops a file database from fighting itself for its own write lock
	ins.SetMaxOpenConns(1)
	ins.SetMaxIdleConns(1)
	ins.SetConnMaxLifetime(0)

	// wait for another process to finish instead of failing with "database is locked"
	_, err = ins.Exec(fmt.Sprintf(`PRAGMA busy_timeout = %d`, busyTimeout.Milliseconds()))
	if err != nil {
		ins.Close()

		return nil, err
	}

	db := &SQLite{
		filepath:  filepath,
		db:        ins,
		tablename: tablename,
		lockTTL:   DefaultLockTTL,
	}

	return db, nil
}

func NewSQLite(filepath string) (*SQLite, error) {
	return NewSQLiteWithTableName(filepath, functionalMigrationTableName)
}

type SQLite struct {
	_         struct{}
	filepath  string
	tablename string
	db        *sql.DB
	lockTTL   time.Duration
}

var _ Locker = (*SQLite)(nil)

// SetLockTTL changes how long a lock can go without being renewed before another process
// takes it over. A lock that is held is renewed three times per TTL. Call this before the
// store is passed to New
func (s *SQLite) SetLockTTL(ttl time.Duration) {
	s.lockTTL = ttl
}

func (s *SQLite) lockTable() string {
	return s.tablename + "_lock"
}

func (s *SQLite) Connect() error {
	return nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) CreateStore() error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL, 
		direction TEXT NOT NULL, 
		timestamp TEXT NOT NULL, 
		status TEXT NOT NULL,
		error TEXT NULL,
		created TEXT NOT NULL
	)`, s.tablename)
	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}

	// there is only ever one row, which exists while the lock is held
	query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		owner TEXT NOT NULL,
		locked_at INTEGER NOT NULL
	)`, s.lockTable())
	_, err = s.db.Exec(query)

	return err
}

func (s *SQLite) ClearStore() error {
	query := fmt.Sprintf(`
	DELETE FROM %s`, s.tablename)
	_, err := s.db.Exec(query)

	return err
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows
type rowScanner interface {
	Scan(dest ...any) error
}

// scanMigration reads a row that was selected with selectFields
func scanMigration(row rowScanner) (*Migration, error) {
	var (
		mig                Migration
		errText            sql.NullString
		timestamp, created string
	)

	err := row.Scan(
		&mig.ID,
		&mig.Name,
		&mig.Direction,
		&mig.Status,
		&errText,
		&timestamp,
		&created,
	)
	if err != nil {
		return nil, err
	}

	mig.Error = errText.String

	mig.Timestamp, err = parseStoredTime(timestamp)
	if err != nil {
		return nil, fmt.Errorf(`unable to read the timestamp of migration %v -- %w`, mig.Name, err)
	}

	mig.Created, err = parseStoredTime(created)
	if err != nil {
		return nil, fmt.Errorf(`unable to read the created time of migration %v -- %w`, mig.Name, err)
	}

	return &mig, nil
}

// queryOne returns the first migration that the query selects or a NoResultsError
func (s *SQLite) queryOne(query string, args ...any) (*Migration, error) {
	mig, err := scanMigration(s.db.QueryRow(query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NoResultsError{OriginalError: err}
		}

		return nil, err
	}

	return mig, nil
}

// queryMany returns every migration that the query selects
func (s *SQLite) queryMany(query string, args ...any) (MigrationSet, error) {
	migs := MigrationSet{}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return migs, err
	}

	defer rows.Close()

	for rows.Next() {
		mig, err := scanMigration(rows)
		if err != nil {
			return migs, err
		}

		migs = append(migs, *mig)
	}

	return migs, rows.Err()
}

func (s *SQLite) LastRun() (*Migration, error) {
	return s.queryOne(fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename))
}

func (s *SQLite) LastStatusRun(status string) (*Migration, error) {
	return s.queryOne(fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	WHERE
		status = $1
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename), status)
}

func (s *SQLite) LastRunByName(name string) (*Migration, error) {
	return s.queryOne(fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	WHERE
		name = $1
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename), name)
}

func (s *SQLite) List() (MigrationSet, error) {
	return s.queryMany(fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	ORDER BY
		id ASC`, selectFields, s.tablename))
}

func (s *SQLite) GetAllByName(name string) (MigrationSet, error) {
	return s.queryMany(fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	WHERE
		name = $1
	ORDER BY
		id ASC`, selectFields, s.tablename), name)
}

func (s *SQLite) Save(current Migration, err error) error {
	query := fmt.Sprintf(`
	INSERT INTO
		%s (name, direction, status, error, timestamp, created)
	VALUES
		($1, $2, $3, $4, $5, $6)`, s.tablename)

	var errText string
	if err != nil {
		errText = err.Error()
	}

	// the timestamp is written in an explicit format instead of leaving it to the driver
	now := time.Now().UTC().Format(time.RFC1123Z)
	timestamp := current.Timestamp.UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(query, current.Name, current.Direction, current.Status, errText, timestamp, now)

	return err
}

// storedTimeLayouts are every format that has been written to the store. new rows are
// written as RFC3339, but databases created by older versions of fofm hold the others
var storedTimeLayouts = []string{
	time.RFC3339Nano,
	// what the sqlite driver wrote when it was handed a time.Time
	"2006-01-02 15:04:05.999999999-07:00",
	// what was written to the created column
	time.RFC1123Z,
}

// parseStoredTime reads a time from any of the formats that have been written to the
// store. A blank value is the zero time
func parseStoredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}

	for _, layout := range storedTimeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf(`unrecognized time format: %q`, value)
}

// isBusy is whether the database was held by someone else for longer than the busy timeout
func isBusy(err error) bool {
	text := err.Error()

	return strings.Contains(text, "SQLITE_BUSY") || strings.Contains(text, "database is locked")
}

// SetBusyTimeout changes how long a query waits for another process that has the database
// file locked before it gives up. The default is five seconds. A context that ends while a
// query is waiting is not noticed until the wait is over, so this is also the longest that a
// cancelled Lock can take to return when another process is writing to the file
func (s *SQLite) SetBusyTimeout(timeout time.Duration) error {
	_, err := s.db.Exec(fmt.Sprintf(`PRAGMA busy_timeout = %d`, timeout.Milliseconds()))

	return err
}

func newLockOwner() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// Lock takes the migration lock. Each call has its own owner, so it also keeps two
// callers in the same process apart. The lock is renewed in the background while it is
// held and a lock that has not been renewed for the TTL is taken over, so a process that
// crashes while holding it does not block the others forever
func (s *SQLite) Lock(ctx context.Context) (func() error, error) {
	owner, err := newLockOwner()
	if err != nil {
		return nil, err
	}

	ttl := s.lockTTL
	if ttl <= 0 {
		ttl = DefaultLockTTL
	}

	// insert the lock, or take over the existing one only if it has gone stale
	acquire := fmt.Sprintf(`
	INSERT INTO %s (id, owner, locked_at) VALUES (1, $1, $2)
	ON CONFLICT(id) DO UPDATE SET owner = $3, locked_at = $4 WHERE locked_at < $5`, s.lockTable())

	for {
		now := time.Now()
		res, err := s.db.ExecContext(ctx, acquire,
			owner, now.UnixMilli(),
			owner, now.UnixMilli(), now.Add(-ttl).UnixMilli(),
		)

		acquired := false

		switch {
		case err == nil:
			rows, rowsErr := res.RowsAffected()
			acquired = rowsErr == nil && rows == 1
		case ctx.Err() != nil:
			// a context that ends while a query is running interrupts the query, and the
			// driver reports that as its own error instead of the context's
			return nil, ctx.Err()
		case isBusy(err):
			// another process has the whole database right now, so wait for it like any
			// other holder of the lock
		default:
			return nil, err
		}

		if acquired {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(lockPollInterval):
		}
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		renew := fmt.Sprintf(`UPDATE %s SET locked_at = $1 WHERE id = 1 AND owner = $2`, s.lockTable())
		ticker := time.NewTicker(ttl / 3)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				res, err := s.db.Exec(renew, time.Now().UnixMilli(), owner)
				if err != nil {
					return
				}

				// the lock was taken over, so there is nothing left to renew
				if rows, err := res.RowsAffected(); err != nil || rows != 1 {
					return
				}
			}
		}
	}()

	var once sync.Once
	var unlockErr error

	unlock := func() error {
		once.Do(func() {
			close(stop)
			<-stopped

			// the context may be the reason that the work stopped, so it is not used here
			release := fmt.Sprintf(`DELETE FROM %s WHERE id = 1 AND owner = $1`, s.lockTable())
			_, unlockErr = s.db.Exec(release, owner)
		})

		return unlockErr
	}

	return unlock, nil
}
