package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const port = ":9000"

func handleConn(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Lỗi đọc dữ liệu:", err)
		return
	}

	filePath := strings.TrimSpace(string(buf[:n]))

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			conn.Write([]byte("ERROR: File không tồn tại\n"))
		} else {
			conn.Write([]byte(fmt.Sprintf("ERROR: %s\n", err.Error())))
		}
		return
	}

	conn.Write([]byte(fmt.Sprintf("OK: Dung lượng file '%s' là %d bytes\n", filePath, info.Size())))
}

func main() {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Println("Không thể khởi động server:", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Println("Server đang lắng nghe tại", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Lỗi chấp nhận kết nối:", err)
			continue
		}
		go handleConn(conn)
	}
}
