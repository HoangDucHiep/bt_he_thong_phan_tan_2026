package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Banker struct {
	Available  []int   // Size m:  How many instances of each resource type are currently free
	Max        [][]int // Size n * m: Maximun number of each Resource type each Process my request
	Allocation [][]int // Size n * m: Number of each Resource type currently allocated to each process
	Need       [][]int // Size n * m: The remining resources each process still requires to compllete (Need = Max - Allocation)
	// m: Number of Resource types
	// n: Numebr of Processes
}

// ==== Banker's Safety Algorithm ===
func (b *Banker) IsSafe() bool {
	m := len(b.Available)
	n := len(b.Max)

	// initialize
	work := make([]int, m) // resources currently available
	copy(work, b.Available)

	finish := make([]bool, n) // default all false

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
					found = true
					for j := 0; j < m; j++ {
						work[j] += b.Need[i][j]
					}
					finish[i] = true
				}
			}
		}

		if !found {
			break
		}
	}

	for i := 0; i < n; i++ {
		if !finish[i] {
			return false
		}
	}

	return true
}

// ==== Banker's Resource Request Algorithm ===
func (b *Banker) Request(proc int, requests []int) bool {
	// Check if the request exceeds the process’s maximum need: If Request[i] ≤ Need[i], continue; otherwise, error
	// Check if resources are available: If Request[i] ≤ Available, continue; otherwise, the process waits.
	for j := 0; j < len(requests); j++ {
		if requests[j] > b.Need[proc][j] || requests[j] > b.Available[j] {
			return false
		}
	}

	// Temporarily allocate the resources:
	for j := 0; j < len(requests); j++ {
		b.Available[j] -= requests[j]
		b.Allocation[proc][j] += requests[j]
		b.Need[proc][j] -= requests[j]
	}

	// Run the Safety Algorithm
	if b.IsSafe() {
		return true
	}

	// Not safe, roll back the resources:
	for j := 0; j < len(requests); j++ {
		b.Available[j] += requests[j]
		b.Allocation[proc][j] -= requests[j]
		b.Need[proc][j] += requests[j]
	}
	return false
}

func NewBanker(available []int, max [][]int) *Banker {
	numProcs := len(max)
	numResType := len(max)

	allocation := make([][]int, numProcs)

	for i := 0; i < numProcs; i++ {
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

// ===== Banker's Helpers =====
func calculateNeed(max, allocation [][]int) [][]int {
	need := make([][]int, len(max))

	for i := 0; i < len(max); i++ {
		need[i] = make([]int, len(max[i]))
		for j := 0; j < len(max[i]); j++ {
			need[i][j] = max[i][j] - allocation[i][j]
		}
	}

	return need
}

func (b *Banker) hasRemainingNeed(proc int) bool {
	for _, v := range b.Need[proc] {
		if v > 0 {
			return true
		}
	}
	return false
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

func main() {
	const numProc = 5
	const numRes = 3

	available := []int{12, 8, 7}

	max := [][]int{
		{7, 5, 3}, // P0
		{3, 2, 2}, // P1
		{9, 0, 2}, // P2
		{2, 2, 2}, // P3
		{4, 3, 3}, // P4
	}

	banker := NewBanker(available, max)

	//fmt.Println(banker.IsSafe()) // old commit

	fmt.Println("=== BANKER'S ALGORITHM DEMO - Deadlock Avoidance ===")
	fmt.Printf("Initial Available: %v\n\n", banker.Available)

	for attempt := 0; attempt < 100; attempt++ {
		if allDone(banker.Need) {
			fmt.Println("\n === All process finish - No DEADLOCK ===")
		}

		// choose a random process that still need res
		p := rand.Intn(numProc)
		if !banker.hasRemainingNeed(p) { // p dont have any need remaining
			continue
		}

		req := randomRequest(banker.Need[p])
		if allZero(req) {
			continue
		}

		if banker.Request(p, req) {
			fmt.Printf("-- GRANTED (safe state): New Available: %v\n", banker.Available)
		} else {
			fmt.Printf("-- DENIED (unsafe state): DEADLOCK blocked\n")
		}

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Println("\n===== Final State =====")
	fmt.Printf("Available: %v\n", banker.Available)
	for i := 0; i < numProc; i++ {
		fmt.Printf("P%d Allocation: %v | Need: %v\n", i, banker.Allocation[i], banker.Need[i])
	}
}

// ref: https://www.geeksforgeeks.org/operating-systems/bankers-algorithm-in-operating-system-2/
