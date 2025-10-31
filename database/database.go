package database

import (
	"fmt"
	"log"
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

	// Use the full URL provided by Orbit DB
	dsn_debug := os.Getenv("NEW_POSTGRES_DATABASE_URL")
	log.Printf("DEBUG DSN: [%s]", dsn_debug)

	if dsn_debug == "" {
		log.Fatal("NEW_POSTGRES_DATABASE_URL environment variable not set")
	}

	fmt.Printf("Connecting using provided URL\n")

	DB, err = gorm.Open(postgres.Open(dsn_debug), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Database connected successfully!")
	DB.AutoMigrate(&Item{})
}
