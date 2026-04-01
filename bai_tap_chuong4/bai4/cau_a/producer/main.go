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
	// Connect to RabbitMQ server
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// Declare a queue
	q, err := ch.QueueDeclare(
		"task_queue", // name
		true,         // durable (queue tồn tại khi broker restart)
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Take task from command line arguments
	body := bodyFrom(os.Args)

	// Publish a message to the queue
	err = ch.Publish(
		"",     // default exchange
		q.Name, // routing key (queue name)
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent, // make message persistent
			ContentType:  "text/plain",
			Body:         []byte(body),
		})
	failOnError(err, "Failed to publish a message")

	fmt.Printf(" [x] Sent %s\n", body)
}

func bodyFrom(args []string) string {
	var s string
	if len(args) < 2 || os.Args[1] == "" {
		s = "Hello World!"
	} else {
		s = strings.Join(args[1:], " ")
	}
	return s
}
