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
		if err != nil {
			log.Println("failed to open db:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping()
		if err == nil {
			log.Println("connected to db")
			return db
		}

		log.Println("db not ready, retrying...", err)
		time.Sleep(2 * time.Second)
	}

	log.Fatal("could not connect to db:", err)
	return nil
}
