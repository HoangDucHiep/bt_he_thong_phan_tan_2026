package main

import (
	"bytes"
	"fmt"
	"log"
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

	q, err := ch.QueueDeclare(
		"task_queue", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// IMPORTANT: Set QoS with prefetch_count=1 for Fair Dispatch
	// Worker will only receive 1 unacknowledged message at a time
	// This ensures fair distribution based on worker capacity, not round-robin
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	// Consume with auto-ack=false to enable manual acknowledgment
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (MUST be false)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	var forever chan struct{} = make(chan struct{})

	go func() {
		for d := range msgs {
			fmt.Printf("Received a message: %s\n", d.Body)

			// Simulate work: each dot represents 1 second of processing time
			dotCount := bytes.Count(d.Body, []byte("."))
			t := time.Duration(dotCount)
			time.Sleep(t * time.Second)

			fmt.Println("Done")

			// Manual ACK after processing completes
			d.Ack(false)
		}
	}()

	fmt.Printf(" [*] Waiting for messages (Fair Dispatch with prefetch_count=1). To exit press CTRL+C\n")
	<-forever
}
