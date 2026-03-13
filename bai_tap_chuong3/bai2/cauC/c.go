package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Resource struct {
	sync.Mutex
	Name string
}

var resources = []*Resource{
	{Name: "R1 - File DB"},
	{Name: "R2 - Network Socket"},
	{Name: "R3 - Printer"},
	{Name: "R4 - GPU"},
	{Name: "R5 - Shared Memory"},
	{Name: "R6 - Config Lock"},
}

// 6 Resources
// Each process will try to take "3" random resources
const (
	numProcesses  = 8
	needResources = 3
	timeout       = 1800 * time.Millisecond
)

func tryLockWithTimeout(r *Resource, t time.Duration) bool {
	done := make(chan bool, 1)

	go func() {
		r.Lock()
		done <- true
	}()

	select {
	case <-done:
		return true
	case <-time.After(t):
		return false
	}
}

func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("-- [P-%02d] Start\n", id)

	// take 3 random resources
	perm := rand.Perm(len(resources))
	selected := make([]*Resource, needResources)
	for i := 0; i < needResources; i++ {
		selected[i] = resources[perm[i]]
	}

	locked := make([]*Resource, 0, needResources)

	for _, r := range selected {
		fmt.Printf("--- [P-%02d] try to take %s\n", id, r.Name)

		if !tryLockWithTimeout(r, timeout) {
			fmt.Printf("--- [P-%02d] Timeout when taking %s -> release all %d taken lock and cancel!\n",
				id, r.Name, len(locked))

			for _, l := range locked {
				l.Unlock()
			}
			return
		}

		fmt.Printf("--- [P-%02d] locked %s\n", id, r.Name)
		locked = append(locked, r)

		// randon sleep, simulate job, increase chance of deadlock
		time.Sleep(time.Duration(rand.Intn(300)+150) * time.Millisecond)
	}

	fmt.Printf("--- [P-%02d] take all 3  resources\n", id)
	time.Sleep(900 * time.Millisecond)

	// release all lock
	for _, l := range locked {
		l.Unlock()
	}
	fmt.Printf("--- [P-%02d] done\n", id)
}

func main() {
	for run := 1; run <= 3; run++ {
		fmt.Printf("\n=== Test run %d / 3 ===\n\n", run)

		var wg sync.WaitGroup
		wg.Add(numProcesses)

		for i := 1; i <= numProcesses; i++ {
			go worker(i, &wg)
		}

		wg.Wait()

		fmt.Printf("\nDone run %d\n", run)
		time.Sleep(400 * time.Millisecond)
	}

}
