package mssql

import (
	"database/sql"
	"os"
	"testing"
	"time"
	"fmt"

	"github.com/komiflo/migrate/file"
	"github.com/komiflo/migrate/migrate/direction"
	pipep "github.com/komiflo/migrate/pipe"
)

// TestMigrate runs some additional tests on Migrate().
// Basic testing is already done in migrate/migrate_test.go
func TestMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	host := os.Getenv("MSSQL_PORT_1433_TCP_ADDR")
	port := os.Getenv("MSSQL_PORT_1433_TCP_PORT")
	driverURL := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s;encrypt=disable;log=2;TrustServerCertificate=true",
		host,
		"sa",
		"Passw0rd",
		port,
		"master",
	)
	// retry connection for 2 minutes
	until := time.Now().Add(time.Second * 120)

	ticker := time.NewTicker(time.Second)
	var connection *sql.DB
	var err error
	// prepare clean database
	for tick := range ticker.C {
		if tick.After(until) {
			ticker.Stop()
			break;
		}
		connection, err = sql.Open("mssql", driverURL)
		err = connection.Ping()
		if err == nil {
			ticker.Stop()
			break;
		}
	}

	if err != nil {
		t.Fatal("Failed to connect to mssql docker container after 2 minutes", err)
	}

	if _, err := connection.Exec(`
				DROP TABLE IF EXISTS yolo;
				DROP TABLE IF EXISTS ` + tableName + `;`); err != nil {
		t.Fatal(err)
	}

	d := &Driver{}
	if err := d.Initialize(driverURL); err != nil {
		t.Fatal(err)
	}

	// testing idempotency: second call should be a no-op, since table already exists
	if err := d.Initialize(driverURL); err != nil {
		t.Fatal(err)
	}

	files := []file.File{
		{
			Path:      "/foobar",
			FileName:  "001_foobar.up.sql",
			Version:   1,
			Name:      "foobar",
			Direction: direction.Up,
			Content: []byte(`
				CREATE TABLE yolo (
					id BIGINT IDENTITY NOT NULL PRIMARY KEY
				);
			`),
		},
		{
			Path:      "/foobar",
			FileName:  "002_foobar.down.sql",
			Version:   1,
			Name:      "foobar",
			Direction: direction.Down,
			Content: []byte(`
				DROP TABLE yolo;
			`),
		},
		{
			Path:      "/foobar",
			FileName:  "002_foobar.up.sql",
			Version:   2,
			Name:      "foobar",
			Direction: direction.Up,
			Content: []byte(`
				CREATE TABLE error (
					id THIS WILL CAUSE AN ERROR
				)
			`),
		},
		{
			Path:      "/foobar",
			FileName:  "20170118205923_demo.up.sql",
			Version:   20170118205923,
			Name:      "demo",
			Direction: direction.Up,
			Content: []byte(`
			CREATE TABLE demo (
				id BIGINT IDENTITY NOT NULL PRIMARY KEY
			)
			`),
		},
		{
			Path:      "/foobar",
			FileName:  "20170118205923_demo.down.sql",
			Version:   20170118205923,
			Name:      "demo",
			Direction: direction.Down,
			Content: []byte(`
				DROP TABLE demo
			`),
		},
	}

	pipe := pipep.New()
	go d.Migrate(files[0], pipe)
	errs := pipep.ReadErrors(pipe)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	pipe = pipep.New()
	go d.Migrate(files[1], pipe)
	errs = pipep.ReadErrors(pipe)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	pipe = pipep.New()
	go d.Migrate(files[2], pipe)
	errs = pipep.ReadErrors(pipe)
	if len(errs) == 0 {
		t.Error("Expected test case to fail")
	}

	pipe = pipep.New()
	go d.Migrate(files[3], pipe)
	errs = pipep.ReadErrors(pipe)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	pipe = pipep.New()
	go d.Migrate(files[4], pipe)
	errs = pipep.ReadErrors(pipe)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}
