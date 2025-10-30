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
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=require",
		os.Getenv("NEW_POSTGRES_DATABASE_HOST"),
		os.Getenv("NEW_POSTGRES_DATABASE_USER"),
		os.Getenv("NEW_POSTGRES_DATABASE_PASSWORD"),
		os.Getenv("NEW_POSTGRES_DATABASE_DATA"),
		os.Getenv("NEW_POSTGRES_DATABASE_PORT"),
	)

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Database connected")

	// Migrate the schema
	DB.AutoMigrate(&Item{})
}
