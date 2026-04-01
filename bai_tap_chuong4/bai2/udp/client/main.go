package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", "localhost:8081")
	conn, _ := net.DialUDP("udp", nil, serverAddr)
	defer conn.Close()

	fmt.Print("Enter username: ")
	username, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	username = strings.TrimSpace(username)

	// Send username to server
	conn.Write([]byte(username))

	var wg sync.WaitGroup
	wg.Add(2)

	// Receive messages
	go func() {
		defer wg.Done()
		buffer := make([]byte, 1024)
		for {
			n, _, _ := conn.ReadFromUDP(buffer)
			fmt.Print(string(buffer[:n]))
		}
	}()

	// Send messages
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			conn.Write([]byte(fmt.Sprintf("[%s]: %s", username, text)))
		}
	}()

	wg.Wait()
}
