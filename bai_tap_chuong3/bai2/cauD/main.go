package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Banker struct {
	mu         sync.Mutex
	Available  []int   // Size m:  How many instances of each resource type are currently free
	Max        [][]int // Size n * m: Maximun number of each Resource type each Process my request
	Allocation [][]int // Size n * m: Number of each Resource type currently allocated to each process
	Need       [][]int // Size n * m: The remining resources each process still requires to compllete (Need = Max - Allocation)
	// m: Number of Resource types
	// n: Numebr of Processes
}

// ===== Banker's Helpers =====
func NewBanker(available []int, max [][]int) *Banker {
	numProcs := len(max)
	numResType := len(available)

	allocation := make([][]int, numProcs)

	for i := range numProcs {
		allocation[i] = make([]int, numResType)
	}

	need := calculateNeed(max, allocation)

	return &Banker{
		Available:  available,
		Max:        max,
		Allocation: allocation,
		Need:       need,
	}
}

func calculateNeed(max, allocation [][]int) [][]int {
	need := make([][]int, len(max))
	for i := range max {
		need[i] = make([]int, len(max[i]))
		for j := range max[i] {
			need[i][j] = max[i][j] - allocation[i][j]
		}
	}
	return need
}

// ==== Banker's Safety Algorithm ===
func (b *Banker) IsSafe() (isSafe bool, seq []int) {
	m := len(b.Available)
	n := len(b.Max)

	// initialize
	work := make([]int, m) // resources currently available
	copy(work, b.Available)
	finish := make([]bool, n) // default all false

	sequence := make([]int, 0, n)

	// Step 2: Look for a process Pi such that:
	// Finish[i] = false
	// Need[i] <= Work (means that resources required can be satisfied)
	for {
		found := false
		for i := 0; i < n; i++ {
			if !finish[i] {
				can := true
				for j := 0; j < m; j++ {
					if b.Need[i][j] > work[j] {
						can = false
						break
					}
				}

				if can == true {
					for j := 0; j < m; j++ {
						work[j] += b.Allocation[i][j]
					}
					found = true
					finish[i] = true
					sequence = append(sequence, i)
				}
			}
		}

		if !found {
			break
		}
	}

	for i := 0; i < n; i++ {
		if !finish[i] {
			return false, nil
		}
	}

	return true, sequence
}

// ==== Banker's Resource Request Algorithm ===
func (b *Banker) Request(proc int, requests []int) (isSafe bool, seq []int) {
	// lock
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if the request exceeds the process’s maximum need: If Request[i] ≤ Need[i], continue; otherwise, error
	// Check if resources are available: If Request[i] ≤ Available, continue; otherwise, the process waits.
	for j := 0; j < len(requests); j++ {
		if requests[j] > b.Need[proc][j] || requests[j] > b.Available[j] {
			return false, nil
		}
	}

	// Temporarily allocate the resources:
	for j := 0; j < len(requests); j++ {
		b.Available[j] -= requests[j]
		b.Allocation[proc][j] += requests[j]
		b.Need[proc][j] -= requests[j]
	}

	// Run the Safety Algorithm
	if safe, seq := b.IsSafe(); safe {
		return true, seq
	}

	// Not safe, roll back the resources:
	for j := 0; j < len(requests); j++ {
		b.Available[j] += requests[j]
		b.Allocation[proc][j] -= requests[j]
		b.Need[proc][j] += requests[j]
	}
	return false, nil
}

func (b *Banker) hasRemainingNeed(proc int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, v := range b.Need[proc] {
		if v > 0 {
			return true
		}
	}
	return false
}

func (b *Banker) Release(proc int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for j := 0; j < len(b.Available); j++ {
		b.Available[j] += b.Allocation[proc][j]
		b.Allocation[proc][j] = 0
	}
}

func (b *Banker) GetNeedCopy(proc int) []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := make([]int, len(b.Need[proc]))
	copy(c, b.Need[proc])
	return c
}

// Main Helpers

// no more need, done!
func allDone(need [][]int) bool {
	for _, r := range need {
		for _, v := range r {
			if v > 0 {
				return false
			}
		}
	}
	return true
}

func randomRequest(needRow []int) []int {
	req := make([]int, len(needRow))
	for j := range needRow {
		if needRow[j] > 0 {
			maxTake := needRow[j]
			if maxTake > 2 {
				maxTake = 2
			}
			req[j] = rand.Intn(maxTake + 1)
		}
	}

	return req
}

func allZero(req []int) bool {
	for _, v := range req {
		if v > 0 {
			return false
		}
	}
	return true
}

func worker(p int, b *Banker, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Printf("P%d started\n", p)

	for {
		if !b.hasRemainingNeed(p) {
			b.Release(p)
			fmt.Printf("P%d FINISHED & RELEASED resources\n", p)
			return
		}

		needCopy := b.GetNeedCopy(p)
		req := randomRequest(needCopy)
		if allZero(req) {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		if granted, seq := b.Request(p, req); granted {
			fmt.Printf("P%d GRANTED %v | Safety Sequence: %v\n", p, req, seq)
		} else {
			fmt.Printf("P%d DENIED %v\n", p, req)
		}

		time.Sleep(300 * time.Millisecond)
	}
}

func main() {
	const numProc = 5
	const numRes = 6

	available := []int{20, 15, 18, 12, 14, 16} // 6 resource types

	max := [][]int{
		{6, 4, 5, 3, 2, 4}, // P0
		{3, 3, 2, 4, 5, 1}, // P1
		{5, 2, 6, 1, 3, 3}, // P2
		{4, 5, 3, 2, 4, 2}, // P3
		{2, 3, 4, 5, 1, 6}, // P4
	}

	banker := NewBanker(available, max)

	fmt.Println("=== BANKER'S ALGORITHM (6 resources - CONCURRENT) ===")
	fmt.Printf("Initial Available: %v\n\n", banker.Available)

	var wg sync.WaitGroup
	for i := 0; i < numProc; i++ {
		wg.Add(1)
		go worker(i, banker, &wg)
	}

	wg.Wait()

	fmt.Println("\nAll processes have done - No DEADLOCK!")
	fmt.Println("Final Available:", banker.Available)
}

// ref: https://www.geeksforgeeks.org/operating-systems/bankers-algorithm-in-operating-system-2/
