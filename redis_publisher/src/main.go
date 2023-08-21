/*
This is based off the redis demo I've made here: https://github.com/felixfyx/go-redis-demo

Based on the server
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type LogText struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func main() {
	// Read config off env to get redis host and port number
	var host = os.Getenv("redis_host")
	var port = os.Getenv("redis_port")

	fmt.Println("--- Connecting to Redis Publisher ---")
	var redisClient = redis.NewClient(&redis.Options{
		Addr: host + ":" + port,
	})

	fmt.Println("Running Publisher...")
	app := fiber.New()
	var ctx = context.Background()

	app.Post("/", func(c *fiber.Ctx) error {
		logtext := new(LogText)

		if err := c.BodyParser(logtext); err != nil {
			panic(err)
		}

		payload, err := json.Marshal(logtext)
		if err != nil {
			panic(err)
		}

		if err := redisClient.Publish(ctx, "send-log-data", payload).Err(); err != nil {
			panic(err)
		}

		return c.SendStatus(200)
	})

	// HACK: Hardcoded for now..
	app.Listen(":8080")
}
