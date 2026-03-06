package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

const SERVER_ADDR = "localhost:8080"

func main() {
	msg := "Hello, Server!\n"

	// command arg
	if len(os.Args) > 1 {
		msg = os.Args[1] + "\n"
	}

	conn, err := net.Dial("tcp", SERVER_ADDR)
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg + "\n"))
	if err != nil {
		fmt.Printf("Write error: %v\n", err)
		return
	}

	// Đọc phản hồi
	reader := bufio.NewReader(conn)
	resp, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return
	}
	fmt.Printf("Server response: %s", resp)
}
