package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

func main() {
	const total = 1_000_000
	const numChildren = 4
	chunk := total / numChildren

	fmt.Printf("[parent PID=%d] Starting parent process, total=%d, numChildren=%d, chunk=%d\n", os.Getpid(), total, numChildren, chunk)

	partialSums := make([]int64, numChildren)
	var wg sync.WaitGroup

	for i := 0; i < numChildren; i++ {
		wg.Add(1)

		start := i*chunk + 1
		end := (i + 1) * chunk
		if i == numChildren-1 {
			end = total
		}

		go func(idx, start, end int) {
			defer wg.Done()

			// spawn ./child
			cmd := exec.Command("./child")

			// Pipe 1: Parent to Child (send range)
			stdinPipe, err := cmd.StdinPipe()
			if err != nil {
				fmt.Printf("[parent] Error creating pipe for child %d: %v\n", idx, err)
				return
			}

			// Pipe 2: Child to Parent (receive sum)
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				fmt.Printf("[parent] Error creating pipe for child %d: %v\n", idx, err)
				return
			}

			cmd.Stderr = os.Stderr

			if err := cmd.Start(); err != nil {
				fmt.Printf("[parent] Error starting child %d: %v\n", idx, err)
				return
			}

			fmt.Printf("[parent] Spawn child %d with PID=%d for range [%d, %d]\n", idx, cmd.Process.Pid, start, end)

			// send task to the child
			fmt.Fprintf(stdinPipe, "%d %d\n", start, end)
			stdinPipe.Close() // close the write end to signal the child process to exit the Scan

			// read result from the child
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "SUM:") {
					val, _ := strconv.ParseInt(strings.TrimPrefix(line, "SUM:"), 10, 64)
					partialSums[idx] = val
					fmt.Printf("[parent] Received partial sum from child %d: %d\n", idx, val)
				}
			}
			stdoutPipe.Close() // close the read end after done reading

			if err := cmd.Wait(); err != nil {
				fmt.Printf("[parent] Child %d exited with error: %v\n", idx, err)
			} else {
				fmt.Printf("[parent] Child %d exited successfully\n", idx)
			}

		}(i, start, end)

	}

	wg.Wait()

	fmt.Println()
	fmt.Println("===== Result =====")

	var grandTotal int64
	for i, sum := range partialSums {
		fmt.Printf("Partial sum from child %d: %d\n", i, sum)
		grandTotal += sum
	}

	expected := int64(total * (total + 1) / 2)

	fmt.Printf("Grand total: %d\n", grandTotal)
	fmt.Printf("Expected total: %d\n", expected)
}
