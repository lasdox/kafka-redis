package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
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

var (
	mechanism = plain.Mechanism{
		Username: "test",
		Password: "password",
	}
	dialer = &kafka.Dialer{
		DualStack:     true,
		SASLMechanism: mechanism,
	}
	transport = &kafka.Transport{
		SASL: mechanism,
	}
	kafkaWriter *kafka.Writer
)

func main() {
	//createTopic()
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
		time.Sleep(2 * time.Second)
	}

	// Prevent main from exiting
	//select {}
}

func createTopic() {
	var controllerConn *kafka.Conn
	controllerConn, err := dialer.Dial("tcp", kafkaBroker)
	if err != nil {
		panic(err.Error())
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             kafkaTopic,
			NumPartitions:     4,
			ReplicationFactor: 1,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		panic(err.Error())
	}
}

func sendKafkaMessages() {
	messageKeys := []string{"aosdfji", "poosdakf", "accountId", "accountId", "accountId", "accountId", "accountId"}
	for i, messageKey := range messageKeys {
		msg := fmt.Sprintf("msg-%d", i)
		if err := kafkaWriter.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(messageKey),
			Value: []byte(msg),
			Headers: []kafka.Header{
				{
					Key:   "timestamp",
					Value: []byte(fmt.Sprintf("%d", time.Now().Unix())),
				},
			},
		}); err != nil {
			log.Println("Kafka Producer Error:", err)
		}
		//l.Infof("Produced to Kafka with key: %s, index: %d", messageKey, i)
	}
}

func startKafkaProducer() *kafka.Writer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        kafkaTopic,
		Balancer:     &kafka.Hash{},
		Transport:    transport,
		BatchTimeout: 100 * time.Millisecond,
	}
	return writer
}

func startKafkaConsumer() *kafka.Reader {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBroker},
		Topic:   kafkaTopic,
		GroupID: "group_1",
		Dialer:  dialer,
	})
	l.Infof("Started kafka consumer: %s listening on partition: %s", r.Stats().ClientID, r.Stats().Partition)

	for {
		msg, err := r.ReadMessage(context.Background())

		var timestamp int64
		for _, header := range msg.Headers {
			if header.Key == "timestamp" {
				timestampStr := string(header.Value)
				timestampInt, _ := strconv.Atoi(timestampStr)
				timestamp = int64(timestampInt)
				break
			}
		}
		msgTime := time.Unix(timestamp, 0)

		if err != nil {
			l.Error("Kafka Consumer Error:", err)
			continue
		}
		l.Infof("Kafka Consumer %d received: Key=%s, Value=%s, Partition=%d, Latency=%s", r.Stats().ClientID, string(msg.Key), string(msg.Value), msg.Partition, time.Now().Sub(msgTime))
	}
}

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
