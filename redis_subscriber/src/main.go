/*
This is based off the redis demo I've made here: https://github.com/felixfyx/go-redis-demo

Based on the client
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

type LogText struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

var ctx = context.Background()

func main() {
	fmt.Println("Reading config...")
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file, %s", err)
	}

	// Create redisClient
	var host = viper.GetString("server.host")
	var port = viper.GetString("server.port")

	fmt.Println("--- Connecting to Redis server ---")
	var redisClient = redis.NewClient(&redis.Options{
		Addr: host + ":" + port,
	})

	fmt.Println("Running client...")

	subscriber := redisClient.Subscribe(ctx, "send-log-data")

	logtext := LogText{}

	for {
		msg, err := subscriber.ReceiveMessage(ctx)
		if err != nil {
			panic(err)
		}

		if err := json.Unmarshal([]byte(msg.Payload), &logtext); err != nil {
			panic(err)
		}

		fmt.Println("Received message from " + msg.Channel + " channel.")
		fmt.Printf("%+v\n", logtext)
	}
}
