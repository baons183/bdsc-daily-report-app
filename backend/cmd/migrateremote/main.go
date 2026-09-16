// Command migrateremote provisions the BDSC_DailyReport schema on a remote
// SQL Server — used once to set up the 192.168.99.37 backup target before
// scripts/backup-mirror-to-99937.mjs can sync data into it. The target
// database itself and a SQL login with rights on it must already exist;
// this only creates the tables, reusing db.Migrate (the exact same
// idempotent schema the local app runs on every startup), so the remote
// copy is guaranteed identical to the local one.
//
// Usage:
//
//	go run ./cmd/migrateremote -server=192.168.99.37 -database=BDSC_DailyReport -username=... -password=...
package main

import (
	"flag"
	"log"

	"bdscdailyreport/internal/config"
	"bdscdailyreport/internal/db"
)

func main() {
	server := flag.String("server", "", "SQL Server host (vd 192.168.99.37)")
	port := flag.String("port", "1433", "SQL Server port")
	database := flag.String("database", "BDSC_DailyReport", "Ten database")
	username := flag.String("username", "", "SQL login")
	password := flag.String("password", "", "SQL password")
	flag.Parse()

	if *server == "" || *username == "" || *password == "" {
		log.Fatal("thieu -server/-username/-password")
	}

	cfg := config.Config{
		DBServer:   *server,
		DBPort:     *port,
		DBDatabase: *database,
		DBUsername: *username,
		DBPassword: *password,
	}
	conn, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("khong ket noi duoc %s/%s: %v", *server, *database, err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("migrate that bai: %v", err)
	}
	log.Printf("Da tao/cap nhat schema tren %s/%s thanh cong.", *server, *database)
}
