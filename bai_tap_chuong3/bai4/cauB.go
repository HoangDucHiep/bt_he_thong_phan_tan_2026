package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	numGoroutines = 100
	increments    = 10000
)

func main() {
	fmt.Println("=== Part b: Using Mutex to avoid Race Condition ===\n")

	fmt.Println("Comparison of two approaches:\n")

	// Case 1: WITHOUT Mutex (race condition)
	fmt.Println("1. Without Mutex:")
	runWithoutMutex()

	// Case 2: WITH Mutex (safe)
	fmt.Println("\n2. With Mutex:")
	runWithMutex()
}

func runWithoutMutex() {
	counter := 0
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				counter++ // ← vẫn race condition
			}
		}()
	}

	wg.Wait()

	duration := time.Since(start)
	expected := numGoroutines * increments
	fmt.Printf("Counter = %d (expected %d) => diff: %d\n", counter, expected, expected-counter)
	fmt.Printf("Duration: %v\n", duration)
}

func runWithMutex() {
	var counter int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	duration := time.Since(start)
	expected := numGoroutines * increments
	fmt.Printf("Counter = %d (expected %d) => diff: %d\n", counter, expected, expected-counter)
	fmt.Printf("Duration: %v\n", duration)
}
