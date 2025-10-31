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
	port := os.Getenv("NEW_POSTGRES_DATABASE_PORT")
	user := os.Getenv("NEW_POSTGRES_DATABASE_USER")
	dbname := os.Getenv("NEW_POSTGRES_DATABASE_DATA")
	passwordRaw := os.Getenv("NEW_POSTGRES_DATABASE_PASSWORD")

	if host == "" || port == "" || user == "" || dbname == "" || passwordRaw == "" {
		log.Fatal("Database environment variables are not fully set")
	}

	password := url.QueryEscape(passwordRaw)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=require",
		host,
		user,
		password,
		dbname,
		port,
	)

	log.Println("Connecting to database...")
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully!")

	DB.AutoMigrate(&Item{})
	log.Println("Database migration complete.")
}
