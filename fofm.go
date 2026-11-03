package fofm

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
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

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

// Migrations is embedded in the struct that has the migrations on it. It is what marks the
// struct as a migration manager, so that New will only take a struct that was meant to be
// one. It has no fields and nothing to set up.
//
//	type MyMigrations struct {
//		fofm.Migrations
//		DB *sql.DB
//	}
//
//	func (m *MyMigrations) Migration_1_up() error { ... }
//	func (m *MyMigrations) Migration_1_down() error { ... }
//
// An embedded struct has no way to see the struct that it is in, so it does not find the
// migrations. New does that by looking at the methods of the struct that it is given.
type Migrations struct{}

// isMigrationManager is what makes MigrationManager possible. It is not exported, so the
// only way to have it is to embed Migrations.
func (Migrations) isMigrationManager() {}

// MigrationManager is a struct that has fofm.Migrations embedded in it. It is what New takes.
// Anything else, like a database connection that was passed in by mistake, is a compile
// error instead of a manager that quietly has no migrations.
type MigrationManager interface {
	isMigrationManager()
}

// New will creae a new instance of FOFM. It will apply the DefaultSettings which
// can be overwritten by passing in settings.
//
// The migration instance is a struct that has Migrations embedded in it and the migration
// methods on it. It can be a value or a pointer. Pass a pointer if the migration methods are
// defined with pointer receivers. It does not need any other methods, new migrations are
// written to the package that it is defined in.
func New(db Store, migrationInstance MigrationManager, settings ...Setting) (*FOFM, error) {
	if db == nil {
		return nil, fmt.Errorf(`a store is required`)
	}

	if migrationInstance == nil {
		return nil, fmt.Errorf(`a migration instance is required`)
	}

	if value := reflect.ValueOf(migrationInstance); value.Kind() == reflect.Pointer && value.IsNil() {
		return nil, fmt.Errorf(`the migration instance is a nil pointer`)
	}

	manager := &FOFM{
		DB:             db,
		Migration:      migrationInstance,
		UpMigrations:   MigrationStack{},
		DownMigrations: MigrationStack{},
	}

	// a new slice is made so the shared DefaultSettings backing array is never written to
	all := make([]Setting, 0, len(DefaultSettings)+len(settings))
	all = append(all, DefaultSettings...)
	all = append(all, settings...)

	for _, setting := range all {
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
	_              struct{}
	DB             Store
	Migration      MigrationManager
	UpMigrations   MigrationStack
	DownMigrations MigrationStack

	// FS is where new migrations are written. Unless a Setting sets it, it is the directory
	// that the migration manager is defined in
	FS WriteFS

	// PackageName is the package that new migrations are in. Unless a Setting sets it, it
	// is read from the Go files that are beside the new migration
	PackageName string

	// Numbering is how new migrations are numbered. It is UnixTimestamps unless a Setting
	// changes it
	Numbering Numbering

	// receiver is how the migration manager is written as a method receiver in a
	// new migration file, either Name or *Name
	receiver string
	seeded   bool
}

func (f *FOFM) init() error {
	if f.seeded {
		return nil
	}

	// the type is used as it was given. the method set of a pointer has the methods of
	// both pointer and value receivers, but a value only has the value receiver ones
	ins := reflect.TypeOf(f.Migration)
	f.receiver = receiverName(ins)

	for i := 0; i < ins.NumMethod(); i++ {
		method := ins.Method(i)
		name := method.Name
		mTime, direction, err := MigrationNameParts(name)
		if err != nil {
			continue
		}

		switch direction {
		case up, down:
			if err := checkMigrationSignature(method); err != nil {
				return err
			}
		}

		switch direction {
		case up:
			f.UpMigrations.Add(name, direction, mTime)
		case down:
			f.DownMigrations.Add(name, direction, mTime)
		}
	}

	if err := f.checkForPointerOnlyMigrations(ins); err != nil {
		return err
	}

	if err := f.UpMigrations.checkForDuplicateTimes(); err != nil {
		return err
	}

	if err := f.DownMigrations.checkForDuplicateTimes(); err != nil {
		return err
	}

	err := f.DB.CreateStore()
	if err != nil {
		return err
	}

	f.UpMigrations.Order()
	f.DownMigrations.Reverse()

	f.seeded = true

	return nil
}

// receiverName is how the type is written in a method receiver
func receiverName(typ reflect.Type) string {
	if typ.Kind() == reflect.Pointer {
		return "*" + typ.Elem().Name()
	}

	return typ.Name()
}

// checkMigrationSignature makes sure that a migration can be called. It must be either
// func() error or func(context.Context) error
func checkMigrationSignature(method reflect.Method) error {
	// the receiver is the first argument of a method that comes from a type
	t := method.Type
	validArgs := t.NumIn() == 1 || (t.NumIn() == 2 && t.In(1) == contextType)
	validReturn := t.NumOut() == 1 && t.Out(0) == errorType

	if !validArgs || !validReturn {
		return fmt.Errorf(
			`%v has an invalid signature. a migration must be either func() error or func(context.Context) error`,
			method.Name,
		)
	}

	return nil
}

// checkForPointerOnlyMigrations returns an error when there are migrations that are
// defined with pointer receivers, but the migration instance is not a pointer. Those
// migrations would never be found, so nothing would run and nothing would complain
func (f *FOFM) checkForPointerOnlyMigrations(ins reflect.Type) error {
	if ins.Kind() == reflect.Pointer {
		return nil
	}

	ptr := reflect.PointerTo(ins)
	var missing []string

	for i := 0; i < ptr.NumMethod(); i++ {
		name := ptr.Method(i).Name
		if _, _, err := MigrationNameParts(name); err != nil {
			continue
		}

		if _, ok := ins.MethodByName(name); !ok {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			`%v has migrations with pointer receivers (%v) so it must be passed in as a pointer, like &%v{}`,
			ins.Name(), strings.Join(missing, ", "), ins.Name(),
		)
	}

	return nil
}

func (m *FOFM) ClearStore() error {
	return m.DB.ClearStore()
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

// call runs one migration. A panic is turned into an error so that it is recorded like
// any other failure
func (m *FOFM) call(ctx context.Context, name string) (err error) {
	method := reflect.ValueOf(m.Migration).MethodByName(name)
	if !method.IsValid() {
		return fmt.Errorf(`%v is not a method of the migration manager`, name)
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf(`panic: %v`, r)
		}
	}()

	var args []reflect.Value
	if method.Type().NumIn() == 1 {
		args = append(args, reflect.ValueOf(ctx))
	}

	ret := method.Call(args)
	if migErr, ok := ret[0].Interface().(error); ok {
		return migErr
	}

	return nil
}

func (m *FOFM) run(ctx context.Context, names ...string) error {
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil
		}

		// stop before starting a migration, never in the middle of one
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(`stopped before running %v -- %w`, name, err)
		}

		_, direction, err := MigrationNameParts(name)
		if err != nil {
			return err
		}

		runErr := m.call(ctx, name)
		mig := Migration{
			Name:      name,
			Status:    STATUS_SUCCESS,
			Timestamp: time.Now().UTC(),
			Direction: direction,
		}

		if runErr != nil {
			err := fmt.Errorf(`error running: %v -- %w`, name, runErr)
			mig.Status = STATUS_FAILURE

			if saveErr := m.DB.Save(mig, err); saveErr != nil {
				return fmt.Errorf(`%w (the failure could not be recorded -- %v)`, err, saveErr)
			}

			return err
		}

		err = m.DB.Save(mig, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// withLock holds the store's lock while fn runs, if the store has one. Two copies of an
// app starting at the same time would otherwise both see the same pending migrations and
// both run them
func (m *FOFM) withLock(ctx context.Context, fn func() error) (err error) {
	locker, ok := m.DB.(Locker)
	if !ok {
		return fn()
	}

	unlock, err := locker.Lock(ctx)
	if err != nil {
		return fmt.Errorf(`unable to lock the migration store -- %w`, err)
	}

	defer func() {
		if unlockErr := unlock(); unlockErr != nil && err == nil {
			err = fmt.Errorf(`unable to unlock the migration store -- %w`, unlockErr)
		}
	}()

	return fn()
}

// lastSuccessfulRun returns the most recent run that was a success, or nil if there
// has not been one. A failed run is skipped over because it did not change anything
func (m *FOFM) lastSuccessfulRun() (*Migration, error) {
	lastRun, err := m.DB.LastRun()
	if err != nil {
		if _, ok := err.(NoResultsError); !ok {
			return nil, err
		}

		return nil, nil
	}

	if lastRun != nil && lastRun.Status != STATUS_SUCCESS {
		lastRun, err = m.DB.LastStatusRun(STATUS_SUCCESS)
		if err != nil {
			if _, ok := err.(NoResultsError); !ok {
				return nil, err
			}

			return nil, nil
		}
	}

	return lastRun, nil
}

// Pending returns the up migrations that Latest would run right now, in the order that
// it would run them. Nothing is run
func (m *FOFM) Pending() (MigrationStack, error) {
	lastRun, err := m.lastSuccessfulRun()
	if err != nil {
		return nil, err
	}

	if lastRun == nil {
		return m.UpMigrations, nil
	}

	switch {
	case lastRun.Is(up):
		return m.UpMigrations.After(lastRun), nil
	case lastRun.Is(down):
		return m.UpMigrations.From(lastRun), nil
	}

	return nil, fmt.Errorf(`unable to find the pending migrations. %v has an unknown direction: %q`, lastRun.Name, lastRun.Direction)
}

// Latest is LatestContext with a context that is never cancelled
func (m *FOFM) Latest() error {
	return m.LatestContext(context.Background())
}

// LatestContext will run every up migration that has not been run yet. Where it picks up from
// depends on the last successful run:
//   - nothing has been run: every up migration is run
//   - an up migration was the last: everything after it is run, so nothing happens if
//     it was the latest migration
//   - a down migration was the last: it and everything after it is run again
//
// A migration that failed is not a successful run, so it is attempted again. Pending
// returns the migrations that this will run.
//
// The context is checked before each migration is started and it is passed to the
// migrations that accept one. If it is cancelled, the remaining migrations are not run and
// the context's error is returned. A migration that is already running is never
// interrupted by fofm, it is up to the migration to watch its context.
func (m *FOFM) LatestContext(ctx context.Context) error {
	return m.withLock(ctx, func() error {
		toRun, err := m.Pending()
		if err != nil {
			return err
		}

		return m.run(ctx, toRun.Names()...)
	})
}

// applied returns the base name (Migration_1) of every migration that is currently
// applied. A migration is applied when its most recent successful run was an up. A failed
// run is ignored because it did not change anything: a failed up leaves the migration
// unapplied and a failed down leaves it applied, so either can be attempted again
func (m *FOFM) applied() (map[string]bool, error) {
	runs, err := m.DB.List()
	if err != nil {
		return nil, err
	}

	applied := map[string]bool{}

	for _, run := range runs {
		if run.Status != STATUS_SUCCESS {
			continue
		}

		// the name is the source of truth for the direction
		_, direction, err := MigrationNameParts(run.Name)
		if err != nil {
			continue
		}

		run.Direction = direction
		applied[run.BaseName()] = direction == up
	}

	return applied, nil
}

// Up is UpContext with a context that is never cancelled
func (m *FOFM) Up(name string) error {
	return m.UpContext(context.Background(), name)
}

// UpContext will run all migrations, in order, up to and inclduing the named one passed in. The name
// can be the full migration name (Migration_1_up), a partial name (Migration_1), or just
// the integer (1). Migrations that are already applied are skipped. An error is returned
// if there is no up migration with that name. See LatestContext for how the context is used
func (m *FOFM) UpContext(ctx context.Context, name string) error {
	mig := m.UpMigrations.Find(name)
	if mig == nil {
		return fmt.Errorf(`unable to run up: there is no up migration named %q`, name)
	}

	return m.withLock(ctx, func() error {
		applied, err := m.applied()
		if err != nil {
			return err
		}

		toRun := MigrationStack{}

		for _, candidate := range m.UpMigrations.BeforeName(mig.Name) {
			if !applied[candidate.BaseName()] {
				toRun = append(toRun, candidate)
			}
		}

		return m.run(ctx, toRun.Names()...)
	})
}

// Down is DownContext with a context that is never cancelled
func (m *FOFM) Down(name string) error {
	return m.DownContext(context.Background(), name)
}

// DownContext will run all migrations, in reverse order, up to and including the named one
// passed in. The name can be the full migration name (Migration_1_down), a partial name
// (Migration_1), or just the integer (1). Only migrations that are currently applied are
// run, so one that was never run, or has already been downed, is skipped. An error is
// returned if there is no down migration with that name. See LatestContext for how the
// context is used
func (m *FOFM) DownContext(ctx context.Context, name string) error {
	mig := m.DownMigrations.Find(name)
	if mig == nil {
		return fmt.Errorf(`unable to run down: there is no down migration named %q`, name)
	}

	return m.withLock(ctx, func() error {
		applied, err := m.applied()
		if err != nil {
			return err
		}

		toRun := MigrationStack{}

		for _, candidate := range m.DownMigrations.BeforeName(mig.Name) {
			if applied[candidate.BaseName()] {
				toRun = append(toRun, candidate)
			}
		}

		return m.run(ctx, toRun.Names()...)
	})
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

var fileTime = regexp.MustCompile(`[\d]+`)

func MigrationFileNameTime(name string) (timestamp time.Time, err error) {
	// only the file name, because the directory may have numbers in it too
	fTime := fileTime.FindString(filepath.Base(name))
	var ts int64
	ts, err = strconv.ParseInt(fTime, 10, 64)
	timestamp = time.Unix(ts, 0)

	return
}
