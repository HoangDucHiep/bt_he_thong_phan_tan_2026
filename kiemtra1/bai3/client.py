import socket
import json

HOST = "localhost"
PORT = 9002


def send_request(code: str, n):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.connect((HOST, PORT))

        payload = json.dumps({"code": code, "n": n}) + "\n"
        s.sendall(payload.encode())

        data = b""
        while True:
            chunk = s.recv(4096)
            if not chunk:
                break
            data += chunk
            if data.endswith(b"\n"):
                break

        response = json.loads(data.decode().strip())
        return response


def main():
    code = input("Nhập đoạn mã lambda (vd: lambda x: x**2): ").strip()
    n_input = input("Nhập tham số n: ").strip()

    # Thử parse n thành số
    try:
        n = int(n_input)
    except ValueError:
        try:
            n = float(n_input)
        except ValueError:
            n = n_input

    print(f"\nGửi đến server: code={code!r}, n={n}")

    response = send_request(code, n)

    if response["status"] == "OK":
        print(f"Kết quả trả về: {response['result']}")
    else:
        print(f"Lỗi từ server: {response['message']}")


if __name__ == "__main__":
    main()
