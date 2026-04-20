package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

const serverAddr = "localhost:9000"

func main() {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Không thể kết nối đến server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Print("Nhập đường dẫn file: ")
	reader := bufio.NewReader(os.Stdin)
	filePath, _ := reader.ReadString('\n')
	filePath = strings.TrimSpace(filePath)

	_, err = conn.Write([]byte(filePath))
	if err != nil {
		fmt.Println("Lỗi gửi dữ liệu:", err)
		os.Exit(1)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Lỗi nhận phản hồi:", err)
		os.Exit(1)
	}

	fmt.Print("Phản hồi từ server: ", string(buf[:n]))
}
