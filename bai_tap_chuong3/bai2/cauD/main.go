package main

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

func main() {

}

// ref: https://www.geeksforgeeks.org/operating-systems/bankers-algorithm-in-operating-system-2/
