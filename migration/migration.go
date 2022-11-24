package migration

import (
	"sort"
	"time"
)

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

type MigrationStack []Migration

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

func (m MigrationStack) After(after *Migration) MigrationStack {
	var i int
	var mig Migration
	stack := MigrationStack{}

	if after != nil {
		for i, mig = range m {
			if mig.Timestamp.After(after.Timestamp) {
				break
			}
		}
	}

	stack = append(stack, m[:i]...)

	return stack
}
