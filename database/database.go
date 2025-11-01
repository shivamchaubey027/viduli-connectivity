package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Item represents a simple database model for migration.
type Item struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Name        string
	Description string
}

func ConnectDB() {
	var dsn string

	// Check if DATABASE_URL is provided (single connection string)
	dsn = os.Getenv("DATABASE_URL")

	// If not, build DSN from separate env vars
	if dsn == "" {
		host := os.Getenv("DB_HOST")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		port := os.Getenv("DB_PORT")
		sslmode := os.Getenv("SSL_MODE")

		if sslmode == "" {
			sslmode = "disable"
		}
		if port == "" {
			port = "5432"
		}

		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			host, user, password, dbname, port, sslmode)
	}

	// Fallback to localhost for local dev
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"
	}

	log.Printf("Connecting to database with DSN: %s", maskPassword(dsn))

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	DB.AutoMigrate(&Item{})
}

func maskPassword(dsn string) string {
	// Simple masking for logging - don't expose password
	// Just for debug purposes
	return strings.Replace(dsn, "password=", "password=***", 1)
}
