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

	// Declare a Fanout Exchange
	// Fanout exchange broadcasts all messages to all bound queues
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

	body := bodyFrom(os.Args)

	// Publish to exchange (not directly to queue)
	// Routing key is ignored for fanout exchange
	err = ch.Publish(
		"logs", // exchange
		"",     // routing key (ignored for fanout)
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	failOnError(err, "Failed to publish a message")

	fmt.Printf(" [x] Sent %s\n", body)
}

func bodyFrom(args []string) string {
	var s string
	if len(args) < 2 || os.Args[1] == "" {
		s = "info: Hello World!"
	} else {
		s = strings.Join(args[1:], " ")
	}
	return s
}
