package main

import (
	"fmt"
	"log"
	"os"
	"strings"

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

	// Declare a Direct Exchange
	// Direct exchange routes messages based on routing key
	err = ch.ExchangeDeclare(
		"logs_direct", // name
		"direct",      // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	// Get severity (routing key) from command line
	severity := severityFrom(os.Args)
	body := bodyFrom(os.Args)

	// Publish with routing key = severity level
	// Only queues bound to this routing key will receive the message
	err = ch.Publish(
		"logs_direct", // exchange
		severity,      // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	failOnError(err, "Failed to publish a message")

	fmt.Printf(" [x] Sent [%s] %s\n", severity, body)
}

func severityFrom(args []string) string {
	var s string
	if len(args) < 2 || os.Args[1] == "" {
		s = "info"
	} else {
		s = os.Args[1]
	}
	return s
}

func bodyFrom(args []string) string {
	var s string
	if len(args) < 3 || os.Args[2] == "" {
		s = "Hello World!"
	} else {
		s = strings.Join(args[2:], " ")
	}
	return s
}
