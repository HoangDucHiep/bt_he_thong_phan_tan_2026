package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	numOfUsers    = 10
	maxConcurrent = 3
)

func main() {
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrent) // buffered channel acting as a semaphore

	fmt.Printf("The room only has %d seats\n", maxConcurrent)
	fmt.Printf("There are %d students trying to get into the room.\n", numOfUsers)

	for i := 1; i <= numOfUsers; i++ {
		wg.Add(1)
		go student(i, &wg, sem)
	}

	wg.Wait()
	fmt.Println("All students have finished studying.")
}

func student(id int, wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()

	fmt.Println("Student [%2d] is standing in line to get into the room. ", id)

	sem <- struct{}{} // acquire a slot in the semaphore
	fmt.Printf("Student [%2d] has entered the room. Seats left: %d\n", id, maxConcurrent-len(sem))

	// Study in the room for a while (simulate with sleep)
	sleepTime := rand.Intn(1500) + 800 // 800 - 2300ms
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)

	// Leave the room
	<-sem
	fmt.Printf("Student [%2d] has left the room.\n", id)
}
