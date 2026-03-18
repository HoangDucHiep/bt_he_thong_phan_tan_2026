package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	numWriters = 8
	filename   = "output.log"
)

func main() {
	var wg sync.WaitGroup

	fileSem := make(chan struct{}, 1) // binary semaphore for file access

	fmt.Printf("%d writers are trying to write to the file '%s'.\n", numWriters, filename)
	fmt.Println("Only 1 writer can write to the file at a time.")

	for i := 1; i <= numWriters; i++ {
		wg.Add(1)
		go writer(i, &wg, fileSem)
	}
	wg.Wait()

	fmt.Printf("All writers have finished writing to the file '%s'.\n", filename)
}

func writer(id int, wg *sync.WaitGroup, fileSem chan struct{}) {
	defer wg.Done()

	// Simulate the time taken to prepare the data to write
	time.Sleep(time.Duration(rand.Intn(400)+100) * time.Millisecond)
	fmt.Printf("Writer [%2d] is waiting to write to the file.\n", id)

	fileSem <- struct{}{}
	fmt.Printf("Writer [%2d] has started writing to the file.\n", id)

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Writer [%2d] encountered an error while opening the file: %v\n", id, err)
		<-fileSem // Release the semaphore
		return
	}
	defer f.Close()

	// Write data
	msg := fmt.Sprintf("Writer [%2d] wrote to the file at %s\n", id, time.Now().Format(time.RFC3339))
	if _, err := f.WriteString(msg); err != nil {
		fmt.Printf("Writer [%2d] encountered an error while writing to the file: %v\n", id, err)
	}

	fmt.Printf("Writer [%2d] has finished writing to the file.\n", id)
	<-fileSem // Release the semaphore
}
