package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

func main() {
	parentToChildR, parentToChildW, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	childToParentR, childToParentW, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer parentToChildR.Close()
		defer childToParentW.Close()

		scanner := bufio.NewScanner(parentToChildR)
		for scanner.Scan() {
			msg := scanner.Text()
			if msg == "exit" {
				break
			}
			fmt.Printf("[Child] received message: %s\n", msg)
			res := "Child received: " + msg
			fmt.Printf("[Child] response for messgae %s\n", msg)
			fmt.Fprintln(childToParentW, res)
		}
	}()

	messages := []string{
		"Hello from parent process!",
		"This is the second message from parent",
		"How are the data in database?",
	}

	scanner := bufio.NewScanner(childToParentR)

	for _, msg := range messages {
		fmt.Printf("[Parent] send: %q\n", msg)
		fmt.Fprintln(parentToChildW, msg)
		if scanner.Scan() {
			res := scanner.Text()
			fmt.Printf("[Parent] received response: %s\n", res)
		}

		time.Sleep(200 * time.Millisecond)
	}

	fmt.Fprintln(parentToChildW, "exit")
	parentToChildW.Close()

	fmt.Println("[Parent] has closed the pipe, waiting for child process to finish...")
	wg.Wait()
	fmt.Println("[Parent] child process has finished, exiting...")
	childToParentR.Close()
}
