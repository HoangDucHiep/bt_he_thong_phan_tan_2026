package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

// Parent to child
func main() {
	fmt.Println("Parent process send data to child process")

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		fmt.Println("Error creating pipe:", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// child
	go func() {
		defer wg.Done()
		defer readEnd.Close()

		fmt.Println("[Child] is waiting for data from parent process...")

		scanner := bufio.NewScanner(readEnd)
		for scanner.Scan() {
			fmt.Println("[Child] process received:", scanner.Text())
		}

		fmt.Println("[Child] parent process has closed the pipe, exiting...")
	}()

	messages := []string{
		"Hello from parent process!",
		"This is the second message from parent",
		"Bye!!",
	}

	for _, msg := range messages {
		fmt.Printf("[Parent] send: %q\n", msg)
		fmt.Fprintln(writeEnd, msg)
		time.Sleep(200 * time.Millisecond) // Simulate some delay between messages
	}

	writeEnd.Close() // Close the write end to signal the child process
	fmt.Println("[Parent] has closed the pipe, waiting for child process to finish...")
	wg.Wait() // Wait for the child process to finish
	fmt.Println("[Parent] child process has finished, exiting...")
}
