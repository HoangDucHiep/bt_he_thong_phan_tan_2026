package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
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

var (
	students = map[string]Student{
		"SV001": {ID: "SV001", Name: "Nguyen Van An", GPA: 3.8},
		"SV002": {ID: "SV002", Name: "Tran Thi Binh", GPA: 2.9},
		"SV003": {ID: "SV003", Name: "Le Minh Cuong", GPA: 3.5},
		"SV004": {ID: "SV004", Name: "Pham Thi Dung", GPA: 3.1},
		"SV005": {ID: "SV005", Name: "Hoang Van Em", GPA: 2.5},
	}
	mu sync.RWMutex // protects concurrent reads on the student map
)

const (
	HOST = "127.0.0.1"
	PORT = "9001"
)

// handleFindByID looks up a single student.
func handleFindByID(id string) Response {
	mu.RLock()
	defer mu.RUnlock()

	if sv, ok := students[id]; ok {
		return Response{Status: true, Data: &sv}
	}
	return Response{Status: false, Message: fmt.Sprintf("Student '%s' not found", id)}
}

// <field><op><value>  gpa>3.5  gpa<=3.0
func handleFilter(expr string) Response {
	expr = strings.TrimSpace(expr)

	// Parse operator (order matters: check 2-char ops before 1-char ops)
	operators := []string{">=", "<=", ">", "<", "="}
	var op, field, valueStr string
	for _, candidate := range operators {
		if idx := strings.Index(expr, candidate); idx != -1 {
			field = strings.TrimSpace(expr[:idx])
			op = candidate
			valueStr = strings.TrimSpace(expr[idx+len(candidate):])
			break
		}
	}

	if op == "" {
		return Response{Status: false, Message: fmt.Sprintf("Invalid filter expression: '%s'", expr)}
	}

	threshold, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return Response{Status: false, Message: fmt.Sprintf("Invalid numeric value: '%s'", valueStr)}
	}

	// Only GPA field supported; easy to extend
	if field != "gpa" {
		return Response{Status: false, Message: fmt.Sprintf("Unknown field: '%s'. Supported: gpa", field)}
	}

	mu.RLock()
	defer mu.RUnlock()

	var matches []Student
	for _, sv := range students {
		var match bool
		switch op {
		case ">":
			match = sv.GPA > threshold
		case "<":
			match = sv.GPA < threshold
		case ">=":
			match = sv.GPA >= threshold
		case "<=":
			match = sv.GPA <= threshold
		case "=":
			match = sv.GPA == threshold
		}
		if match {
			matches = append(matches, sv)
		}
	}

	if len(matches) == 0 {
		return Response{Status: false, Message: fmt.Sprintf("No students match filter: %s", expr)}
	}
	return Response{Status: true, Results: matches}
}

// dispatch parses a raw request string and routes to the correct handler.
func dispatch(raw string) []byte {
	raw = strings.TrimSpace(raw)
	var resp Response

	switch {
	case strings.HasPrefix(raw, "FIND_BY_ID:"):
		id := strings.TrimPrefix(raw, "FIND_BY_ID:")
		resp = handleFindByID(strings.TrimSpace(id))

	case strings.HasPrefix(raw, "FILTER:"):
		expr := strings.TrimPrefix(raw, "FILTER:")
		resp = handleFilter(strings.TrimSpace(expr))

	default:
		resp = Response{
			Status:  false,
			Message: "Unknown command. Use FIND_BY_ID:<id> or FILTER:<expr>",
		}
	}

	data, _ := json.Marshal(resp)
	return data
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr()
	fmt.Printf("New connection from %s\n", addr)

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("Read error from %s: %v\n", addr, err)
		return
	}

	raw := string(buf[:n])
	fmt.Printf("Request from %s: '%s'\n", addr, raw)

	resp := dispatch(raw)
	conn.Write(resp)
	fmt.Printf("Response sent to %s.\n\n", addr)
}

func main() {
	ln, err := net.Listen("tcp", HOST+":"+PORT)
	if err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Printf("Multithreaded server listening on %s:%s\n", HOST, PORT)
	fmt.Println("Supported commands:")
	fmt.Println("  FIND_BY_ID:<id>    — find student by ID")
	fmt.Println("  FILTER:<expr>      — filter by field, e.g. FILTER:gpa>3.5")
	fmt.Println()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		go handleConnection(conn)
	}
}
