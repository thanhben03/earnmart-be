package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/earnmart/earnmart-be/internal/config"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("load configuration", err)
	}

	db, err := sql.Open("mysql", cfg.Database.DSN()+"&multiStatements=true")
	if err != nil {
		fatal("open mysql", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fatal("connect to mysql", err)
	}

	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		fatal("create mysql migration driver", err)
	}

	migrationsURL := os.Getenv("MIGRATIONS_URL")
	if migrationsURL == "" {
		migrationsURL = "file://migrations"
	}

	runner, err := migrate.NewWithDatabaseInstance(migrationsURL, cfg.Database.Name, driver)
	if err != nil {
		fatal("create migration runner", err)
	}
	defer runner.Close()

	switch os.Args[1] {
	case "up":
		err = runUp(runner, os.Args[2:])
	case "down":
		err = runDown(runner, os.Args[2:])
	case "version":
		err = printVersion(runner)
	default:
		usage()
		os.Exit(2)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("No migration changes to apply.")
		return
	}
	if err != nil {
		fatal("run migration", err)
	}
}

func runUp(runner *migrate.Migrate, args []string) error {
	if len(args) == 0 {
		if err := runner.Up(); err != nil {
			return err
		}
		fmt.Println("All pending migrations applied.")
		return nil
	}

	steps, err := positiveSteps(args[0])
	if err != nil {
		return err
	}
	if err := runner.Steps(steps); err != nil {
		return err
	}
	fmt.Printf("Applied %d migration(s).\n", steps)
	return nil
}

func runDown(runner *migrate.Migrate, args []string) error {
	steps := 1
	var err error
	if len(args) > 0 {
		steps, err = positiveSteps(args[0])
		if err != nil {
			return err
		}
	}
	if err := runner.Steps(-steps); err != nil {
		return err
	}
	fmt.Printf("Rolled back %d migration(s).\n", steps)
	return nil
}

func printVersion(runner *migrate.Migrate) error {
	version, dirty, err := runner.Version()
	if err != nil {
		return err
	}
	fmt.Printf("Migration version: %d (dirty: %t)\n", version, dirty)
	return nil
}

func positiveSteps(value string) (int, error) {
	steps, err := strconv.Atoi(value)
	if err != nil || steps < 1 {
		return 0, fmt.Errorf("steps must be a positive integer")
	}
	return steps, nil
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/migrate up [steps]")
	fmt.Println("  go run ./cmd/migrate down [steps]")
	fmt.Println("  go run ./cmd/migrate version")
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
