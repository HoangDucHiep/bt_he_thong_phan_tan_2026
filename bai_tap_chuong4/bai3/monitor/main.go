package main

import (
	"encoding/json"
	"fmt"

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
		SetClientID("monitor-1").
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
		fmt.Printf("[Monitor] Nhận được: Sensor %d | Nhiệt độ: %.1f°C | Time: %s\n",
			data.SensorID, data.Temp, data.Timestamp)
	}); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Println("Monitor đang lắng nghe topic sensors/temperature...")
	select {} // giữ chương trình chạy
}
