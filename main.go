package main

import (
	"time"

	l "github.com/sirupsen/logrus"
)

const (
	kafkaTopic    = "test_topic"
	kafkaBroker   = "localhost:39092"
	redisStream   = "test_stream"
	redisGroup    = "test_group"
	redisConsumer = "consumer"
	redisAddr     = "localhost:6379"
)

func main() {
	kafkaWriter = startKafkaProducer()
	l.Info(kafkaWriter)
	l.Info("Started Kafka Producer")

	redisClient := startRedisClient()
	l.Info("Started Redis Client")

	// Start multiple Kafka Consumers
	for i := 0; i < 2; i++ {
		go startKafkaConsumer()
	}

	// Start multiple Redis Stream Consumers
	for i := 0; i < 2; i++ {
		go startRedisStreamConsumer(redisClient, i)
	}

	// messages to both kafka and redis are sent concurrently
	for {
		go sendKafkaMessages()
		go sendRedisStreamMessages(redisClient)
		time.Sleep(5 * time.Second)
	}

	// Prevent main from exiting
	//select {}
}
