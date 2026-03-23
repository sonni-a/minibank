package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func Connect(connStr string) *sql.DB {
	var db *sql.DB
	var err error

	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				return db
			}
		}

		log.Println("waiting for db...")
		time.Sleep(2 * time.Second)
	}

	log.Fatal(err)
	return nil
}
