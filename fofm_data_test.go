package fofm_test

var TestPKGNAME = "test_pkg_name"

type TestMigrationManager struct {
}

func (t TestMigrationManager) GetMigrationsPath() string {
	return ""
}

func (t TestMigrationManager) GetPackageName() string {
	return TestPKGNAME
}

func (i TestMigrationManager) Migration_1_up() error {

	return nil
}

func (i TestMigrationManager) Migration_1_down() error {

	return nil
}

type TestMigrationManagerMultiple struct {
}

func (t TestMigrationManagerMultiple) GetMigrationsPath() string {
	return ""
}

func (t TestMigrationManagerMultiple) GetPackageName() string {
	return TestPKGNAME
}

func (i TestMigrationManagerMultiple) Migration_1_up() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_1_down() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_5_up() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_5_down() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_10_up() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_10_down() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_15_up() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_15_down() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_18_up() error {

	return nil
}

func (i TestMigrationManagerMultiple) Migration_18_down() error {

	return nil
}
