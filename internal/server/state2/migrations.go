package state2

import (
	"embed"
	"fmt"
)

//go:embed schema/*.sql
var migrations embed.FS

func (s *State) applyMigrations() error {
	txn, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	files, err := migrations.ReadDir("schema")
	if err != nil {
		return err
	}
	for _, file := range files {
		data, err := migrations.ReadFile("schema/" + file.Name())
		if err != nil {
			return err
		}
		if _, err := txn.Exec(string(data)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %v", file.Name(), err)
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}
