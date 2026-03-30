package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type Student struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	GPA  float64 `json:"gpa"`
}

type Response struct {
	Status  bool     `json:"status"`
	Data    *Student `json:"data,omitempty"`
	Message string   `json:"message,omitempty"`
}

var students = map[string]Student{
	"SV001": {ID: "SV001", Name: "Nguyễn Văn An", GPA: 3.8},
	"SV002": {ID: "SV002", Name: "Trần Thị Bình", GPA: 2.9},
	"SV003": {ID: "SV003", Name: "Lê Minh Cường", GPA: 3.5},
	"SV004": {ID: "SV004", Name: "Phạm Thị Dung", GPA: 3.1},
	"SV005": {ID: "SV005", Name: "Hoàng Văn Em", GPA: 2.5},
}

const (
	HOST = "127.0.0.1"
	PORT = "9000"
)

func handleRequest(id string) []byte {
	var resp Response
	if sv, ok := students[id]; ok {
		resp = Response{Status: true, Data: &sv}
	} else {
		resp = Response{Status: false, Message: fmt.Sprintf("Student with ID '%s' not found", id)}
	}
	data, _ := json.Marshal(resp)
	return data
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("Connection from %s\n", conn.RemoteAddr())

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}

	id := string(buf[:n])
	fmt.Printf("Received request: '%s'\n", id)

	resp := handleRequest(id)
	conn.Write(resp)
	fmt.Printf("Response sent.\n\n")
}

func main() {
	ln, err := net.Listen("tcp", HOST+":"+PORT)
	if err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
	defer ln.Close()
	fmt.Printf("Listening on %s:%s ...\n", HOST, PORT)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		handleConnection(conn)
	}
}
