package main

import (
	"fmt"
	"log"

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
	// Empty name means RabbitMQ will generate a unique name
	// Exclusive = true means queue is deleted when connection closes
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
	// All messages sent to "logs" exchange will be received by this queue
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

	var forever chan struct{} = make(chan struct{})

	go func() {
		for d := range msgs {
			fmt.Printf(" [CONSOLE] %s\n", d.Body)
		}
	}()

	fmt.Printf(" [*] Waiting for logs (Console Output). To exit press CTRL+C\n")
	<-forever
}
