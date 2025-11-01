package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

type Item struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Name        string
	Description string
}

func ConnectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Build DSN from individual environment variables if DATABASE_URL isn't set
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		sslmode := os.Getenv("SSL_MODE")

		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "5432"
		}
		if user == "" {
			user = "postgres"
		}
		if password == "" {
			password = "postgres"
		}
		if dbname == "" {
			dbname = "postgres"
		}
		if sslmode == "" {
			// default to disable for local development; change to "require" in production if needed
			sslmode = "disable"
		}

		dsn = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			host, user, password, dbname, port, sslmode,
		)

		// Log a sanitized DSN (do not print password)
		sanitized := fmt.Sprintf(
			"host=%s user=%s dbname=%s port=%s sslmode=%s",
			host, user, dbname, port, sslmode,
		)
		log.Printf("Using DSN: %s", sanitized)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Migrate the schema
	DB.AutoMigrate(&Item{})
}
