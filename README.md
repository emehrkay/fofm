# fofm (eff of em)

Functional migrations in Go. This library will allow you to run functions that exist on a struct as a migration.

## Usage

1. Define a struct with `fofm.Migrations` embedded in it. That is what marks it as a migration manager. It has nothing to set up, and the rest of the struct is whatever your migrations need to do their work.

```go
package migrations

import "github.com/emehrkay/fofm"

type MyMigrationsManager struct {
	fofm.Migrations

	NeededResource *SomeLibrary
}
```

> `fofm.New` only accepts a struct that embeds `fofm.Migrations`, so passing something else by mistake, like a database connection, is a compile error (`does not implement fofm.MigrationManager (missing method isMigrationManager)`) instead of a manager that finds no migrations and quietly does nothing. The embedded struct does not find the migrations itself, Go gives it no way to see the struct that it is in. `fofm.New` does that by looking at the methods of the struct you give it.

> New migrations are written to the package that this struct is in, so give it a package of its own if you use `CreateMigration`. **fofm** works out where that is from the struct itself, there is nothing to implement. See [Creating migrations](#creating-migrations)

> The methods can use value or pointer receivers. If they use pointer receivers, pass a pointer to `fofm.New` (`&MyMigrationsManager{}`). `fofm.New` returns an error if it finds migrations that a value does not have, rather than quietly running nothing.

2. Add some migrations as functions that return an error. This can be done manually as long as the names adhere to the format `Migration_$SOME_INTEGER_up` (for every `up` migration there should be corresponding `down` migration) 

```go
func (m *MyMigrationsManager) Migration_1_up() error {
    // do something once
}

func (m *MyMigrationsManager) Migration_1_down() error {
    // undo the thing from Migration_1_up
}
```

A migration can also take a `context.Context`, which is the one that was passed to `LatestContext`, `UpContext` or `DownContext`

```go
func (m *MyMigrationsManager) Migration_2_up(ctx context.Context) error {
    return m.NeededResource.DoSomethingSlow(ctx)
}
```

Any other signature is an error when you call `fofm.New`. Two migrations can't have the same integer.

> You can also create migrations programatically. 

```go
db, _ := fofm.NewSQLite(":memory:")
myMig := &MyMigrationsManager{
    NeededResource: SomeLibrary.New(),
}
manager, _ := fofm.New(db, myMig)

manager.CreateMigration()
```

> This will add a new file `migration_$unix_time.go` with methods `Migration_$unix_time_up` and `Migration_$unix_time_down` for you to fill in (the number can be a counter instead, see below). It will never overwrite a file that is already there, and it will not reuse the number of an existing migration. It returns the path to the file

Every migration is ordered based on the integer in the method name -- `Migration_1_up, Migration_2_up, ..., Migration_X_up` etc.

#### Numbering new migrations

By default a new migration is numbered with the current unix time (`Migration_1658164360_up`). That is a setting, and you can swap it by passing another one to `fofm.New`

```go
// the default
manager, _ := fofm.New(db, myMig, fofm.UnixTimestamps)

// 1, 2, 3, ... padded to five digits: Migration_00001_up in migration_00001.go
manager, _ := fofm.New(db, myMig, fofm.IncrementingCounter)
```

The counter is one more than the highest migration that exists. Past 99999 it is simply wider. The integers are compared as numbers, so `manager.Up("2")` finds `Migration_00002_up`. Don't switch an existing project from unix times to the counter and expect it to start over at 1 -- it continues from the highest number, because migrations are ordered by their numbers.

To do something else, `fofm.WithNumbering` takes a `fofm.Numbering`, which is two functions: `Next` picks the number, given the numbers that are already used, and `Format` writes it into the name (it has to be digits). It is what `UnixTimestamps` and `IncrementingCounter` are made of.

```go
fofm.WithNumbering(fofm.Numbering{
	Next:   func(taken []int64) int64 { return nextFromMyTicketSystem() },
	Format: func(number int64) string { return fmt.Sprintf("%07d", number) },
})
```

3. Run the migrations

```go
manager.Latest() // to run every migration that has not been run, in order, including the latest one
manager.Up("Migration_10_up") // to run every migration in order up to "Migration_10_up"
manager.Down("1") // to run every migration in reverse order down to "Migration_1_down" 
manager.Pending() // to see what Latest() would run, without running anything
```

> Both the Up and Down methods can accept the full migration name `Migration_1_up`, a partial name `Migration_1`, or just the integer `1`. A name that does not match a migration is an error and nothing is run

### Creating migrations

A new migration has to be in the same package as the struct that has the migrations on it. **fofm** finds that package by looking at where the struct's own methods are defined, since every method of a type is in the same package and so the same directory. The `package` name is read from the Go files that are already there. There is nothing to implement and nothing to configure.

That can't be done in two cases, and `CreateMigration` returns an error that says so when it happens:

- the struct has no methods of its own yet (the embedded `fofm.Migrations` doesn't count, it is not defined in your package). Add any method to it, or tell **fofm** where to write with `fofm.WithMigrationsDir`
- the program was built with `-trimpath`, or it is running somewhere that the source is not. Use `fofm.WithMigrationsDir`

Running migrations never needs any of this, only creating them does.

```go
// the settings that say where new migrations go. you rarely need them
fofm.WithMigrationsDir("./migrations") // a directory, relative to where the program is run
fofm.WithMigrationsFS(myWriteFS)       // any fofm.WriteFS, like one in memory for a test
fofm.WithPackageName("migrations")     // when the package can't be read from the Go files
```

A `fofm.WriteFS` is an `fs.FS` that can also write a file. The standard library's `fs.FS` can only read, so it has the one extra method, `WriteFile`. It must not overwrite a file. `fofm.DirFS(dir)` is the one for a directory on disk.

The files that are already there count: their `migration_$number.go` names are given to the numbering as numbers that are taken. A migration you just created is not part of the program until it is built, so without that, creating two in a row would give them both the same number.

### How it decides what to run

A migration is **applied** when its most recent *successful* run was an `up`. A failed run changes nothing: a failed `up` leaves the migration unapplied and a failed `down` leaves it applied, so either one is tried again the next time.

- `Up(name)` runs the migrations up to and including `name` that are not applied yet
- `Down(name)` runs the downs from the newest applied migration back to and including `name`. Migrations that are not applied are skipped
- `Latest()` runs everything after the last successful `up`. If the last successful run was a `down`, it runs everything from that migration on. If nothing has run, it runs everything

Every run is kept in the store, which is what `Status()` reports.

### Contexts

`Latest`, `Up` and `Down` each have a variant that takes a context -- `LatestContext`, `UpContext` and `DownContext`.

The context is checked before each migration starts. If it is cancelled, the remaining migrations are not run and its error is returned (`errors.Is(err, context.Canceled)`). A migration that is already running is not interrupted by **fofm**, so a long running migration should take a `context.Context` and watch it. The migrations that did finish are recorded, so the next call picks up where this one stopped.

### Locking

**fofm** holds a lock in the store while it runs, so two copies of an app starting at the same time will not both run the same migrations. The second one waits for the first to finish and then finds nothing left to run.

The SQLite store keeps the lock in a table next to the migrations. It is renewed while it is held and it expires if it is not renewed, so an app that crashes in the middle of a migration does not block the others forever (`SQLite.SetLockTTL` changes how long that takes, the default is `fofm.DefaultLockTTL`). Waiting for the lock honors the context that was passed in. If another process is writing to the database file itself, a query waits for it up to five seconds before it is tried again, and a context that ends in the middle of that wait is noticed when the wait is over. `SQLite.SetBusyTimeout` changes how long that is.

A store gets this by implementing the optional `Locker` interface. A store that does not will still work, but then you need to make sure that two copies are never migrating at once.

### Failures

If a migration returns an error, or panics, it is recorded as a failure with the error and **fofm** stops. Nothing is rolled back. A migration is an arbitrary Go function, so there is no transaction to wrap it in. Write migrations so that running one again after a partial failure is safe -- `CREATE TABLE IF NOT EXISTS`, upserts, and checks before changes -- and use a transaction inside the migration if the thing that you are changing supports one.

### Extending

As of now, `sqlite` is the only storage engine pacakged with **fofm**. Luckily, it adheres to the `Store` interface so rolling your own is pretty straight forward. Implement `Locker` as well to get the locking. A migration manager is just a struct that embeds `fofm.Migrations`.

**fofm** really shines when it is used as a command line tool. Simply wrap its public methods behind your defined CLI interfaces and you're good to go. Here is how I used **fofm** with **cobra**

```go

func init() {
	migs := &myMigrationManager{}
	migrationsLocation := env.Get("MIGRATION_LOCATION", ":memory:")
	db, err := fofm.NewSQLite(migrationsLocation)
	if err != nil {
		panic(err)
	}

	manager, err := fofm.New(db, migs)
	if err != nil {
		panic(err)
	}

	var latest = &cobra.Command{
		Use: "migrate_latest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.LatestContext(cmd.Context())
		},
	}

	var create = &cobra.Command{
		Use: "migrate_create",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := manager.CreateMigration()
			if err == nil {
				fmt.Println(path)
			}

			return err
		},
	}

	var pending = &cobra.Command{
		Use: "migrate_pending",
		RunE: func(cmd *cobra.Command, args []string) error {
			pending, err := manager.Pending()
			if err != nil {
				return err
			}

			for _, mig := range pending {
				fmt.Println(mig.Name)
			}

			return nil
		},
	}

	var status = &cobra.Command{
		Use: "migrate_status",
		RunE: func(cmd *cobra.Command, args []string) error {
			migStatus, err := manager.Status()
			if err != nil {
				return err
			}

			headers := []string{"ORDER", "MIGRATION", "STATUS", "RUNS"}
			writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
			fmt.Fprintln(writer, strings.Join(headers, "\t"))

			for i, mig := range migStatus.Migrations {
				runs := []string{}
				status := "not run"

				for _, run := range mig.Runs {
					runs = append(runs, fmt.Sprintf(`(%s %s)`, run.Status, run.Timestamp))
					status = run.Status
				}

				if len(runs) == 0 {
					runs = append(runs, "-")
				}

				row := []string{
					fmt.Sprintf("%v", i),
					mig.Migration.Name,
					status,
					strings.Join(runs, " "),
				}

				fmt.Fprintln(writer, strings.Join(row, "\t"))
			}

			return writer.Flush()
		},
	}

	RootCmd.AddCommand(latest, create, pending, status)
}

```

### Use Cases

Call `manager.Latest()` everytime your app starts up with confidence that it is up to date with any pre-defined one-time calls. It is safe to do from several copies of an app at once.

License MIT
