package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	pid := os.Getpid()

	// read from stdin that parent send through pipe
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		fmt.Fprintln(os.Stderr, "[child - %d] don't receive data from parent process, exiting...", pid)
		os.Exit(1)
	}

	parts := strings.Fields(scanner.Text())
	if len(parts) < 2 {
		fmt.Fprintln(os.Stderr, "[child - %d] invalid data format, exiting...", pid)
		os.Exit(1)
	}

	start, _ := strconv.Atoi(parts[0])
	end, _ := strconv.Atoi(parts[1])

	fmt.Printf("[child - %d] received range: [%d, %d]\n", pid, start, end)

	var sum int64

	for i := start; i <= end; i++ {
		sum += int64(i)
	}

	// write to stdout that parent read through pipe
	fmt.Fprintf(os.Stdout, "SUM:%d\n", sum)
	fmt.Fprintf(os.Stderr, "[child PID=%d] written SUM:%d to stdout pipe\n", pid, sum)
}
