package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type DB struct {
	*gorm.DB
}

func New(dsn string) (*DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Second)

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (db *DB) AutoMigrate() error {
	if err := db.DB.AutoMigrate(
		&sessions.Session{},
		&sessions.SessionEvent{},
		&sessions.SessionMetrics{},
		&sessions.Pool{},
	); err != nil {
		return fmt.Errorf("migrate session tables: %w", err)
	}

	if err := db.DB.AutoMigrate(
		&workpool.WorkPool{},
	); err != nil {
		return fmt.Errorf("migrate workpool tables: %w", err)
	}

	return nil
}
