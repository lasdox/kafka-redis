package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	l "github.com/sirupsen/logrus"
)

func startRedisClient() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	status := client.Ping(context.Background())
	if status.Err() != nil {
		log.Fatal("error while starting redis client", status.Err())
	}
	return client
}

func sendRedisStreamMessages(client *redis.Client) {
	pipe := client.Pipeline()

	for i := 0; i < 5; i++ {
		msg := fmt.Sprintf("msg-%d", i) // You can also use unique message identifiers
		timestampStr := fmt.Sprintf("%d", time.Now().Unix())
		pipe.XAdd(context.Background(), &redis.XAddArgs{
			Stream: redisStream,
			Values: map[string]interface{}{"message": msg, "timestamp": timestampStr},
		})
	}

	_, err := pipe.Exec(context.Background())
	if err != nil {
		log.Println("Redis Stream Pipeline Error:", err)
	} else {
		fmt.Println("All messages successfully produced to Redis Stream")
	}
}

func startRedisStreamConsumer(client *redis.Client, id int) {
	// Ensure the consumer group exists
	if _, err := client.XGroupCreateMkStream(context.Background(), redisStream, redisGroup, "$").Result(); err != nil {
		if !redisErrGroupExists(err) {
			log.Fatalf("Could not create Redis Stream Consumer Group: %v", err)
		}
	}

	for {
		msgs, err := client.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Group:    redisGroup,
			Consumer: fmt.Sprintf("%s-%d", redisConsumer, id),
			Streams:  []string{redisStream, ">"},
			Count:    10,
			Block:    0,
		}).Result()
		if err != nil {
			l.Error("Redis Stream Consumer Error:", err)
			continue
		}
		for _, stream := range msgs {
			for _, message := range stream.Messages {
				val := message.Values["message"]
				timestampStr := message.Values["timestamp"]
				timestamp, _ := strconv.Atoi(timestampStr.(string))
				ts64 := int64(timestamp)
				timeValue := time.Unix(ts64, 0)
				l.Infof("Redis Stream Consumer %d received: Value=%v, Latency=%s", id, val, time.Now().Sub(timeValue))
				client.XAck(context.Background(), redisStream, redisGroup, message.ID)
			}
		}
	}
}

func redisErrGroupExists(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}
