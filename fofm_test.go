package fofm_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/emehrkay/fofm"
	"github.com/emehrkay/fofm/store"
)

func TestNew(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Errorf(`unable to make db -- %v`, err)
	}

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
	db, err := store.NewSQLite(":memory:")
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
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Errorf(`unable to make db -- %v`, err)
	}

	// overwrite the writer for testing
	var newMigration string
	testWriter := func(ins *fofm.FOFM) error {
		ins.Writer = func(filename string, data []byte, perm fs.FileMode) error {
			newMigration = string(data)
			return nil
		}

		return nil
	}

	tm := TestMigrationManager{}
	mig, err := fofm.New(db, tm, testWriter)
	if err != nil {
		t.Errorf("expected New but got -- %s", err)
	}

	out, err := mig.CreateMigration()
	if err != nil {
		t.Errorf("expected new migration from template but got -- %s", err)
	}

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

func TestRunUpMigration(t *testing.T) {

}

func TestReRunFailedUpMigration(t *testing.T) {

}

func TestRunDownMigration(t *testing.T) {

}

func TestReRunFailedDownMigration(t *testing.T) {

}

func TestListMigrations(t *testing.T) {

}
