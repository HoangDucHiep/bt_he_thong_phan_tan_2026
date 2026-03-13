package main

import "fmt"

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

func NewBanker(available []int, max [][]int, allocation [][]int) *Banker {
	/* numProcs := len(max)
	numResType := len(max) */
	needs := calculateNeed(max, allocation)

	return &Banker{
		Available:  available,
		Max:        max,
		Allocation: allocation,
		Need:       needs,
	}
}

// ===== Helpers =====
func calculateNeed(max, allocation [][]int) [][]int {
	needs := make([][]int, len(max))

	for i := 0; i < len(max); i++ {
		needs[i] = make([]int, len(max[i]))
		for j := 0; j < len(max[i]); j++ {
			needs[i][j] = max[i][j] - allocation[i][j]
		}
	}

	return needs
}

func main() {
	available := []int{3, 3, 2}

	max := [][]int{
		{7, 5, 3}, // P0
		{3, 2, 2}, // P1
		{9, 0, 2}, // P2
		{2, 2, 2}, // P3
		{4, 3, 3}, // P4
	}

	allocation := [][]int{
		{0, 1, 0},
		{2, 0, 0},
		{3, 0, 2},
		{2, 1, 1},
		{0, 0, 2},
	}

	banker := NewBanker(available, max, allocation)

	fmt.Println(banker.IsSafe())

}

// ref: https://www.geeksforgeeks.org/operating-systems/bankers-algorithm-in-operating-system-2/
