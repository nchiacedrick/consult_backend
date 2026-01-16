package db

import (
	"context"
	"database/sql"
	"time"
)

// create database connection
func New(addr string, maxOpenConns, maxIdleConns int, maxIdleTime string) (*sql.DB, error) {
	db, err := sql.Open("postgres", addr)
	if err != nil {
		return nil, err
	}

	// set maxIdleTime value
	duration, err := time.ParseDuration(maxIdleTime)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //if it takes more thatn 5seconds to connect we have a timeout
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
}
