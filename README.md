# fofm (eff of em)

Functional migrations in Go. This library will allow you to run functions that exist on a struct as a migration.

## Usage

1. Define a struct that adheres to the `FunctionalMigration` interface. 

```go
package some_package

import "github.com/emehrkay/fofm"

type MyMigrationsManager struct {
    NeededResource *SomeLibrary
}

func (m MyMigrationsManager) GetPackageName() string {
	return "some_package"
}

func (m MyMigrationsManager) GetMigrationsPath() string {
	_, curFile, _, _ := runtime.Caller(0)
	parts := strings.Split(curFile, "/")

	return strings.Join(parts[0:len(parts)-1], "/")
}
```

> Put this in its own package if you ever use the `CreateMigration` function as it will add files to that directory

2. Add some migrations as functions that return an error. This can be done manually as long as the names adhere to the format `Migration_$SOME_INTEGER_up` (for every `up` migration there should be corresponding `down` migration) 

```go
func (m MyMigrationsManager) Migration_1_up() error {
    // do something once
}

func (m MyMigrationsManager) Migration_1_down() error {
    // undo the thing from Migration_1_up
}
```

> You can also create migrations programatically. 

```go
db, _ := fofm.NewSQLite(":memory:")
myMig := MyMigrationsManager{
    NeededResource: SomeLibrary.New(),
}
manager, _ := fofm.New(myMig, db)

manager.CreateMigration()
```

> This will add a new file `migration_$unix_time.go` with methods `Migration_$unix_time_up` and `Migration_$unix_time_down` for you to fill in

Every migration is ordered based on the integer in the method name -- `Migration_1_up, Migration_2_up, ..., Migration_X_up` etc.

3. Run the migrations

```go
manager.Latest() // to run all migrations in order including the latest one
manager.Up("Migration_10_up") // to run every migration in order up to "Migration_10_up"
manager.Down("1") // to run every migration in reverse order down to "Migration_1_down" 
```

> Both the Up and Down methods can accept the full migration name `Migration_1_up`, a partial name `Migration_1`, or just the integer `1`

### Extending

As of now, `sqlite` is the only storage engine pacakged with **fofm**. Luckily, it adheres to the `Store` interface so rolling your own is pretty straight forward.

**fofm** really shines when it is used as a command line tool. Simply wrap its public methods behind your defined CLI interfaces and you're good to go. Here is how I used **fofm** with **cobra**

```go

func init() {
	migs := myMigrationManager{}
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
		Run: func(cmd *cobra.Command, args []string) {
			manager.Latest()
		},
	}

	var create = &cobra.Command{
		Use: "migrate_create",
		Run: func(cmd *cobra.Command, args []string) {
			manager.CreateMigration()
		},
	}

	var status = &cobra.Command{
		Use: "migrate_status",
		Run: func(cmd *cobra.Command, args []string) {
			migStatus, err := manager.Status()
			if err != nil {
				panic(err)
			}

			headers := []string{"ORDER", "MIGRATION", "STATUS", "RUNS"}
			writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
			fmt.Fprintln(writer, strings.Join(headers, "\t"))

			for i, mig := range migStatus.Migrations {
				order := fmt.Sprintf("%v", i)
				runs := []string{}

				for _, run := range mig.Runs {
					runs = append(runs, fmt.Sprintf(`(%s %s)`, run.Status, run.Timestamp))
				}

				var status string
				if len(mig.Runs) == 0 {
					status = "not run"
					runs = append(runs, "-")
				}

				row := []string{
					order,
					mig.Migration.Name,
					status,
					strings.Join(runs, " "),
				}

				fmt.Fprintln(writer, strings.Join(row, "\t"))
			}

			writer.Flush()
		},
	}

	RootCmd.AddCommand(latest, create, status)
}

```

### Use Cases

Call `manager.Latest()` everytime your app starts up with confidence that it is up to date with any pre-defined one-time calls.

License MIT