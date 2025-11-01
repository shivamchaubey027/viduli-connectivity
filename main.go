package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db    *gorm.DB
	cache *redis.Client
	ctx   = context.Background()
)

type Item struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func connectDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"
	}
	log.Printf("DSN being used: '%s'", dsn)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("Failed to connect to database: %v", err)
	}

	db.AutoMigrate(&Item{})
	return nil
}

func connectCache() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func main() {
	_ = godotenv.Load()

	// DB
	if err := connectDB(); err != nil {
		log.Println("DB not available, running without persistent DB:", err)
	} else {
		log.Println("Connected to DB, running migrations")
		if err := db.AutoMigrate(&Item{}); err != nil {
			log.Printf("AutoMigrate warning: %v", err)
		}
	}

	// Redis
	c, err := connectCache()
	if err != nil {
		log.Println("Redis not available, continuing without cache:", err)
		cache = nil
	} else {
		cache = c
		log.Println("Connected to Redis")
	}

	router := gin.Default()

	router.Static("/assets", "./public/assets")
	router.GET("/", func(c *gin.Context) {
		if _, err := os.Stat("public/index.html"); err == nil {
			c.File("public/index.html")
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Hi from Viduli"})
	})
	router.NoRoute(func(c *gin.Context) {
		if _, err := os.Stat("public/index.html"); err == nil {
			c.File("public/index.html")
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
	})

	api := router.Group("/api")
	{
		api.GET("/items", getItems)
		api.POST("/items", createItem)
		api.GET("/items/:id", getItem)
		api.PUT("/items/:id", updateItem)
		api.DELETE("/items/:id", deleteItem)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func getItems(c *gin.Context) {
	var items []Item
	if db != nil {
		db.Find(&items)
		c.JSON(http.StatusOK, items)
		return
	}
	c.JSON(http.StatusOK, items)
}

func createItem(c *gin.Context) {
	var item Item
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt

	if db != nil {
		if err := db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB create failed"})
			return
		}
		c.JSON(http.StatusCreated, item)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func getItem(c *gin.Context) {
	var item Item
	id := c.Param("id")

	// cache hit
	if cache != nil {
		if val, err := cache.Get(ctx, "item:"+id).Result(); err == nil && strings.TrimSpace(val) != "" {
			c.Data(http.StatusOK, "application/json", []byte(val))
			return
		}
	}

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if err := db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if cache != nil {
		if jsonItem, err := json.Marshal(item); err == nil {
			cache.Set(ctx, "item:"+id, jsonItem, 10*time.Minute)
		}
	}

	c.JSON(http.StatusOK, item)
}

func updateItem(c *gin.Context) {
	var item Item
	id := c.Param("id")

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if err := db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item.UpdatedAt = time.Now()
	db.Save(&item)
	if cache != nil {
		cache.Del(ctx, "item:"+id)
	}
	c.JSON(http.StatusOK, item)
}

func deleteItem(c *gin.Context) {
	var item Item
	id := c.Param("id")

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	if err := db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	db.Delete(&item)
	if cache != nil {
		cache.Del(ctx, "item:"+id)
	}
	c.Status(http.StatusNoContent)
}
