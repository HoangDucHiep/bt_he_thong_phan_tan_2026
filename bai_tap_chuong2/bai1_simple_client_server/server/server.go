package main

import (
	"bufio"
	"fmt"
	"net"
)

const PORT = ":8080"

// using TCP socket
func main() {
	listener, err := net.Listen("tcp", PORT)

	if err != nil {
		fmt.Errorf("Failed to listen on port %s: %v", PORT, err)
		panic(err)
	}

	// close the listener eventually
	defer listener.Close()
	fmt.Printf("Server is listening on port %s...\n", PORT)

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	addr := conn.RemoteAddr().String()
	fmt.Printf("Connected with client: %s\n", addr)

	reader := bufio.NewReader(conn)

	msg, err := reader.ReadString('\n')

	if err != nil {
		fmt.Printf("Failed to read from client %s: %v", addr, err)
		return
	}

	fmt.Printf("Received message from client %s: %s", addr, msg)

	response := fmt.Sprintf("Server received your message: %s", msg)
	_, err = conn.Write([]byte(response))

	if err != nil {
		fmt.Printf("Failed to send response to client %s: %v", addr, err)
		return
	}

	fmt.Printf("Sent back response to client %s\n", addr)
}
