import socket
import json

HOST = "0.0.0.0"
PORT = 9002


def handle_client(conn, addr):
    print(f"Kết nối từ {addr}")
    try:
        data = b""
        while True:
            chunk = conn.recv(4096)
            if not chunk:
                break
            data += chunk
            if data.endswith(b"\n"):
                break

        payload = json.loads(data.decode().strip())
        code = payload.get("code", "")
        n = payload.get("n")

        print(f"Nhận code: {code!r}, n={n}")

        func = eval(code)
        result = func(n)

        response = json.dumps({"status": "OK", "result": result}) + "\n"
        conn.sendall(response.encode())

    except Exception as e:
        error_resp = json.dumps({"status": "ERROR", "message": str(e)}) + "\n"
        conn.sendall(error_resp.encode())
    finally:
        conn.close()


def main():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        s.bind((HOST, PORT))
        s.listen()
        print(f"Server đang lắng nghe tại {HOST}:{PORT}")

        while True:
            conn, addr = s.accept()
            handle_client(conn, addr)


if __name__ == "__main__":
    main()
