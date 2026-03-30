package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	HOST          = "127.0.0.1"
	PORT          = "9002"
	SYNC_INTERVAL = 5 * time.Second
	SYNC_ROUNDS   = 3
)

type clientClock struct {
	skew time.Duration
}

func (c *clientClock) Now() time.Time {
	return time.Now().Add(c.skew)
}

func syncWithServer(clock *clientClock) (corrected time.Time, delay, offset time.Duration, T1, T2, T3, T4 int64, err error) {
	conn, err := net.DialTimeout("tcp", HOST+":"+PORT, 3*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	scanner := bufio.NewScanner(conn)

	T1 = clock.Now().UnixNano()
	writer.WriteString("GET_TIME\n")
	writer.Flush()

	if !scanner.Scan() {
		err = fmt.Errorf("no response from server")
		return
	}
	T4 = clock.Now().UnixNano()

	line := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(line, "TIME:") {
		err = fmt.Errorf("unexpected response: %s", line)
		return
	}

	parts := strings.Split(strings.TrimPrefix(line, "TIME:"), ":")
	if len(parts) != 2 {
		err = fmt.Errorf("malformed TIME response: %s", line)
		return
	}
	T2, e1 := strconv.ParseInt(parts[0], 10, 64)
	T3, e2 := strconv.ParseInt(parts[1], 10, 64)
	if e1 != nil || e2 != nil {
		err = fmt.Errorf("invalid timestamps in response")
		return
	}

	delta := (T4 - T1) - (T3 - T2)
	theta := ((T2 - T1) + (T3 - T4)) / 2

	delay = time.Duration(delta)
	offset = time.Duration(theta)
	corrected = clock.Now().Add(offset)
	return
}

func main() {
	name := flag.String("name", "Client", "Client name")
	skewMs := flag.Int64("skew", 0, "Clock skew in milliseconds")
	flag.Parse()

	clock := &clientClock{skew: time.Duration(*skewMs) * time.Millisecond}

	fmt.Printf("CLIENT: %s  skew: %+d ms  server: %s:%s\n\n", *name, *skewMs, HOST, PORT)

	for round := 1; ; round++ {
		fmt.Printf("--- Sync Round #%d (%s) ---\n", round, *name)
		fmt.Printf("  Before : %s  (skew %+.0f ms)\n",
			clock.Now().Format("15:04:05.000000000"), float64(*skewMs))

		corrected, delay, offset, T1, T2, T3, T4, err := syncWithServer(clock)
		if err != nil {
			fmt.Printf("  Sync failed: %v\n\n", err)
		} else {
			clock.skew = corrected.Sub(time.Now())

			fmt.Printf("  T1 (client sent)     : %s\n", time.Unix(0, T1).Format("15:04:05.000000000"))
			fmt.Printf("  T2 (server received) : %s\n", time.Unix(0, T2).Format("15:04:05.000000000"))
			fmt.Printf("  T3 (server sent)     : %s\n", time.Unix(0, T3).Format("15:04:05.000000000"))
			fmt.Printf("  T4 (client received) : %s\n", time.Unix(0, T4).Format("15:04:05.000000000"))
			fmt.Printf("  T4-T1 (total RTT)    : %.3f ms\n", float64((T4-T1)/1e3)/1000.0)
			fmt.Printf("  T3-T2 (server proc)  : %.3f ms\n", float64((T3-T2)/1e3)/1000.0)
			fmt.Printf("  δ = (T4-T1)-(T3-T2) : %.3f ms\n", float64(delay.Microseconds())/1000.0)
			fmt.Printf("  θ = ((T2-T1)+(T3-T4))/2 : %+.3f ms\n", float64(offset.Microseconds())/1000.0)
			fmt.Printf("  After  : %s  (new skew %+.3f ms)\n",
				clock.Now().Format("15:04:05.000000000"),
				float64(clock.skew.Microseconds())/1000.0)
		}
		fmt.Println()

		if SYNC_ROUNDS > 0 && round >= SYNC_ROUNDS {
			fmt.Printf("[%s] Done %d rounds.\n", *name, SYNC_ROUNDS)
			os.Exit(0)
		}
		time.Sleep(SYNC_INTERVAL)
	}
}
