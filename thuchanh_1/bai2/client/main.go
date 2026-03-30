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
	Status  bool      `json:"status"`
	Data    *Student  `json:"data,omitempty"`
	Results []Student `json:"results,omitempty"`
	Message string    `json:"message,omitempty"`
}

const (
	HOST = "127.0.0.1"
	PORT = "9001"
)

// sendRequest opens a TCP connection, sends the payload, and returns the parsed response.
func sendRequest(payload string) (*Response, error) {
	conn, err := net.Dial("tcp", HOST+":"+PORT)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to server: %w", err)
	}
	defer conn.Close()

	conn.Write([]byte(payload))

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

func printResponse(resp *Response) {
	if resp.Status {
		if resp.Data != nil {
			sv := resp.Data
			fmt.Println("\nStudent found:")
			fmt.Printf("   ID   : %s\n", sv.ID)
			fmt.Printf("   Name : %s\n", sv.Name)
			fmt.Printf("   GPA  : %.1f\n", sv.GPA)
		} else if len(resp.Results) > 0 {
			fmt.Printf("\n%d student(s) matched:\n", len(resp.Results))
			fmt.Printf("   %-8s %-20s %s\n", "ID", "Name", "GPA")
			fmt.Println("   " + strings.Repeat("-", 36))
			for _, sv := range resp.Results {
				fmt.Printf("   %-8s %-20s %.1f\n", sv.ID, sv.Name, sv.GPA)
			}
		} else {
			fmt.Println("\nNo data found.")
		}
	} else {
		fmt.Printf("\n%s\n", resp.Message)
	}
}

func printMenu() {
	fmt.Println("\n")
	fmt.Println("1. Find student by ID                  ")
	fmt.Println("2. Filter students (Code Migration)    ")
	fmt.Println("q. Quit                                ")
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		printMenu()
		fmt.Print("Choose an option: ")
		scanner.Scan()
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			fmt.Print("Enter student ID: ")
			scanner.Scan()
			id := strings.TrimSpace(scanner.Text())
			if id == "" {
				continue
			}
			payload := "FIND_BY_ID:" + id
			fmt.Printf("\n→ Sending: '%s'\n", payload)
			resp, err := sendRequest(payload)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			printResponse(resp)

		case "2":
			fmt.Println("\nFilter expression examples:")
			fmt.Println("  gpa>3.5   gpa<=3.0   gpa=2.9   gpa>=3.5")
			fmt.Print("Enter filter expression: ")
			scanner.Scan()
			expr := strings.TrimSpace(scanner.Text())
			if expr == "" {
				continue
			}
			payload := "FILTER:" + expr
			fmt.Printf("\n→ Sending filter logic to server: '%s'\n", payload)
			resp, err := sendRequest(payload)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			printResponse(resp)

		case "q", "Q":
			fmt.Println("Goodbye!")
			os.Exit(0)

		default:
			fmt.Println("Invalid choice. Please enter 1, 2, or q.")
		}
	}
}
