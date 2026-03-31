# I. Phần lý thuyết

## Bai 1: So sánh các mô hình giao tiếp

| Mô hình                           | Đặc điểm chính                                                                                              | Ưu điểm                                                                                               | Nhược điểm                                                                                                           | Tình huống nên sử dụng                                                                     |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| **Remote Procedure Call (RPC)**   | Gọi thủ tục từ xa như gọi cục bộ (stub/skeleton, parameter marshalling). Thường synchronous.                | Dễ lập trình, che giấu sự phân tán, trừu tượng cao (gần giống gọi hàm local).                         | Synchronous dễ gây deadlock, overhead marshalling lớn, khó xử lý failure, không phù hợp với giao tiếp không đồng bộ. | Microservices cần gọi hàm đồng bộ, tính toán ngắn, truy vấn database.                      |
| **Message Passing**               | Gửi/nhận tin nhắn (send/receive). Có transient (socket) và persistent (message queue). Hỗ trợ asynchronous. | Decoupling mạnh (sender và receiver không cần biết nhau), scalable, hỗ trợ asynchronous & persistent. | Phức tạp hơn (phải tự xử lý ordering, reliability, duplicate), abstraction thấp hơn RPC.                             | Hệ thống cần loose coupling, task queue, logging, xử lý sự kiện dài hạn (RabbitMQ, Kafka). |
| **Stream-Oriented Communication** | Truyền dữ liệu liên tục (continuous data flow) với yêu cầu về timing (thường dùng UDP/TCP cho multimedia).  | Hiệu suất cao cho dữ liệu lớn & real-time, hỗ trợ streaming (Server/Client/Bidirectional).            | Không đảm bảo reliability nếu dùng UDP, rất nhạy cảm với delay, jitter, packet loss, khó xử lý ordering.             | Truyền dữ liệu real-time: video call, sensor stream, audio/video broadcasting.             |

## Baì 1:

**_a. Client gửi yêu cầu tính tổng hai số nguyên đến Server. Server nhận yêu cầu, tính toán kết quả và trả về cho Client. Giao tiếp giữa Client và Server phải được thực hiện thông qua_**
![Alt text](./bai1/cau1_a.png)
**_b. So sánh gRPC với REST API_**

#### Bảng so sánh chi tiết

| Tiêu chí                     | gRPC (dựa trên RPC)                                                                                                           | REST API                                                | Ưu điểm của gRPC so với REST                                |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- | ----------------------------------------------------------- |
| **Mô hình giao tiếp**        | Procedure-oriented (gọi hàm từ xa như gọi cục bộ) – hỗ trợ Unary, Server Streaming, Client Streaming, Bidirectional Streaming | Resource-oriented (CRUD trên resource qua HTTP methods) | Trừu tượng cao hơn, dễ lập trình, hỗ trợ streaming tự nhiên |
| **Giao thức truyền tải**     | HTTP/2 (multiplexing, header compression, server push)                                                                        | Thường HTTP/1.1 (mỗi request một connection)            | Hiệu suất cao hơn, latency thấp, multiplexing mạnh          |
| **Định dạng dữ liệu**        | Protocol Buffers (binary, strongly typed)                                                                                     | JSON (text-based)                                       | Payload nhỏ hơn, marshalling nhanh, giảm overhead mạng      |
| **Hiệu suất & Bandwidth**    | Rất cao (binary + HTTP/2)                                                                                                     | Thấp hơn (JSON + HTTP/1.1)                              | Giảm đáng kể độ trễ và băng thông                           |
| **Streaming**                | Hỗ trợ native (Server/Client/Bidirectional)                                                                                   | Không hỗ trợ native (phải poll hoặc WebSocket)          | Dễ dàng triển khai Stream-Oriented Communication            |
| **Error handling & Timeout** | Built-in status codes, deadlines, cancellation                                                                                | Phải tự implement (HTTP status codes)                   | Xử lý lỗi và timeout tốt hơn                                |
| **Bảo mật**                  | TLS mặc định + authentication tích hợp                                                                                        | Phải cấu hình riêng (HTTPS)                             | Dễ bật TLS và authentication                                |
| **Contract / Schema**        | Protobuf (compile-time checking)                                                                                              | OpenAPI/Swagger (runtime checking)                      | Type-safe mạnh, ít lỗi runtime                              |

#### gRPC có ưu điểm gì so với REST trong hệ thống phân tán?

- **Hiệu suất cao hơn rõ rệt**: Binary serialization + HTTP/2 giúp giảm kích thước dữ liệu và latency → phù hợp với nguyên tắc **scalability** và **dependability** .
- **Hỗ trợ streaming native**: Giải quyết trực tiếp nhu cầu của Stream-Oriented Communication.
- **Strong typing & contract rõ ràng**: Giảm lỗi khi truyền tham số.
- **Built-in features**: Compression, deadline/timeout, cancellation, load balancing → che giấu sự phân tán tốt hơn.
- **Phù hợp internal microservices**: Dùng giữa các service trong hệ thống, không cần browser compatibility.

#### Khi nào nên sử dụng gRPC thay vì REST?

- Khi hệ thống **cần hiệu suất cao và latency thấp** (microservices nội bộ, high-throughput).
- Khi cần **streaming dữ liệu real-time** (sensor data, video, chat, monitoring).
- Khi các service **cùng ngôn ngữ/tech stack** và muốn type-safe mạnh.
- Khi muốn **giảm overhead mạng** (payload nhỏ, multiplexing).
- Khi triển khai trên Kubernetes (dễ load balancing với gRPC).

**Nên dùng REST** khi:

- API công khai (public API) cần hỗ trợ browser/web client.
- Cần compatibility rộng (mobile, third-party).
- Ưu tiên readability và debugging dễ (JSON dễ đọc).
