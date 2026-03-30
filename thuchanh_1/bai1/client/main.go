package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
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

const (
	HOST = "127.0.0.1"
	PORT = "9000"
)

func searchStudent(id string) {
	conn, err := net.Dial("tcp", HOST+":"+PORT)
	if err != nil {
		fmt.Println("Cannot connect to server. Make sure the server is running.")
		return
	}
	defer conn.Close()

	conn.Write([]byte(id))

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	var resp Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		fmt.Println("Failed to parse response:", err)
		return
	}

	if resp.Status {
		sv := resp.Data
		fmt.Println("\nStudent found:")
		fmt.Printf("   ID   : %s\n", sv.ID)
		fmt.Printf("   Name : %s\n", sv.Name)
		fmt.Printf("   GPA  : %.1f\n", sv.GPA)
	} else {
		fmt.Printf("\n%s\n", resp.Message)
	}
}

func main() {

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nEnter student ID (or 'q' to quit): ")
		scanner.Scan()
		id := strings.TrimSpace(scanner.Text())

		if id == "q" || id == "Q" {
			fmt.Println("Goodbye!")
			break
		}
		if id == "" {
			continue
		}
		searchStudent(id)
	}
}
