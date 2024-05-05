package fofm

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
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
)

func NewSQLiteWithTableName(filepath, tablename string) (*SQLite, error) {
	ins, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	db := &SQLite{
		filepath:  filepath,
		db:        ins,
		tablename: tablename,
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

	return err
}

func (s *SQLite) ClearStore() error {
	query := fmt.Sprintf(`
	DELETE FROM %s`, s.tablename)
	_, err := s.db.Exec(query)

	return err
}

func (s *SQLite) LastRun() (*Migration, error) {
	query := fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename)
	mig := Migration{}
	var timestamp, created string
	fields := []any{
		&mig.ID,
		&mig.Name,
		&mig.Direction,
		&mig.Status,
		&mig.Error,
		&timestamp,
		&created,
	}
	err := s.db.QueryRow(query).Scan(fields...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NoResultsError{OriginalError: err}
		}

		return nil, err
	}

	err = mig.CreatedFromString(created)
	if err != nil {
		return nil, err
	}

	err = mig.TimestampFromString(created)
	if err != nil {
		return nil, err
	}

	return &mig, nil
}

func (s *SQLite) LastStatusRun(status string) (*Migration, error) {
	query := fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	WHERE
		status = $1
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename)
	mig := Migration{}
	var timestamp, created string
	fields := []any{
		&mig.ID,
		&mig.Name,
		&mig.Direction,
		&mig.Status,
		&mig.Error,
		&timestamp,
		&created,
	}

	err := s.db.QueryRow(query, status).Scan(fields...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NoResultsError{OriginalError: err}
		}

		return nil, err
	}

	err = mig.CreatedFromString(created)
	if err != nil {
		return nil, err
	}

	err = mig.TimestampFromString(created)
	if err != nil {
		return nil, err
	}

	return &mig, err
}

func (s *SQLite) LastRunByName(name string) (*Migration, error) {
	query := fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	WHERE
		name = $1
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename)
	mig := Migration{}
	var timestamp, created string
	fields := []any{
		&mig.ID,
		&mig.Name,
		&mig.Direction,
		&mig.Status,
		&mig.Error,
		&timestamp,
		&created,
	}
	err := s.db.QueryRow(query, name).Scan(fields...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NoResultsError{OriginalError: err}
		}

		return nil, err
	}

	err = mig.CreatedFromString(created)
	if err != nil {
		return nil, err
	}

	err = mig.TimestampFromString(created)
	if err != nil {
		return nil, err
	}

	return &mig, err
}

func (s *SQLite) List() (MigrationSet, error) {
	migs := MigrationSet{}
	query := fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	ORDER BY
		id ASC
	`, selectFields, s.tablename)
	rows, err := s.db.Query(query)

	if err != nil {
		return migs, err
	}

	defer rows.Close()

	for rows.Next() {
		mig := Migration{}
		var timestamp, created string
		fields := []any{
			&mig.ID,
			&mig.Name,
			&mig.Direction,
			&mig.Status,
			&mig.Error,
			&timestamp,
			&created,
		}
		err = rows.Scan(fields...)

		if err != nil {
			return migs, err
		}

		err = mig.CreatedFromString(created)
		if err != nil {
			return migs, err
		}

		err = mig.TimestampFromString(created)
		if err != nil {
			return migs, err
		}

		migs = append(migs, mig)
	}

	return migs, err
}

func (s *SQLite) GetAllByName(name string) (MigrationSet, error) {
	migs := MigrationSet{}
	query := fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	WHERE
		name = $1
	ORDER BY
		id ASC
	`, selectFields, s.tablename)
	rows, err := s.db.Query(query, name)

	if err != nil {
		return migs, err
	}

	defer rows.Close()

	for rows.Next() {
		mig := Migration{}
		var timestamp, created string
		fields := []any{
			&mig.ID,
			&mig.Name,
			&mig.Direction,
			&mig.Status,
			&mig.Error,
			&timestamp,
			&created,
		}
		err = rows.Scan(fields...)

		if err != nil {
			return migs, err
		}

		err = mig.CreatedFromString(created)
		if err != nil {
			return migs, err
		}

		err = mig.TimestampFromString(created)
		if err != nil {
			return migs, err
		}

		migs = append(migs, mig)
	}

	return migs, err
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

	now := time.Now().UTC().Format(time.RFC1123Z)
	_, err = s.db.Exec(query, current.Name, current.Direction, current.Status, errText, current.Timestamp, now)

	return err
}
