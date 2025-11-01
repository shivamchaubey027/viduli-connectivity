package main

import (
	"context"
	"encoding/json"
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

// Todo model
type Todo struct {
	ID        uint      `json:"id" gorm:"primary_key"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func connectDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
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
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
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

	// try a few times
	var err error
	for i := 1; i <= 3; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, derr := db.DB()
			if derr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					return nil
				} else {
					err = pingErr
				}
			} else {
				err = derr
			}
		}
		log.Printf("DB connect attempt %d failed: %v", i, err)
		time.Sleep(time.Duration(i) * time.Second)
	}
	return err
}

func connectCache() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6380"
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
		if err := db.AutoMigrate(&Todo{}); err != nil {
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
		api.GET("/todos", getTodos)
		api.POST("/todos", createTodo)
		api.GET("/todos/:id", getTodo)
		api.PUT("/todos/:id", updateTodo)
		api.DELETE("/todos/:id", deleteTodo)
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

func getTodos(c *gin.Context) {
	var todos []Todo
	if db != nil {
		db.Find(&todos)
		c.JSON(http.StatusOK, todos)
		return
	}
	c.JSON(http.StatusOK, todos) // empty if no DB
}

func createTodo(c *gin.Context) {
	var todo Todo
	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if db != nil {
		if err := db.Create(&todo).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB create failed"})
			return
		}
		c.JSON(http.StatusCreated, todo)
		return
	}
	// no persistent DB: set timestamps and return created id = 0
	todo.CreatedAt = time.Now()
	todo.UpdatedAt = todo.CreatedAt
	c.JSON(http.StatusCreated, todo)
}

func getTodo(c *gin.Context) {
	var todo Todo
	id := c.Param("id")

	// cache hit
	if cache != nil {
		if val, err := cache.Get(ctx, "todo:"+id).Result(); err == nil && strings.TrimSpace(val) != "" {
			c.Data(http.StatusOK, "application/json", []byte(val))
			return
		}
	}

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if cache != nil {
		if jsonTodo, err := json.Marshal(todo); err == nil {
			cache.Set(ctx, "todo:"+id, jsonTodo, 10*time.Minute)
		}
	}

	c.JSON(http.StatusOK, todo)
}

func updateTodo(c *gin.Context) {
	var todo Todo
	id := c.Param("id")

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.Save(&todo)
	if cache != nil {
		cache.Del(ctx, "todo:"+id)
	}
	c.JSON(http.StatusOK, todo)
}

func deleteTodo(c *gin.Context) {
	var todo Todo
	id := c.Param("id")

	if db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	db.Delete(&todo)
	if cache != nil {
		cache.Del(ctx, "todo:"+id)
	}
	c.Status(http.StatusNoContent)
}
