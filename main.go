package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	moduleboilerplate "github.com/Twibbonize/go-module-boilerplate-mongodb"

	"github.com/redis/go-redis/v9"
)

const (
	blank = ""
)

type SuccessullResponse struct {
	Data map[string]interface{} `json:"Data"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error
}

var redisClient redis.UniversalClient
var loggerMain *slog.Logger

func initLogger() {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelDebug)

	loggerMain = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	}))
}

func connectRedis() redis.UniversalClient {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		log.Fatal("REDIS_HOST environment variable not set")
	}

	if os.Getenv("APP_ENV") == "production" {
		clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    []string{redisHost},
			Password: os.Getenv("REDIS_PASS"),
		})

		_, err := clusterClient.Ping(context.Background()).Result()
		if err != nil {
			log.Fatal(err)
		}

		return clusterClient
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: os.Getenv("REDIS_PASS"),
		DB:       0,
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal(err)
	}

	return client
}

func main() {

	initLogger()

	redisClient = connectRedis()

	app := fiber.New()
	envOrigins := os.Getenv("ORIGIN")
	envOrigins = strings.Replace(envOrigins, "*", "", -1)

	originList := strings.Split(envOrigins, ",")
	allowedOrigins := append(originList, "http://localhost:5174", "https://localhost:5174")

	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(allowedOrigins, ","),
		AllowMethods:     "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD",
		AllowHeaders:     "Origin, Content-Type, Content-Length, Authorization, User-Agent, Accept, Referer, X-Requested-With",
		ExposeHeaders:    "Content-Length",
		AllowCredentials: true,
		MaxAge:           43200, // 12 hours
	}))

	app.Get("/:uuid", func(c *fiber.Ctx) error {
		anyModuleGetter := moduleboilerplate.NewGetterLib(&redisClient)
		return GetOne(c, *anyModuleGetter)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		anyModuleGetter := moduleboilerplate.NewGetterLib(&redisClient)
		return GetMany(c, *anyModuleGetter)
	})

	defer func() {

		if c, ok := redisClient.(io.Closer); ok {

			c.Close()
		} else {
			panic(errors.New("Failed to close redis connection"))
		}
	}()

	port := os.Getenv("GIN_PORT")

	app.Listen(":" + port)
}
