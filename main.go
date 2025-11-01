package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
	"viduli-test-app/database"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var (
	rdb *redis.Client
	ctx = context.Background()
)

func main() {
	// Load .env file
	var err error
	err = godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to PostgreSQL
	database.ConnectDB()

	// Connect to Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Could not parse Redis URL: %v", err)
	}
	rdb = redis.NewClient(opt)

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Item endpoints
	r.POST("/items", createItem)
	r.GET("items", getItems)

	// Cache endpoints
	r.POST("/cache", setCache)
	r.GET("/cache/:key", getCache)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}

	log.Println("--- DEBUGGING ENV VARS ---")
	log.Printf("DB_HOST: [%s]", os.Getenv("DB_HOST"))
	log.Printf("DB_USER: [%s]", os.Getenv("DB_USER"))
	log.Printf("DB_NAME: [%s]", os.Getenv("DB_NAME"))
	log.Printf("SSL_MODE: [%s]", os.Getenv("SSL_MODE"))
	log.Println("----------------------------")
}

// POST /items
func createItem(c *gin.Context) {
	var newItem database.Item
	if err := c.ShouldBindJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newItem.CreatedAt = time.Now()

	if err := database.DB.Create(&newItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create item"})
		return
	}

	c.JSON(http.StatusCreated, newItem)
}

// GET /items
func getItems(c *gin.Context) {
	var items []database.Item
	if err := database.DB.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve items"})
		return
	}

	c.JSON(http.StatusOK, items)
}

// POST /cache
func setCache(c *gin.Context) {
	var newCacheItem struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := c.ShouldBindJSON(&newCacheItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := rdb.Set(ctx, newCacheItem.Key, newCacheItem.Value, 1*time.Minute).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /cache/:key
func getCache(c *gin.Context) {
	key := c.Param("key")
	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"value": val})
}
