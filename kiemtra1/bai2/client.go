package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

const serverAddr = "localhost:9001"

func main() {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Không thể kết nối đến App Server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Nhập key: ")
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)

	fmt.Print("Nhập value: ")
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)

	msg := fmt.Sprintf("%s=%s", key, value)
	_, err = conn.Write([]byte(msg))
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

	fmt.Print("Phản hồi từ App Server: ", string(buf[:n]))
}
