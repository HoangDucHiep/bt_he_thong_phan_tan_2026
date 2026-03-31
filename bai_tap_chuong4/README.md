# I. Phần lý thuyết

## Bai 1: So sánh các mô hình giao tiếp

| Mô hình                           | Đặc điểm chính                                                                                              | Ưu điểm                                                                                               | Nhược điểm                                                                                                           | Tình huống nên sử dụng                                                                     |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| **Remote Procedure Call (RPC)**   | Gọi thủ tục từ xa như gọi cục bộ (stub/skeleton, parameter marshalling). Thường synchronous.                | Dễ lập trình, che giấu sự phân tán, trừu tượng cao (gần giống gọi hàm local).                         | Synchronous dễ gây deadlock, overhead marshalling lớn, khó xử lý failure, không phù hợp với giao tiếp không đồng bộ. | Microservices cần gọi hàm đồng bộ, tính toán ngắn, truy vấn database.                      |
| **Message Passing**               | Gửi/nhận tin nhắn (send/receive). Có transient (socket) và persistent (message queue). Hỗ trợ asynchronous. | Decoupling mạnh (sender và receiver không cần biết nhau), scalable, hỗ trợ asynchronous & persistent. | Phức tạp hơn (phải tự xử lý ordering, reliability, duplicate), abstraction thấp hơn RPC.                             | Hệ thống cần loose coupling, task queue, logging, xử lý sự kiện dài hạn (RabbitMQ, Kafka). |
| **Stream-Oriented Communication** | Truyền dữ liệu liên tục (continuous data flow) với yêu cầu về timing (thường dùng UDP/TCP cho multimedia).  | Hiệu suất cao cho dữ liệu lớn & real-time, hỗ trợ streaming (Server/Client/Bidirectional).            | Không đảm bảo reliability nếu dùng UDP, rất nhạy cảm với delay, jitter, packet loss, khó xử lý ordering.             | Truyền dữ liệu real-time: video call, sensor stream, audio/video broadcasting.             |

## Baì 1:

a. Client gửi yêu cầu tính tổng hai số nguyên đến Server. Server nhận yêu cầu, tính toán kết quả và trả về cho Client. Giao tiếp giữa Client và Server phải được thực hiện thông qua
![Alt text](./bai1/cau1_a.png)
b. So sánh gRPC với REST API
