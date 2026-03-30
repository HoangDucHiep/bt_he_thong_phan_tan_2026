package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	HOST = "127.0.0.1"
	PORT = "9002"
)

func handleClient(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr()
	fmt.Printf("Connection from %s\n", addr)

	scanner := bufio.NewScanner(conn)
	writer := bufio.NewWriter(conn)

	for scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())
		if msg != "GET_TIME" {
			writer.WriteString("ERROR:unknown_command\n")
			writer.Flush()
			continue
		}
		T2 := time.Now().UnixNano() // server receives request

		// There should be a small processing delay here

		T3 := time.Now().UnixNano() // server sends response

		response := fmt.Sprintf("TIME:%d:%d\n", T2, T3)
		writer.WriteString(response)
		writer.Flush()

		fmt.Printf("Sent to %s — T2=%s T3=%s\n",
			addr,
			time.Unix(0, T2).Format("15:04:05.000000000"),
			time.Unix(0, T3).Format("15:04:05.000000000"),
		)
	}
}

func main() {
	ln, err := net.Listen("tcp", HOST+":"+PORT)
	if err != nil {
		fmt.Println("Failed to start Time Server:", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Println("Time Server listening on", HOST+":"+PORT)
	fmt.Printf("Server time: %s\n\n", time.Now().Format("2006-01-02 15:04:05.000000000"))

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		go handleClient(conn)
	}
}
