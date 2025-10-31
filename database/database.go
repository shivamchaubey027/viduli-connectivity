package database

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Item struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

var DB *gorm.DB

func ConnectDB() {
	var err error

	host := os.Getenv("NEW_POSTGRES_DATABASE_HOST")
	user := os.Getenv("NEW_POSTGRES_DATABASE_USER")
	rawPassword := os.Getenv("NEW_POSTGRES_DATABASE_PASSWORD")
	dbname := os.Getenv("NEW_POSTGRES_DATABASE_DATABASE")
	port := os.Getenv("NEW_POSTGRES_DATABASE_PORT")

	// Properly encode for URL
	encodedUser := url.QueryEscape(user)
	encodedPassword := url.QueryEscape(rawPassword)

	// Try with sslmode=require first
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=require",
		encodedUser, encodedPassword, host, port, dbname)

	fmt.Printf("Attempting connection to: postgresql://%s:****@%s:%s/%s\n", user, host, port, dbname)

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		// If sslmode=require fails, try prefer
		fmt.Println("Retrying with sslmode=prefer...")
		dsn = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=prefer",
			encodedUser, encodedPassword, host, port, dbname)

		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
	}

	fmt.Println("Database connected")
	DB.AutoMigrate(&Item{})
}
