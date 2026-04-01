package main

import (
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// Declare the same fanout exchange
	err = ch.ExchangeDeclare(
		"logs",   // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	// Declare a temporary, exclusive queue
	q, err := ch.QueueDeclare(
		"",    // name (empty = auto-generated)
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Bind queue to the fanout exchange
	err = ch.QueueBind(
		q.Name, // queue name
		"",     // routing key (ignored for fanout)
		"logs", // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	// Open log file for writing
	logFile, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %s", err)
	}
	defer logFile.Close()

	var forever chan struct{} = make(chan struct{})

	go func() {
		for d := range msgs {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			logEntry := fmt.Sprintf("[%s] %s\n", timestamp, d.Body)

			// Write to file
			if _, err := logFile.WriteString(logEntry); err != nil {
				log.Printf("Failed to write to log file: %s", err)
			}

			// Also print to console
			fmt.Printf(" [FILE] Written to logs.txt: %s", logEntry)
		}
	}()

	fmt.Printf(" [*] Waiting for logs (File Writer). To exit press CTRL+C\n")
	<-forever
}
