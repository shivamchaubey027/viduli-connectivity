package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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
	var host, port, password string

	if dsn == "" {
		host = os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port = os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "postgres"
		}
		password = os.Getenv("DB_PASSWORD")
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
	} else {
		// parse DATABASE_URL for host/port/password for diagnostics
		if u, err := url.Parse(dsn); err == nil {
			if h := u.Hostname(); h != "" {
				host = h
			}
			if p := u.Port(); p != "" {
				port = p
			} else {
				port = "5432"
			}
			if u.User != nil {
				if pw, ok := u.User.Password(); ok {
					password = pw
				}
			}
		}
		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "5432"
		}
	}

	// sanitized DSN for logs (don't print password)
	safeDSN := strings.ReplaceAll(dsn, "password="+password, "password=REDACTED")
	log.Printf("connectDB: attempting with DSN: %s", safeDSN)
	addr := net.JoinHostPort(host, port)

	// raw TCP check
	dialErr := func() error {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}()

	if dialErr != nil {
		log.Printf("connectDB: RAW TCP connect to %s failed: %v", addr, dialErr)
	} else {
		log.Printf("connectDB: RAW TCP connect to %s succeeded", addr)
	}

	// try GORM open + ping with retries and clear logging
	var openErr error
	for i := 1; i <= 3; i++ {
		db, openErr = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if openErr == nil {
			sqlDB, derr := db.DB()
			if derr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					log.Printf("connectDB: connected to DB on attempt %d", i)
					return nil
				} else {
					openErr = pingErr
				}
			} else {
				openErr = derr
			}
		}
		log.Printf("connectDB: gorm attempt %d failed: %v", i, openErr)
		time.Sleep(time.Duration(i) * time.Second)
	}

	// helpful guidance in error
	if dialErr != nil {
		return fmt.Errorf("gorm/connect failed: %w; raw tcp error: %v", openErr, dialErr)
	}
	return openErr
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
	// quick ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func main() {
	_ = godotenv.Load() // optional .env

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

	// serve static assets if present
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

	// no persistent DB: return created item (ID will be zero)
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
