package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	counters  = 100
	increment = 10000
)

func main() {
	fmt.Println("== Race condition - no Mutex ==")

	for run := 1; run <= 3; run++ {
		var wg sync.WaitGroup

		count := 0
		start := time.Now()

		for i := 0; i < counters; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()
				for j := 0; j < increment; j++ {
					count++
				}
			}()
		}
		wg.Wait()
		duration := time.Since(start)
		expected := counters * increment

		fmt.Printf("Run %d: Count = %d (expected %d) => diff = %d, duration = %s\n", run, count, expected, expected-count, duration)

	}
}
