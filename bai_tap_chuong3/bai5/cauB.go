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

		fmt.Println("[Child] is waiting for task from parent process...")
		scanner := bufio.NewScanner(parentToChildR)
		scanner.Scan()
		task := scanner.Text()
		fmt.Printf("[Child] received task: %q\n", task)

		// process task
		time.Sleep(300 * time.Millisecond)
		result := fmt.Sprintf("Result: %s [Done]", task)

		fmt.Fprintln(childToParentW, result)
		fmt.Println("[Child] has sent result back to parent process, exiting...")
	}()

	fmt.Println("[Parent] is sending task to child process...")
	task := "Get me the data from database"
	fmt.Fprintln(parentToChildW, task)
	parentToChildW.Close() // Close the write end to signal the child process

	fmt.Println("[Parent] is waiting for result from child process...")
	scanner := bufio.NewScanner(childToParentR)
	scanner.Scan()
	result := scanner.Text()
	fmt.Printf("[Parent] received result: %q\n", result)

	childToParentR.Close() // Close the read end
	fmt.Println("[Parent] has received result, waiting for child process to finish...")
	wg.Wait() // Wait for the child process to finish
	fmt.Println("[Parent] child process has finished, exiting...")
}
