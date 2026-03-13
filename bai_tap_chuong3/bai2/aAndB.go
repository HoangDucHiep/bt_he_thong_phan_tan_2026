package main

import (
	"fmt"
	"sync"
	"time"
)

// 2 resource
var (
	resourceA sync.Mutex
	resourceB sync.Mutex
)

type Result struct {
	Success bool
	Reason  string
}

func processA(wg *sync.WaitGroup, result chan<- Result) {
	defer wg.Done()

	res := Result{Success: false}

	fmt.Println("[A] start, trying to take [resourceA]")
	if !tryLockWithTimeout(&resourceA, 1500*time.Millisecond) {
		res.Reason = "timeout [resourceA]"
		result <- res
		fmt.Println("[A] Timeout when taking [resourceA] -> skip")
		return
	}
	defer resourceA.Unlock()
	fmt.Println("[A] locked [resourceA]")

	time.Sleep(400 * time.Millisecond)

	fmt.Println("[A] start, trying to take [resourceB]")
	if !tryLockWithTimeout(&resourceB, 1200*time.Millisecond) {
		res.Reason = "timeout [resourceB]"
		result <- res
		fmt.Println("[A] Timeout when taking [resourceB] -> skip")
		return
	}
	defer resourceB.Unlock()
	fmt.Println("[A] locked both [resourceA] and [resourceB]")

	// simulate hard work
	time.Sleep(800 * time.Millisecond)

	res.Success = true
	res.Reason = "Done"
	result <- res

	fmt.Println("[A] done, unlock both")
}

func processB(wg *sync.WaitGroup, result chan<- Result) {
	defer wg.Done()

	res := Result{Success: false}

	fmt.Println("[B] start, trying to take [resourceB]")
	if !tryLockWithTimeout(&resourceB, 1500*time.Millisecond) {
		res.Reason = "timeout [resourceB]"
		result <- res
		fmt.Println("[B] Timeout when taking [resourceB] -> skip")
		return
	}
	defer resourceB.Unlock()
	fmt.Println("[B] locked [resourceB]")

	time.Sleep(400 * time.Millisecond)

	fmt.Println("[B] start, trying to take [resourceA]")
	if !tryLockWithTimeout(&resourceA, 1200*time.Millisecond) {
		res.Reason = "timeout [resourceA]"
		result <- res
		fmt.Println("[B] Timeout when taking [resourceA] -> skip")
		return
	}
	defer resourceA.Unlock()
	fmt.Println("[B] locked both [resourceA] and [resourceB]")

	// simulate hard work
	time.Sleep(800 * time.Millisecond)

	res.Success = true
	res.Reason = "Done"
	result <- res

	fmt.Println("[B] done, unlock both")
}

func tryLockWithTimeout(m *sync.Mutex, timeout time.Duration) bool {
	done := make(chan bool, 1) // channel with buff size 1
	// here we have a channel, if we can lock the m, "true" push in the channel, and we can get it at line 32
	// else after timeout at line 35, we still can't get any thing from the ch (which mean we have not locked the m and push smt into "done"), we return false

	go func() {
		m.Lock()
		done <- true
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func main() {
	var wg sync.WaitGroup
	fmt.Println("======= Deadlock + Timeout Demo ==========")

	for i := 1; i <= 2; i++ {
		resultA := make(chan Result, 1)
		resultB := make(chan Result, 1)
		fmt.Printf("\n-- Run %d --", i)
		wg.Add(2)

		go processA(&wg, resultA)
		go processB(&wg, resultB)

		wg.Wait()

		ra := <-resultA
		rb := <-resultB

		if !ra.Success && !rb.Success {
			fmt.Println("Both false, chance of DEADLOCK")
		}

		time.Sleep(300 * time.Millisecond)

	}
}
