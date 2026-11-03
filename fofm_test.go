package fofm_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/emehrkay/fofm"
)

func getDB(t *testing.T) fofm.Store {
	db, err := fofm.NewSQLite(":memory:")
	if err != nil {
		t.Errorf(`unable to make db -- %v`, err)
	}

	return db
}

func TestNew(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	if len(mig.UpMigrations) != 1 {
		t.Errorf("expected one up migration got -- %v", len(mig.UpMigrations))
	}

	if len(mig.DownMigrations) != 1 {
		t.Errorf("expected one down migration got -- %v", len(mig.DownMigrations))
	}
}

func TestNewShouldOrderMigrations(t *testing.T) {
	db, err := fofm.NewSQLite(":memory:")
	if err != nil {
		t.Errorf(`unable to make db -- %v`, err)
	}

	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	if len(mig.UpMigrations) != 5 {
		t.Errorf("expected five up migrations got -- %v", len(mig.UpMigrations))
	}

	if len(mig.DownMigrations) != 5 {
		t.Errorf("expected five down migrations got -- %v", len(mig.DownMigrations))
	}

	past := time.Unix(-100000, 0)
	fut := time.Now().AddDate(1, 1, 1)

	for i, m := range mig.UpMigrations {
		if !m.Timestamp.After(past) {
			t.Errorf(`the up orderign is wrong. migration %d @ %s is before the previous time: %s`, i, m.Timestamp, past)
		}

		past = m.Timestamp
	}

	for i, m := range mig.DownMigrations {
		if !m.Timestamp.Before(fut) {
			t.Errorf(`the up orderign is wrong. migration %d @ %s is before the previous time: %s`, i, m.Timestamp, fut)
		}

		fut = m.Timestamp
	}
}

func TestCreateMigrationTemplate(t *testing.T) {
	db := getDB(t)

	// send new migrations to memory
	fsys, settings := newMemSettings()

	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm, settings...)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	out, err := mig.CreateMigration()
	if err != nil {
		t.Errorf("expected new migration from template but got -- %s", err)
	}

	newMigration := fsys.read(out)

	mTime, err := fofm.MigrationFileNameTime(out)
	if err != nil {
		t.Errorf("unable to call MigrationFileNameParts on %s -- %s", out, err)
	}

	// check the template for expected content
	expectedPKG := fmt.Sprintf(`package %v`, TestPKGNAME)
	if !strings.Contains(newMigration, expectedPKG) {
		t.Errorf(`expcted the template to contain -- %v`, expectedPKG)
	}

	expectedUp := fmt.Sprintf(`Migration_%v_up`, mTime.Unix())
	if !strings.Contains(newMigration, expectedUp) {
		t.Errorf(`expcted the template to contain -- %v`, expectedUp)
	}

	expectedDown := fmt.Sprintf(`Migration_%v_down`, mTime.Unix())
	if !strings.Contains(newMigration, expectedDown) {
		t.Errorf(`expcted the template to contain -- %v`, expectedDown)
	}
}

func TestRunLatestUpMigration(t *testing.T) {
	db := getDB(t)

	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	ranMig := false
	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		ranMig = true
		return nil
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf("unable to run latest-- %s", err)
	}

	if !ranMig {
		t.Errorf(`unable to run up migration`)
	}

	MigrationUpFunc = MigrationUpFuncOrig

	list, err := mig.DB.List()
	if err != nil || len(list) != 1 {
		t.Errorf(`the number of migrations in the list is incorrect. expected 1 got %v`, len(list))
	}
}

func TestRunLatestUpMigrationAfterRunningEarlierMigration(t *testing.T) {
	db := getDB(t)

	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	ranMig := false
	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		ranMig = true
		return nil
	}

	err = mig.Up("Migration_1_up")
	if err != nil {
		t.Errorf("unable to run Migration_1_up -- %v", err)
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf("unable to run latest -- %v", err)
	}

	if !ranMig {
		t.Errorf(`unable to run up migration`)
	}

	MigrationUpFunc = MigrationUpFuncOrig

	// Up(1) runs 1, then Latest() runs the remaining 5, 10, 15 and 18. this used to
	// expect 4 because Latest() stopped after 10 and never ran 15 or 18
	list, err := mig.DB.List()
	if err != nil {
		t.Fatalf(`unable to list migrations -- %v`, err)
	}

	got := []string{}
	for _, m := range list {
		got = append(got, m.Name)
	}

	expected := []string{"Migration_1_up", "Migration_5_up", "Migration_10_up", "Migration_15_up", "Migration_18_up"}
	if strings.Join(got, ",") != strings.Join(expected, ",") {
		t.Errorf("incorrect migrations ran\n got: %v\nwant: %v", got, expected)
	}
}

func TestRunLatestUpMigrationOnceWhenCalledMultipleTimes(t *testing.T) {
	db := getDB(t)

	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	first := "first"
	ranMig := ""
	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		ranMig = first
		return nil
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf("unable to run latest -- %v", err)
	}

	if ranMig != first {
		t.Errorf(`unable to run up migration`)
	}

	second := "second"
	MigrationUpFunc = func() error {
		ranMig = second
		return nil
	}

	err = mig.Latest()
	if err != nil || ranMig != first {
		t.Errorf(`test ran the second Latest() call`)
	}

	MigrationUpFunc = MigrationUpFuncOrig
}

func TestRunLatestUpMigrationAndAllBeforeIt(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	ranMig := false
	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		ranMig = true
		return nil
	}

	ranMig10 := false
	MigrationUpFuncOrig10 := MigrationUpFunc10
	MigrationUpFunc10 = func() error {
		ranMig10 = true
		return nil
	}

	ranMig18 := false
	MigrationUpFuncOrig18 := MigrationUpFunc18
	MigrationUpFunc18 = func() error {
		ranMig18 = true
		return nil
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf("unable to run latest -- %v", err)
	}

	if !ranMig || !ranMig10 || !ranMig18 {
		t.Errorf(`unable to run up migration`)
	}

	MigrationUpFunc = MigrationUpFuncOrig
	MigrationUpFunc10 = MigrationUpFuncOrig10
	MigrationUpFunc18 = MigrationUpFuncOrig18
}

func TestShouldNotRerunUpMigrationIfLastStatusWasSuccess(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	ranMig := false
	migValue := ""
	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		ranMig = true
		return nil
	}

	err = mig.Latest()
	if !ranMig || err != nil || migValue != "" {
		t.Errorf(`unable to run up migration`)
	}

	MigrationUpFunc = func() error {
		ranMig = true
		migValue = "some value"
		return nil
	}

	err = mig.Up("Migration_1_up")
	if err != nil || migValue != "" {
		t.Errorf("expected not to rerun migration -- %v", err)
	}

	MigrationUpFunc = MigrationUpFuncOrig
}

func TestRunFailedUpMigration(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		return fmt.Errorf("some failure")
	}

	err = mig.Latest()
	if err == nil {
		t.Errorf(`the migration should've returned an error`)
	}

	list, err := mig.DB.List()
	if err != nil {
		t.Errorf(`unable to list migrations -- %v`, err)
	}

	if len(list) != 1 {
		t.Errorf(`unexpected number of migrations (%v) should be 1`, len(list))
	}

	if list[0].Status != fofm.STATUS_FAILURE {
		t.Errorf(`expected %v but got %v`, fofm.STATUS_FAILURE, list[0].Status)
	}

	MigrationUpFunc = MigrationUpFuncOrig
}

func TestReRunFailedUpMigration(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	var migRan bool
	MigrationUpFuncOrig := MigrationUpFunc
	MigrationUpFunc = func() error {
		return fmt.Errorf("some failure")
	}

	err = mig.Latest()
	if err == nil {
		t.Errorf(`the migration should've returned an error -- %v`, err)
	}

	list, err := mig.DB.List()
	if err != nil {
		t.Errorf(`unable to list migrations -- %v`, err)
	}

	if len(list) != 1 {
		t.Errorf(`unexpected number of migrations (%v) should be 1`, len(list))
	}

	if list[0].Status != fofm.STATUS_FAILURE {
		t.Errorf(`expected %v but got %v`, fofm.STATUS_FAILURE, list[0].Status)
	}

	// fix the migration
	MigrationUpFunc = func() error {
		migRan = true
		return nil
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf(`unable to rerun migration -- %v`, err)
	}

	if migRan != true {
		t.Errorf(`the migration did not rerun`)
	}

	list, err = mig.DB.List()
	if err != nil {
		t.Errorf(`unable to list migrations -- %v`, err)
	}

	if len(list) != 2 {
		t.Errorf(`unexpected number of migrations (%v) should be 2`, len(list))
	}

	if list[1].Status != fofm.STATUS_SUCCESS {
		t.Errorf(`expected %v but got %v`, fofm.STATUS_SUCCESS, list[1].Status)
	}

	MigrationUpFunc = MigrationUpFuncOrig
}

func TestRunDownMigration(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	// run a migration
	err = mig.Up("Migration_18_up")
	if err != nil {
		t.Errorf("unable to run migration -- %s", err)
	}

	migDown := false
	MigrationDownFuncOrig := MigrationDownFunc
	MigrationDownFunc = func() error {
		migDown = true
		return nil
	}

	err = mig.Down("Migration_1_down")
	if err != nil {
		t.Errorf("unable to run migration -- %s", err)
	}

	if !migDown {
		t.Error("unable to migrate down")
	}

	// check the store
	entries, err := mig.DB.GetAllByName("Migration_1_down")
	if err != nil {
		t.Errorf("unable to get Migration_1_down from store -- %s", err)
	}

	if len(entries) != 1 {
		t.Errorf("incorrect number of Migration_1_down entires in the store, should be 1 got %v", len(entries))
	}

	MigrationDownFunc = MigrationDownFuncOrig
}

func TestReRunFailedDownMigration(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	// run a migration
	err = mig.Up("Migration_18_up")
	if err != nil {
		t.Errorf("unable to run migration -- %s", err)
	}

	MigrationDownFuncOrig := MigrationDownFunc
	MigrationDownFunc = func() error {
		return errors.New("some migration error")
	}

	err = mig.Down("Migration_1_down")
	if err == nil {
		t.Errorf("the migration shouldve failed")
	}

	// fix the migration
	migDown := false
	MigrationDownFunc = func() error {
		migDown = true
		return nil
	}

	err = mig.Down("Migration_1")
	if err != nil {
		t.Errorf("the migration shouldve passed -- %v", err)
	}

	if !migDown {
		t.Error("unable to migrate down")
	}

	MigrationDownFunc = MigrationDownFuncOrig
}

func TestCanRunLatestMigraionsMigrateDownAndBackToLatest(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf(`unable to run latest migration -- %v`, err)
	}

	err = mig.Down("1")
	if err != nil {
		t.Errorf(`unable to migrate down -- %v`, err)
	}

	err = mig.Latest()
	if err != nil {
		t.Errorf(`unable to rerun latest migration -- %v`, err)
	}

	list, err := mig.DB.List()
	if err != nil {
		t.Errorf(`unable to get list -- %v`, err)
	}

	if len(list) != 15 {
		t.Errorf(`expected 15 runs, but got %v`, len(list))
	}
}

func TestMigrationStatus(t *testing.T) {
	db := getDB(t)
	tm := TestMigrationManagerMultiple{}
	mig, err := fofm.New(db, tm)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	// run a migration
	run := "15"
	err = mig.Up(run)
	if err != nil {
		t.Errorf("unable to run migration -- %s", err)
	}

	status, err := mig.Status()
	if err != nil {
		t.Errorf("unable to build status -- %s", err)
	}

	if len(status.Migrations) != len(mig.UpMigrations) {
		t.Errorf("incorrect status entries. got %v but wanted %v", len(status.Migrations), len(mig.UpMigrations))
	}

	// ensure that the migrations that have run, have run times
	unRun := "Migration_18_up"
	for _, mig := range status.Migrations {
		if mig.Migration.Name != unRun {
			if len(mig.Runs) == 0 {
				t.Errorf("expectd %v to have previously run", mig.Migration.Name)
			}
		} else if len(mig.Runs) != 0 {
			// Up("15") used to run every migration because "15" was not matched
			t.Errorf("expected %v not to have run", mig.Migration.Name)
		}
	}
}
