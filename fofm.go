package fofm

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	up               = "up"
	down             = "down"
	migration_prefix = "Migration"
	STATUS_SUCCESS   = "success"
	STATUS_FAILURE   = "failure"
)

type FunctionalMigration interface {
	// GetMigrationsPath this should return the directory where your migration
	// manager lives. This path is used when new migration files are created
	GetMigrationsPath() string

	// GetPackageName should return the name of the package where your migration
	// manager lives. This is sued when migration files are created
	GetPackageName() string
}

// BaseMigration provides an embed struct to easily adhere to the FunctionalMigration interface
type BaseMigration struct{}

func (b BaseMigration) GetMigrationsPath() string {
	// get the path of the file where this is called from
	_, curFile, _, _ := runtime.Caller(1)
	parts := strings.Split(curFile, "/")

	return strings.Join(parts[0:len(parts)-1], "/")
}

// New will creae a new instance of FOFM. It will apply the DefaultSettings which
// can be overwritten by passing in settings
func New(db Store, migrationInstance FunctionalMigration, settings ...Setting) (*FOFM, error) {
	manager := &FOFM{
		DB:                 db,
		Migration:          migrationInstance,
		UpMigrations:       MigrationStack{},
		DownMigrations:     MigrationStack{},
		migrationStuctName: reflect.TypeOf(migrationInstance).Name(),
	}

	settings = append(DefaultSettings, settings...)

	for _, setting := range settings {
		err := setting(manager)
		if err != nil {
			return nil, fmt.Errorf(`error when calling a setting -- %w`, err)
		}
	}

	err := manager.init()
	if err != nil {
		return nil, err
	}

	return manager, nil
}

type FOFM struct {
	_                  struct{}
	DB                 Store
	Migration          FunctionalMigration
	UpMigrations       MigrationStack
	DownMigrations     MigrationStack
	migrationStuctName string
	Seeded             bool
	Writer             WriteFile
}

func (f *FOFM) init() error {
	if f.Seeded {
		return nil
	}

	ins := reflect.TypeOf(f.Migration)
	if ins.Kind() == reflect.Pointer {
		ins = ins.Elem()
	}

	for i := 0; i < ins.NumMethod(); i++ {
		method := ins.Method(i)
		name := method.Name
		mTime, direction, err := MigrationNameParts(name)
		if err != nil {
			continue
		}

		switch direction {
		case up:
			f.UpMigrations.Add(name, direction, mTime)
		case down:
			f.DownMigrations.Add(name, direction, mTime)
		}
	}

	err := f.DB.CreateStore()
	if err != nil {
		return err
	}

	f.UpMigrations.Order()
	f.DownMigrations.Reverse()

	f.Seeded = true

	return nil
}

func (m *FOFM) ClearStore() error {
	return m.DB.ClearStore()
}

// GetNextMigrationTemplate will return a migration template and its unix time
func (m *FOFM) GetNextMigrationTemplate() (string, int64) {
	now := time.Now().Unix()
	sName := m.migrationStuctName
	template := fmt.Sprintf(`package %s
	
func (i %s) Migration_%v_up() error {
	// up migration here
	return nil
}

func (i %s) Migration_%v_down() error {
	// down migration here
	return nil
}
`, m.Migration.GetPackageName(), sName, now, sName, now)

	return template, now
}

// CreateMigration will create a new migration template based on the current unix time
// and it will call the defined Writer (which is a file writer by default)
func (m *FOFM) CreateMigration() (string, error) {
	template, now := m.GetNextMigrationTemplate()
	fileName := fmt.Sprintf(`migration_%v.go`, now)
	fullPath := fmt.Sprintf(`%s/%s`, m.Migration.GetMigrationsPath(), fileName)
	b := []byte(template)
	err := m.Writer(fullPath, b, 0644)

	if err != nil {
		return "", err
	}

	return fullPath, nil
}

// Status returns a list of all migrations and all of the times when they've been run
// since migrations can be run mulitple times
func (m *FOFM) Status() (MigrationSetStatus, error) {
	status := MigrationSetStatus{}

	for _, mig := range m.UpMigrations {
		all, err := m.DB.GetAllByName(mig.Name)
		if err != nil {
			return status, err
		}

		status.Migrations = append(status.Migrations, Status{
			Migration: mig,
			Runs:      all.ToRuns(),
		})
	}

	return status, nil
}

func (m *FOFM) run(names ...string) error {
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil
		}

		_, direction, err := MigrationNameParts(name)
		if err != nil {
			return err
		}

		ret := reflect.ValueOf(m.Migration).MethodByName(name).Call([]reflect.Value{})
		err, _ = ret[0].Interface().(error)
		mig := Migration{
			Name:      name,
			Status:    STATUS_SUCCESS,
			Timestamp: time.Now().UTC(),
			Direction: direction,
		}

		if err != nil {
			err := fmt.Errorf(`error running: %v -- %v`, name, err)
			mig.Status = STATUS_FAILURE
			m.DB.Save(mig, err)

			return err
		}

		err = m.DB.Save(mig, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// Latest will run all up migrations between the last migration that was run and the
// latest defined migration. If the last migration was a failure, it will attempt to rerun it
// if the last run migration is the latest and it was successful, it will return an error. If that
// migration was a failure, it will attempt to rerun it
func (m *FOFM) Latest() error {
	lastRun, err := m.DB.LastRun()
	if err != nil {
		if _, ok := err.(NoResultsError); !ok {
			return err
		}
	}

	if lastRun != nil {
		switch lastRun.Status {
		case STATUS_SUCCESS:
			// do not run anything if the lastRun is actually the latest migration
			if last := m.UpMigrations.Last(); last != nil {
				if lastRun.Name == last.Name {
					return nil
				}
			}

			// the last run should be the next one after the successful run
			after := m.UpMigrations.After(lastRun)
			if len(after) > 1 && lastRun.Is("up") {
				lastRun = &after[1]
			} else {
				lastRun = nil
			}

		case STATUS_FAILURE:
			lastRun, err = m.DB.LastStatusRun(STATUS_SUCCESS)
			if err != nil {
				if _, ok := err.(NoResultsError); !ok {
					return err
				}
			}
		}
	}

	toRun := m.UpMigrations.After(lastRun)

	return m.run(toRun.Names()...)
}

// UP will run all migrations, in order, up to and inclduing the named one passed in
func (m *FOFM) Up(name string) error {
	// ensure that the latest migration with the name arg
	// was not successful
	latest, err := m.DB.LastRunByName(name)
	if err == nil && latest != nil {
		if latest.Status == STATUS_SUCCESS {
			// return fmt.Errorf(`migration "%s" latest run was successful. will not run again`, name)
			return nil
		}
	}

	toRun := m.UpMigrations.BeforeName(name)

	return m.run(toRun.Names()...)
}

// Down will run all migrations, in reverse order, up to and including the named one
// passed in
func (m *FOFM) Down(name string) error {
	toRun := m.DownMigrations.BeforeName(name)

	return m.run(toRun.Names()...)
}

// utility funcs

func MigrationNameParts(name string) (timestamp time.Time, direction string, err error) {
	parts := strings.Split(name, "_")
	if len(parts) != 3 {
		err = fmt.Errorf(`incorrect name: %v must be in the format of Migration_1658164360_up`, name)
		return
	}

	if parts[0] != migration_prefix {
		err = fmt.Errorf(`non-migration method %v`, name)
		return
	}

	direction = parts[2]
	var ts int64
	ts, err = strconv.ParseInt(parts[1], 10, 64)
	timestamp = time.Unix(ts, 0)

	return
}

var fileTime = regexp.MustCompile("[\\d]+")

func MigrationFileNameTime(name string) (timestamp time.Time, err error) {
	fTime := fileTime.FindString(name)
	var ts int64
	ts, err = strconv.ParseInt(fTime, 10, 64)
	timestamp = time.Unix(ts, 0)

	return
}
