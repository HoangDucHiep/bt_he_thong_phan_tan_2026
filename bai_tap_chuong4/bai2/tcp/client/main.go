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
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Print("Enter username: ")
	username, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	username = strings.TrimSpace(username)
	conn.Write([]byte(username + "\n"))

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to receive messages from the server
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(conn)
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Disconnected from server")
				return
			}
			fmt.Print(msg)
		}
	}()

	// Goroutine to send messages to the server
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			conn.Write([]byte(text))
		}
	}()

	wg.Wait()
}
