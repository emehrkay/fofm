package fofm

import (
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emehrkay/fofm/migration"
	"github.com/emehrkay/fofm/store"
)

const (
	up               = "up"
	down             = "down"
	migration_prefix = "Migration"
	STATUS_SUCCESS   = "failure"
	STATUS_FAILURE   = "success"
)

type FunctionalMigration interface {
	GetMigrationsPath() string
	GetPackageName() string
}

func New(db store.Store, migrationInstance FunctionalMigration, settings ...Setting) (*FOFM, error) {
	manager := &FOFM{
		DB:                 db,
		Migration:          migrationInstance,
		UpMigrations:       migration.MigrationStack{},
		DownMigrations:     migration.MigrationStack{},
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
	DB                 store.Store
	Migration          FunctionalMigration
	UpMigrations       migration.MigrationStack
	DownMigrations     migration.MigrationStack
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

func (m *FOFM) SaveTemplate(contents []byte, writer io.Writer) error {

	return nil
}

func (m *FOFM) Status() []migration.Migration {
	migrations := []migration.Migration{}

	return migrations
}

func (m *FOFM) Run(name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}

	ret := reflect.ValueOf(m.Migration).MethodByName(name).Call([]reflect.Value{})
	err, _ := ret[0].Interface().(error)
	mig := migration.Migration{
		Name:   name,
		Status: STATUS_SUCCESS,
	}

	if err != nil {
		err := fmt.Errorf(`error running: %v -- %v`, name, err)
		mig.Status = STATUS_FAILURE
		m.DB.Save(mig, err)

		return err
	}

	m.DB.Save(mig, nil)

	return nil
}

func (m *FOFM) Latest() error {
	lastRun, err := m.DB.LastRun()
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// if the last run failed, we will get the previous successful run
	if lastRun.Status == STATUS_FAILURE {
		// TODO: log

		lastRun, err = m.DB.LastStatusRun(STATUS_SUCCESS)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}

	toRun := m.UpMigrations.After(&lastRun)
	for _, mig := range toRun {
		err := m.Run(mig.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *FOFM) Down(name string) error {
	lastRun, err := m.DB.LastRun()
	if err != nil {
		return err
	}

	// if the last run failed, we will get the previous successful run
	if lastRun.Status == STATUS_FAILURE {
		// TODO: log

		lastRun, err = m.DB.LastStatusRun(STATUS_SUCCESS)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}

	toRun := m.DownMigrations.After(&lastRun)
	for _, mig := range toRun {
		err := m.Run(mig.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

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
