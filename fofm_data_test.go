package fofm_test

import (
	"runtime"
	"strings"
)

var TestPKGNAME = "test_pkg_name"

type TestMigrationManager struct {
}

func (t TestMigrationManager) GetPackageName() string {
	return TestPKGNAME
}

func (t TestMigrationManager) GetMigrationsPath() string {
	_, curFile, _, _ := runtime.Caller(0)
	parts := strings.Split(curFile, "/")

	return strings.Join(parts[0:len(parts)-1], "/")
}

var MigrationUpFunc = func() error {
	return nil
}

var MigrationUpFunc2 = func() error {
	return nil
}

var MigrationUpFunc3 = func() error {
	return nil
}

var MigrationDownFunc = func() error {
	return nil
}

var MigrationDownFunc2 = func() error {
	return nil
}

var MigrationDownFunc3 = func() error {
	return nil
}

func (i TestMigrationManager) Migration_1_up() error {
	return MigrationUpFunc()
}

func (i TestMigrationManager) Migration_1_down() error {
	return MigrationDownFunc()
}

type TestMigrationManagerMultiple struct {
}

func (t TestMigrationManagerMultiple) GetPackageName() string {
	return TestPKGNAME
}

func (t TestMigrationManagerMultiple) GetMigrationsPath() string {
	_, curFile, _, _ := runtime.Caller(0)
	parts := strings.Split(curFile, "/")

	return strings.Join(parts[0:len(parts)-1], "/")
}

func (i TestMigrationManagerMultiple) Migration_1_up() error {
	return MigrationUpFunc()
}

func (i TestMigrationManagerMultiple) Migration_1_down() error {
	return MigrationDownFunc()
}

var MigrationUpFunc5 = func() error {
	return nil
}

var MigrationDownFunc5 = func() error {
	return nil
}

func (i TestMigrationManagerMultiple) Migration_5_up() error {
	return MigrationUpFunc5()
}

func (i TestMigrationManagerMultiple) Migration_5_down() error {
	return MigrationDownFunc5()
}

var MigrationUpFunc10 = func() error {
	return nil
}

var MigrationDownFunc10 = func() error {
	return nil
}

func (i TestMigrationManagerMultiple) Migration_10_up() error {
	return MigrationUpFunc10()
}

func (i TestMigrationManagerMultiple) Migration_10_down() error {
	return MigrationDownFunc10()
}

var MigrationUpFunc15 = func() error {
	return nil
}

var MigrationDownFunc15 = func() error {
	return nil
}

func (i TestMigrationManagerMultiple) Migration_15_up() error {
	return MigrationUpFunc15()
}

func (i TestMigrationManagerMultiple) Migration_15_down() error {
	return MigrationDownFunc15()
}

var MigrationUpFunc18 = func() error {
	return nil
}

var MigrationDownFunc18 = func() error {
	return nil
}

func (i TestMigrationManagerMultiple) Migration_18_up() error {
	return MigrationUpFunc18()
}

func (i TestMigrationManagerMultiple) Migration_18_down() error {
	return MigrationDownFunc18()
}
