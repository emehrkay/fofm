package fofm

import (
	"errors"
	"fmt"
	"go/token"
	"strconv"
	"time"
)

type Setting func(ins *FOFM) error

var DefaultSettings = []Setting{
	// name new migrations with the current unix time
	UnixTimestamps,
}

// Numbering decides the number in the name of a new migration, like the 1658164360 in
// Migration_1658164360_up. Migrations run in the order of their numbers.
type Numbering struct {
	// Next returns the number for the next migration. taken is the number of every
	// migration that already exists, from lowest to highest. The result must not be one of
	// them, because two migrations can not share a number
	Next func(taken []int64) int64

	// Format is how the number is written in the migration's name and in the name of its
	// file. It has to be digits, since that is what a migration's name is read as. If it is
	// not set, the number is written as it is
	Format func(number int64) string
}

// UnixNumbering numbers migrations with the current unix time
var UnixNumbering = Numbering{
	Next: func(taken []int64) int64 {
		used := make(map[int64]bool, len(taken))
		for _, number := range taken {
			used[number] = true
		}

		// two migrations created in the same second would otherwise be the same
		now := time.Now().Unix()
		for used[now] {
			now++
		}

		return now
	},
	Format: formatDecimal,
}

// counterWidth is how many digits CounterNumbering pads its numbers out to
const counterWidth = 5

// CounterNumbering numbers migrations 1, 2, 3, and so on, written with leading zeros so that
// they are five digits wide, like Migration_00001_up. The next number is one more than the
// highest one that exists. Past 99999 the number is simply wider
var CounterNumbering = Numbering{
	Next: func(taken []int64) int64 {
		var highest int64
		for _, number := range taken {
			if number > highest {
				highest = number
			}
		}

		return highest + 1
	},
	Format: func(number int64) string {
		return fmt.Sprintf("%0*d", counterWidth, number)
	},
}

func formatDecimal(number int64) string {
	return strconv.FormatInt(number, 10)
}

// UnixTimestamps is the default. New migrations are named with the current unix time
func UnixTimestamps(ins *FOFM) error {
	ins.Numbering = UnixNumbering

	return nil
}

// IncrementingCounter names new migrations with a counter that is padded to five digits
// instead of a time
func IncrementingCounter(ins *FOFM) error {
	ins.Numbering = CounterNumbering

	return nil
}

// WithNumbering names new migrations with your own Numbering
func WithNumbering(numbering Numbering) Setting {
	return func(ins *FOFM) error {
		if numbering.Next == nil {
			return errors.New(`a Numbering needs a Next function`)
		}

		ins.Numbering = numbering

		return nil
	}
}

// WithMigrationsDir writes new migrations to a directory. This is not needed in most
// cases, because the directory is worked out from where the migration manager is defined.
// It is for when that can not be done: a manager that has no methods yet, or a program
// that was built with -trimpath. The directory has to be the package of the manager
func WithMigrationsDir(dir string) Setting {
	return func(ins *FOFM) error {
		if dir == "" {
			return errors.New(`WithMigrationsDir needs a directory`)
		}

		ins.FS = DirFS(dir)

		return nil
	}
}

// WithMigrationsFS writes new migrations to any WriteFS, like one in memory. The files in it
// are read to find the package name and the numbers that are already used
func WithMigrationsFS(fsys WriteFS) Setting {
	return func(ins *FOFM) error {
		if fsys == nil {
			return errors.New(`WithMigrationsFS needs a filesystem`)
		}

		ins.FS = fsys

		return nil
	}
}

// WithPackageName sets the package for new migrations. Without it, the package is read
// from the Go files that the migrations are written beside
func WithPackageName(name string) Setting {
	return func(ins *FOFM) error {
		if !token.IsIdentifier(name) {
			return fmt.Errorf(`%q is not a valid package name`, name)
		}

		ins.PackageName = name

		return nil
	}
}
