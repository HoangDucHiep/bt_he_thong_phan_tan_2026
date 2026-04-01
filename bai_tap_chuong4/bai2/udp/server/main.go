package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

var (
	clientsUDP   = make(map[string]net.Addr) // addrStr -> addr
	clientsUDPMu sync.Mutex
)

func main() {
	addr, _ := net.ResolveUDPAddr("udp", ":8081")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()

	fmt.Println("UDP is listening on :8081...")

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, _ := conn.ReadFromUDP(buffer)
		msg := strings.TrimSpace(string(buffer[:n]))

		clientsUDPMu.Lock()
		addrStr := clientAddr.String()
		if _, ok := clientsUDP[addrStr]; !ok {
			clientsUDP[addrStr] = clientAddr
			fmt.Printf("Client mới: %s\n", addrStr)
		}
		clientsUDPMu.Unlock()

		// Broadcast
		clientsUDPMu.Lock()
		for _, addr := range clientsUDP {
			if addr.String() != addrStr { // not send back to sender
				conn.WriteToUDP([]byte(msg+"\n"), addr.(*net.UDPAddr))
			}
		}
		clientsUDPMu.Unlock()
	}
}
