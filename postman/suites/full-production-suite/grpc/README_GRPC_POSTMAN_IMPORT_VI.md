# gRPC — Import vào Postman (Tiếng Việt)

## 1. Tạo gRPC request
- Postman Desktop → **New** → **gRPC Request**.

## 2. Server
- URL: `{{grpcHost}}:{{grpcPort}}` (TLS nếu listener yêu cầu — thường là mạng nội bộ).
- Nếu chưa biết endpoint production, để trống host/port và lấy từ đội vận hành / `deployments`.

## 3. Import proto
- **File gộp:** `grpc/avf_all_services.proto` (tiện chọn service/method).
- **Hoặc** thêm thư mục `grpc/proto` làm import root (chứa package `avf/` như repo gốc) nếu bạn cần đường import y hệt compiler.

## 4. Chọn service / method
- Tra cứu `fullMethod` trong `AVF_GRPC_86_METHOD_MATRIX.csv` (86 RPC).

## 5. Metadata (Authorization)
- `authorization: Bearer {{machineToken}}` cho package runtime **machine**.
- `authorization: Bearer {{accessToken}}` cho **internal/admin** và proto legacy song song.
- Thêm `x-request-id` / `x-correlation-id` nếu Postman không tự điền — template JSON trong repo có ví dụ.

## 6. Message (body)
- Dán JSON từ `grpc/grpc_request_templates.json` tương ứng `fullMethod` (Protobuf JSON encoding).

## 7. An toàn
- Ưu tiên RPC `destructiveLevel=READ_ONLY` / ma trận ghi rõ read-only.
- RPC ghi coi như canary; chỉ chạy từ mạng được phép và có token đúng vai trò.
- Cột `registeredOnListener` / `listenerBinding`: RPC có thể chỉ tồn tại trong proto mà không gắn listener public.
