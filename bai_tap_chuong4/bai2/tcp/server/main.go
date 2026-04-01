package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

var (
	clients   = make(map[net.Conn]string)
	clientsMu sync.Mutex
)

func broadcast(message string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for conn := range clients {
		_, err := conn.Write([]byte(message))
		if err != nil {
			conn.Close()
			delete(clients, conn)
		}
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	clientsMu.Lock()
	clients[conn] = username
	clientsMu.Unlock()

	fmt.Printf("[%s] has joined\n", username)
	broadcast(fmt.Sprintf("[%s] has joined\n", username))

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		msg = strings.TrimSpace(msg)
		fullMsg := fmt.Sprintf("[%s]: %s\n", username, msg)
		fmt.Print(fullMsg)
		broadcast(fullMsg)
	}

	clientsMu.Lock()
	delete(clients, conn)
	clientsMu.Unlock()
	broadcast(fmt.Sprintf("[%s] đã rời chat\n", username))
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleClient(conn)
	}

}
