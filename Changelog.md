# FOFM

## Unreleased

### Fixed
- `Latest()` re-ran migrations that had already run. After `Up("Migration_2_up")`, it ran `1, 2` again, and
  after `Up("Migration_1_up")` it stopped partway and never reached the newest migrations. It is now
  based on the time in each migration's name and runs exactly what is left
- After a `Down` to a migration in the middle, `Latest()` only runs the migrations that were downed
- `Up()` and `Down()` with a name that matches nothing ran every migration and returned `nil`. They now
  return an error and run nothing
- `Up()` and `Down()` now accept `Migration_1` and `1`, as the README said they did
- `Up()` re-ran migrations that were already applied, and did nothing when asked for a migration that had
  been downed since it last ran. Both are based on what is currently applied now
- `Down()` re-ran migrations that were already downed or were never run
- A migration manager with pointer receivers had no migrations found, so nothing ran and nothing
  complained. Pointer receivers are supported, and a value that is missing migrations that its pointer
  has is an error from `New`
- A new migration file for a pointer receiver manager was generated without a receiver type
- A migration that panics is recorded as a failure instead of crashing the caller
- An error saving a failed run was dropped. It is returned along with the failure
- The `timestamp` of a migration read from the SQLite store came from the `created` column. Both
  columns are now read, in every format that has been written
- `New` modified the shared `DefaultSettings` slice when it was given settings

### Added
- `Migrations`, a struct to embed in the migration manager, and the `MigrationManager` interface that it
  satisfies. They have no methods that can be called. They exist so that `New` can tell, when the program
  is built, that it was given a migration manager
- New migrations are written to the package that the migration manager is defined in, which is
  worked out from where its methods are defined. The package name is read from the Go files that are
  there. `WithMigrationsDir`, `WithMigrationsFS` and `WithPackageName` set them when that can not be
  done (a manager with no methods yet, or a program built with `-trimpath`). `WriteFS` is an `fs.FS`
  that can write a file, `DirFS` is the one for a directory, and `FOFM.FS` and `FOFM.PackageName` are
  the fields the settings set
- The numbers of the `migration_$number.go` files that are already in the directory are given to the
  numbering as taken, so a migration that has been created but not built yet is not given again
- `SQLite.SetBusyTimeout`
- Numbering of new migrations is a setting. `UnixTimestamps` is the default and `IncrementingCounter`
  numbers them 1, 2, 3, ... padded to five digits (`Migration_00001_up`, `migration_00001.go`).
  `WithNumbering` takes your own `Numbering` (a `Next` and a `Format` function), and `UnixNumbering` and
  `CounterNumbering` are the two that are built in. It is the exported `FOFM.Numbering` field
- `LatestContext`, `UpContext` and `DownContext`. Migrations may take a `context.Context`
- `Pending()` returns the migrations that `Latest()` would run
- Locking, so copies of an app starting together do not run the same migrations. It is the optional
  `Locker` interface, which the SQLite store implements with an expiring, renewed lock.
  `SQLite.SetLockTTL` and `DefaultLockTTL` control the expiry
- `MigrationStack.Find`, `MigrationStack.From` and `Migration.MigrationTime`, `Migration.BaseName`
- `New` returns an error for a nil store, a nil migration manager, a migration with a signature that is
  not `func() error` or `func(context.Context) error`, and two migrations with the same time

### Changed
- `New` takes a `MigrationManager`, which is a struct that has `fofm.Migrations` embedded in it, where
  it took a `FunctionalMigration`. A struct that is not one is a compile error
- `GetNextMigrationTemplate` returns an error as well, for when it can't tell where the migration goes
- `MigrationFileNameTime` reads the number from the name of the file, not the whole path
- `Lock` keeps trying while another process has the whole database file, until its context ends, and
  returns the context's error when it ends in the middle of a query
- The default file writer will not overwrite a file that exists
- `CreateMigration` will not reuse the number of an existing migration
- `MigrationStack.Find` compares the integer in a name as a number, so `2` finds `Migration_00002_up`
- `MigrationStack.After` returns the migrations after the one given, based on the time in its name. It
  returned a prefix of the stack before
- `MigrationStack.BeforeName` returns an empty stack for a name it does not know
- The SQLite store uses a single connection and waits up to 5 seconds on a database that another process
  has locked
- `Migration` timestamps are written to the SQLite store as RFC 3339. Rows written by older versions
  are still read, but older versions can not read rows written by this one, so update every copy of an
  app that shares a database together

### Removed
- The `FunctionalMigration` interface, and its `GetMigrationsPath` and `GetPackageName` methods. Embed
  `fofm.Migrations` instead, and new migrations are written to the package the struct is defined in.
  See the first entry under Added
- `FileWriter`, `FOFM.Writer` and the `WriteFile` type, which are now `WriteFS`
- `BaseMigration`. It worked out the migrations directory from the file of whatever called it, which
  is not reliably the file that defines your manager. Where new migrations go is worked out from the
  manager itself now
- The exported `FOFM.Seeded` field
