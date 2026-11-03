package fofm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MigrationSet []Migration

func (m MigrationSet) ToRuns() []Run {
	runs := []Run{}

	for _, mig := range m {
		runs = append(runs, Run{
			Timestamp: mig.Timestamp,
			Status:    mig.Status,
		})
	}

	return runs
}

type Run struct {
	_         struct{}  `json:"-"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}
type Status struct {
	_         struct{}  `json:"-"`
	Migration Migration `json:"migration"`
	Runs      []Run     `json:"runs"`
}
type MigrationSetStatus struct {
	_          struct{} `json:"-"`
	Migrations []Status `json:"migration_statuses"`
}

type Migration struct {
	_         struct{}  `json:"-"`
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Direction string    `json:"direction"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	Created   time.Time `json:"created"`
}

func (m *Migration) Scan() []any {
	return []any{
		&m.ID,
		&m.Name,
		&m.Direction,
		&m.Status,
		&m.Error,
		&m.Timestamp,
		&m.Created,
	}
}

// BaseName is the migration's name without its direction, so Migration_1_up and
// Migration_1_down are both Migration_1. It is what ties an up to its down
func (m Migration) BaseName() string {
	return strings.TrimSuffix(m.Name, "_"+m.Direction)
}

func (m Migration) Is(direction string) bool {
	return strings.EqualFold(direction, m.Direction)
}

func (m *Migration) TimestampFromString(timeStr string) error {
	if strings.TrimSpace(timeStr) == "" {
		return nil
	}

	var err error
	m.Timestamp, err = parseStoredTime(timeStr)
	return err
}

func (m *Migration) CreatedFromString(timeStr string) error {
	if strings.TrimSpace(timeStr) == "" {
		return nil
	}

	var err error
	m.Created, err = parseStoredTime(timeStr)
	return err
}

// MigrationTime is the time that is encoded in the migration's name. It is what
// migrations are ordered by. A migration that was read from the store has a Timestamp
// of when it was run, which is a different thing, so the name is preferred
func (m Migration) MigrationTime() time.Time {
	if mTime, _, err := MigrationNameParts(m.Name); err == nil {
		return mTime
	}

	return m.Timestamp
}

type MigrationStack []Migration

func (m MigrationStack) Names() []string {
	names := []string{}

	for _, mig := range m {
		names = append(names, mig.Name)
	}

	return names
}

func (m MigrationStack) First() *Migration {
	if len(m) > 0 {
		return &m[0]
	}

	return nil
}

func (m MigrationStack) Last() *Migration {
	if len(m) > 0 {
		return &m[len(m)-1]
	}

	return nil
}

func (m *MigrationStack) Add(name, direction string, timestamp time.Time) error {
	*m = append(*m, Migration{
		Timestamp: timestamp,
		Name:      name,
		Direction: direction,
	})

	return nil
}

func (m MigrationStack) Order() {
	sort.Slice(m, func(i, j int) bool {
		return m[i].Timestamp.Before(m[j].Timestamp)
	})
}

func (m MigrationStack) Reverse() {
	sort.Slice(m, func(i, j int) bool {
		return m[i].Timestamp.After(m[j].Timestamp)
	})
}

// checkForDuplicateTimes returns an error if two migrations have the same time in their
// names, like Migration_1_up and Migration_01_up. Their order would be a guess
func (m MigrationStack) checkForDuplicateTimes() error {
	seen := map[int64]string{}

	for _, mig := range m {
		key := mig.Timestamp.Unix()
		if other, ok := seen[key]; ok {
			return fmt.Errorf(`%v and %v have the same time in their names. every migration must have its own`, other, mig.Name)
		}

		seen[key] = mig.Name
	}

	return nil
}

// Find returns the migration that the name refers to or nil if there isn't one. The name
// can be the full migration name, like Migration_1_up, the name without a direction, like
// Migration_1, or just the integer, like 1. The integer is compared as a number, so 2 finds
// Migration_00002_up. Direction is decided by the stack, so Migration_1_down will never
// find anything in a stack of up migrations
func (m MigrationStack) Find(name string) *Migration {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	for i, mig := range m {
		if mig.Name == name {
			return &m[i]
		}
	}

	number, err := strconv.ParseInt(strings.TrimPrefix(name, migration_prefix+"_"), 10, 64)
	if err != nil {
		return nil
	}

	for i, mig := range m {
		if mig.Timestamp.Unix() == number {
			return &m[i]
		}
	}

	return nil
}

// BeforeName returns every migration from the start of the stack up to and including
// the named one. It returns an empty stack when there isn't a migration with that name
func (m MigrationStack) BeforeName(name string) MigrationStack {
	for i, mig := range m {
		if mig.Name == name {
			return m[:i+1]
		}
	}

	return MigrationStack{}
}

// After returns every migration whose time is later than the one passed in. The time
// is read from the name of the migration that is passed in, because that is what the
// stack is ordered by. A nil migration returns the whole stack
func (m MigrationStack) After(after *Migration) MigrationStack {
	if after == nil {
		return m
	}

	cutoff := after.MigrationTime()

	return m.filter(func(t time.Time) bool {
		return t.After(cutoff)
	})
}

// From is like After, but it also includes the migration that is passed in
func (m MigrationStack) From(from *Migration) MigrationStack {
	if from == nil {
		return m
	}

	cutoff := from.MigrationTime()

	return m.filter(func(t time.Time) bool {
		return !t.Before(cutoff)
	})
}

func (m MigrationStack) filter(keep func(migrationTime time.Time) bool) MigrationStack {
	stack := MigrationStack{}

	for _, mig := range m {
		if keep(mig.MigrationTime()) {
			stack = append(stack, mig)
		}
	}

	return stack
}
