package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type SensorData struct {
	SensorID  int     `json:"sensor_id"`
	Temp      float64 `json:"temperature"`
	Timestamp string  `json:"timestamp"`
}

func main() {
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://broker.hivemq.com:1883").
		SetClientID("sensor-1").
		SetCleanSession(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	defer client.Disconnect(250)

	fmt.Println("Sensor is sending temperature data every second...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		data := SensorData{
			SensorID:  1,
			Temp:      25.0 + rand.Float64()*10, // 25.0 ~ 35.0
			Timestamp: time.Now().Format(time.RFC3339),
		}
		payload, _ := json.Marshal(data)

		token := client.Publish("sensors/temperature", 1, false, payload) // QoS 1
		token.Wait()
		fmt.Printf("Published: %s\n", string(payload))
	}
}
