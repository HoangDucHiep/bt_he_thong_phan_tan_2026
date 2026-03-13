package main

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// ===== SEMAPHORE =====
type Semaphore struct {
	ch chan struct{}
}

func NewSemaphore(cap int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, cap),
	}
}

func (s *Semaphore) Acquire() {
	s.ch <- struct{}{} // wait if channel is full
}

func (s *Semaphore) Release() {
	<-s.ch // wait if channel is empty
}

// ===== RESOURCE ======
type Resource struct {
	sem   *Semaphore
	Name  string
	Index int // lock order
}

var resources = []*Resource{
	{sem: NewSemaphore(1), Name: "R1 - File DB", Index: 1},
	{sem: NewSemaphore(2), Name: "R2 - Network Socket", Index: 2},
	{sem: NewSemaphore(1), Name: "R3 - Printer", Index: 3},
	{sem: NewSemaphore(1), Name: "R4 - GPU", Index: 4},
	{sem: NewSemaphore(3), Name: "R5 - Shared Memory", Index: 5},
	{sem: NewSemaphore(1), Name: "R6 - Config Lock", Index: 6},
}

const (
	numProcesses  = 8
	needResources = 3
)

// ===== WORKER =====
func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Printf("[P-%d] started\n", id)

	// select 3 random resources
	perm := rand.Perm(len(resources))
	selected := make([]*Resource, needResources)
	for i := 0; i < needResources; i++ {
		selected[i] = resources[perm[i]]
	}

	// sort by lock order
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Index < selected[j].Index
	})

	// acquire resources in order
	locked := make([]*Resource, 0, needResources)
	for _, res := range selected {
		fmt.Printf("[P-%d] trying to acquire %s\n", id, res.Name)
		res.sem.Acquire()
		fmt.Printf("[P-%d] acquired %s\n", id, res.Name)
		locked = append(locked, res)
		time.Sleep(time.Duration(rand.Intn(250)+100) * time.Millisecond)
	}

	// doing jobs
	time.Sleep(time.Duration(rand.Intn(500)+200) * time.Millisecond)

	// release in reverse order
	for i := len(locked) - 1; i >= 0; i-- {
		res := locked[i]
		res.sem.Release()
		fmt.Printf("[P-%d] released %s\n", id, res.Name)
	}

	fmt.Printf("[P-%d] finished\n", id)
}

func main() {
	for run := 1; run <= 3; run++ {
		fmt.Printf("\n=== RUN %d / 3 ===\n\n", run)

		var wg sync.WaitGroup
		wg.Add(numProcesses)

		for i := 1; i <= numProcesses; i++ {
			go worker(i, &wg)
		}

		wg.Wait()
		fmt.Printf("Run %d done\n", run)
		time.Sleep(400 * time.Millisecond)
	}
}
