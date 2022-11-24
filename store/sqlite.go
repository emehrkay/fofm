package store

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/emehrkay/fofm/migration"
)

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

func (s *SQLite) Clear() error {
	return nil
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

func (s *SQLite) LastRun() (migration.Migration, error) {
	query := fmt.Sprintf(`
	SELECT
		%s
	FROM
		%s
	ORDER BY
		id DESC
	LIMIT 1`, selectFields, s.tablename)
	mig := migration.Migration{}
	err := s.db.QueryRow(query).Scan(mig.Scan()...)

	return mig, err
}

func (s *SQLite) LastStatusRun(status string) (migration.Migration, error) {
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
	mig := migration.Migration{}
	err := s.db.QueryRow(query, status).Scan(mig.Scan()...)

	return mig, err
}

func (s *SQLite) List() ([]migration.Migration, error) {
	migs := []migration.Migration{}
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
		mig := migration.Migration{}
		err = rows.Scan(mig.Scan()...)

		if err != nil {
			return migs, err
		}

		migs = append(migs, mig)
	}

	return migs, err
}

func (s *SQLite) Save(current migration.Migration, err error) error {
	query := fmt.Sprintf(`
	INSERT INTO
		%s (name, direction, status, error, timestamp, created)
	VALUES
		($1, $2, $3, $4, $5, NOW()`, s.tablename)

	_, err = s.db.Exec(query, current.Name, current.Direction, current.Status, err.Error(), current.Timestamp)

	return err
}
