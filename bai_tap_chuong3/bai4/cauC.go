package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========= REENTRANT MUTEX =========
type ReentrantMutex struct {
	sync.Mutex
	owner uint64
	count int
}

func (rm *ReentrantMutex) Lock() {
	id := getGoroutineID()
	if rm.owner == id {
		rm.count++ // reentrant
		return
	}
	rm.Mutex.Lock()
	rm.owner = id
	rm.count = 1
}

func (rm *ReentrantMutex) Unlock() {
	if rm.count > 1 {
		rm.count-- // still owned by the same goroutine
		return
	}
	rm.owner = 0
	rm.count = 0
	rm.Mutex.Unlock()
}

func getGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, _ := strconv.ParseUint(idField, 10, 64)
	return id
}

// =================================================================

var rmu ReentrantMutex

func funcA() {
	rmu.Lock()
	fmt.Println("funcA: Locked (1)")
	funcB() // nested
	rmu.Unlock()
	fmt.Println("funcA: Finish")
}

func funcB() {
	rmu.Lock() // ← reacquire same lock (reentrant)
	fmt.Println("funcB: Locked (reentrant)")
	time.Sleep(100 * time.Millisecond)
	rmu.Unlock()
}

func main() {
	funcA()
}
