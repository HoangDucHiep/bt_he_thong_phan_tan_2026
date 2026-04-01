package main

import (
	"encoding/json"
	"flag"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type SensorData struct {
	SensorID  int     `json:"sensor_id"`
	Temp      float64 `json:"temperature"`
	Timestamp string  `json:"timestamp"`
}

func main() {
	id := flag.Int("id", 1, "Monitor ID")
	flag.Parse()

	opts := mqtt.NewClientOptions().
		AddBroker("tcp://broker.hivemq.com:1883").
		SetClientID(fmt.Sprintf("monitor-%d", *id)).
		SetCleanSession(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	defer client.Disconnect(250)

	topic := "sensors/temperature"
	if token := client.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		var data SensorData
		json.Unmarshal(msg.Payload(), &data)
		fmt.Printf("[Monitor %d] Received: Sensor %d | Temperature: %.1f°C | Time: %s\n",
			*id, data.SensorID, data.Temp, data.Timestamp)

		// warn if temperature exceeds threshold
		const threshold = 32.0
		if data.Temp > threshold {
			fmt.Printf("[Monitor %d] WARNING: Sensor %d temperature %.1f°C exceeds threshold!\n",
				*id, data.SensorID, data.Temp)
		}
	}); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Printf("Monitor %d is listening on topic sensors/temperature...\n", *id)
	select {} // giữ chương trình chạy
}
