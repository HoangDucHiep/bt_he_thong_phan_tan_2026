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

	// Declare the same direct exchange
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

	// IMPORTANT: Bind to MULTIPLE routing keys
	// This subscriber will receive messages with any of these severity levels
	severities := []string{"info", "warning", "error"}
	for _, s := range severities {
		err = ch.QueueBind(
			q.Name,        // queue name
			s,             // routing key
			"logs_direct", // exchange
			false,
			nil,
		)
		failOnError(err, "Failed to bind a queue")
		fmt.Printf(" [*] Binding to routing key: %s\n", s)
	}

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
			fmt.Printf(" [ALL LOGS] [%s] %s\n", d.RoutingKey, d.Body)
		}
	}()

	fmt.Printf(" [*] Waiting for ALL logs (info, warning, error). To exit press CTRL+C\n")
	<-forever
}
