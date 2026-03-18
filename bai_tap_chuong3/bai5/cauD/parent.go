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

			// Pipe 1: Parent to Child (send range)
			stdinR, stdinW, err := os.Pipe()
			if err != nil {
				fmt.Printf("[parent] Error creating pipe for child %d: %v\n", idx, err)
				return
			}

			// Pipe 2: Child to Parent (receive sum)
			stdoutR, stdoutW, err := os.Pipe()
			if err != nil {
				fmt.Printf("[parent] Error creating pipe for child %d: %v\n", idx, err)
				return
			}

			// spawn ./child
			cmd := exec.Command("./child")
			cmd.Stdin = stdinR   // asign the read end of pipe 1 to child stdin
			cmd.Stdout = stdoutW // asign the write end of pipe 2 to child stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Start(); err != nil {
				fmt.Printf("[parent] Error starting child %d: %v\n", idx, err)
				return
			}

			// close unused ends in parent
			stdinR.Close()
			stdoutW.Close() // if not close, child process will block when write to stdout pipe because parent still hold the write end

			fmt.Printf("[parent] Spawn child %d with PID=%d for range [%d, %d]\n", idx, cmd.Process.Pid, start, end)

			// send task to the child
			fmt.Fprintf(stdinW, "%d %d\n", start, end)
			stdinW.Close() // close the write end to signal the child process to exit the Scan

			// read result from the child
			scanner := bufio.NewScanner(stdoutR)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "SUM:") {
					val, _ := strconv.ParseInt(strings.TrimPrefix(line, "SUM:"), 10, 64)
					partialSums[idx] = val
					fmt.Printf("[parent] Received partial sum from child %d: %d\n", idx, val)
				}
			}
			stdoutR.Close() // close the read end after done reading

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
