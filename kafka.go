package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	l "github.com/sirupsen/logrus"
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
		l.Infof("Kafka Stream Consumer received: Key=%s, Value=%s, Partition=%d, Latency=%s", string(msg.Key), string(msg.Value), msg.Partition, time.Now().Sub(msgTime))
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

func sendKafkaMessages() {
	messageKeys := []string{"UO9JEgfeQUGLrqAsIrR1Ug", "UO9JEgfeQUGLrqAsIrR1Ug", "NWR7SyTESuC5NnJdASUFNg", "NWR7SyTESuC5NnJdASUFNg", "NWR7SyTESuC5NnJdASUFNg", "UO9JEgfeQUGLrqAsIrR1Ug", "NWR7SyTESuC5NnJdASUFNg"}
	for i, messageKey := range messageKeys {
		msg := fmt.Sprintf("msg-order-%d", i)
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
