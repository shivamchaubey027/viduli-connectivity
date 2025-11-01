package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Item struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
}

var (
	DB       *gorm.DB
	memLock  sync.Mutex
	memStore      = make([]Item, 0)
	nextID   uint = 1
)

func ConnectDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "postgres"
		}
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			password = "postgres"
		}
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = "postgres"
		}
		sslmode := os.Getenv("SSL_MODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		dsn = "host=" + host +
			" user=" + user +
			" password=" + password +
			" dbname=" + dbname +
			" port=" + port +
			" sslmode=" + sslmode
	}

	// try a couple of times briefly
	var err error
	for i := 1; i <= 3; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, derr := DB.DB()
			if derr == nil {
				pingErr := sqlDB.Ping()
				if pingErr == nil {
					if amErr := DB.AutoMigrate(&Item{}); amErr != nil {
						log.Printf("AutoMigrate warning: %v", amErr)
					}
					return nil
				}
				err = pingErr
			} else {
				err = derr
			}
		}
		log.Printf("DB connect attempt %d failed: %v", i, err)
		time.Sleep(time.Duration(i) * time.Second)
	}
	return err
}

func main() {
	_ = godotenv.Load() // optional .env

	if err := ConnectDB(); err != nil {
		log.Println("DB not available, running with in-memory store:", err)
	} else {
		log.Println("Connected to DB")
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/items", createItem)
	r.GET("/items", getItems)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func createItem(c *gin.Context) {
	var it Item
	if err := c.ShouldBindJSON(&it); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	it.CreatedAt = time.Now()
	it.UpdatedAt = it.CreatedAt

	if DB != nil {
		if err := DB.Create(&it).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB create failed"})
			return
		}
		c.JSON(http.StatusCreated, it)
		return
	}

	// in-memory fallback
	memLock.Lock()
	it.ID = nextID
	nextID++
	memStore = append(memStore, it)
	memLock.Unlock()

	c.JSON(http.StatusCreated, it)
}

func getItems(c *gin.Context) {
	if DB != nil {
		var items []Item
		if err := DB.Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB query failed"})
			return
		}
		c.JSON(http.StatusOK, items)
		return
	}

	memLock.Lock()
	items := make([]Item, len(memStore))
	copy(items, memStore)
	memLock.Unlock()

	c.JSON(http.StatusOK, items)
}
