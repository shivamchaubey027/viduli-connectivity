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
	password := url.QueryEscape(os.Getenv("NEW_POSTGRES_DATABASE_PASSWORD"))
	dbname := os.Getenv("NEW_POSTGRES_DATABASE_DATABASE")
	port := os.Getenv("NEW_POSTGRES_DATABASE_PORT")

	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Database connected")

	// Migrate the schema
	DB.AutoMigrate(&Item{})
}
