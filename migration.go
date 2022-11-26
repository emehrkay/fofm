package fofm

import (
	"sort"
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

func (m *Migration) TimestampFromString(timeStr string) error {
	if strings.TrimSpace(timeStr) == "" {
		return nil
	}

	var err error
	m.Timestamp, err = time.Parse(time.RFC1123Z, timeStr)
	return err
}

func (m *Migration) CreatedFromString(timeStr string) error {
	if strings.TrimSpace(timeStr) == "" {
		return nil
	}

	var err error
	m.Created, err = time.Parse(time.RFC1123Z, timeStr)
	return err
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

func (m MigrationStack) BeforeName(name string) MigrationStack {
	stack := MigrationStack{}

	if len(m) == 0 {
		return stack
	}

	var i int
	var mig Migration

	for i, mig = range m {
		if mig.Name == name {
			break
		}
	}

	i += 1
	stack = m[:i]

	return stack
}

func (m MigrationStack) After(after *Migration) MigrationStack {
	if after == nil {
		return m
	}

	var i int
	var mig Migration
	stack := MigrationStack{}

	if len(m) == 0 {
		return stack
	}

	if after != nil {
		for i, mig = range m {
			if mig.Timestamp.After(after.Timestamp) {
				break
			}
		}

		i += 1
	}

	stack = append(stack, m[:i]...)

	return stack
}
