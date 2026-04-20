package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

const (
	appPort   = ":9001"
	redisAddr = "localhost:6379"
)

var (
	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	ctx = context.Background()
)

func handleConn(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Lỗi đọc dữ liệu:", err)
		return
	}

	// Định dạng nhận: "key=value"
	msg := strings.TrimSpace(string(buf[:n]))
	parts := strings.SplitN(msg, "=", 2)
	if len(parts) != 2 {
		conn.Write([]byte("ERROR: Định dạng không hợp lệ. Dùng: key=value\n"))
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Lưu vào Redis
	err = rdb.Set(ctx, key, value, 0).Err()
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("ERROR: Không thể lưu vào Redis: %s\n", err.Error())))
		return
	}

	// Đọc lại từ Redis để xác nhận
	savedValue, err := rdb.Get(ctx, key).Result()
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("ERROR: Không thể đọc lại từ Redis: %s\n", err.Error())))
		return
	}

	conn.Write([]byte(fmt.Sprintf("OK: Đã lưu thành công vào Redis. key='%s', value='%s'\n", key, savedValue)))
}

func main() {
	// Kiểm tra kết nối Redis
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("Không thể kết nối Redis:", err)
		os.Exit(1)
	}
	fmt.Println("Đã kết nối Redis tại", redisAddr)

	ln, err := net.Listen("tcp", appPort)
	if err != nil {
		fmt.Println("Không thể khởi động App Server:", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Println("App Server đang lắng nghe tại", appPort)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Lỗi chấp nhận kết nối:", err)
			continue
		}
		go handleConn(conn)
	}
}
