package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	var err error
	err = godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// database.ConnectDB()

	// redisURL := os.Getenv("REDIS_URL")
	// if redisURL == "" {
	// 	redisURL = "redis://localhost:6379/0"
	// }
	// opt, err := redis.ParseURL(redisURL)
	// if err != nil {
	// 	log.Fatalf("Could not parse Redis URL: %v", err)
	// }
	// rdb = redis.NewClient(opt)

	// _, err = rdb.Ping(ctx).Result()
	// if err != nil {
	// 	log.Fatalf("Could not connect to Redis: %v", err)
	// }
	// log.Println("Connected to Redis")

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// r.POST("/items", createItem)
	// r.GET("/items", getItems)

	r.POST("/cache", setCache)
	r.GET("/cache/:key", getCache)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	log.Printf("Server listening on port %s", port)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}

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

func getItems(c *gin.Context) {
	var items []database.Item
	if err := database.DB.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve items"})
		return
	}

	c.JSON(http.StatusOK, items)
}

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
