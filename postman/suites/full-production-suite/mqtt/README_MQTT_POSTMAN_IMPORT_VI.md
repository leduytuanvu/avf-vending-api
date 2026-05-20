# MQTT — Kiểm thử bằng Postman (Tiếng Việt)

## 1. Kết nối
- Postman Desktop → **New** → **MQTT**.
- Host `{{mqttHost}}`, port `{{mqttPort}}` — production thường **8883** (MQTTS).
- Username/password từ env (**để trống trong repo**; không hard-code).

## 2. Subscribe vs Publish
- **Subscribe** trước các pattern trong ma trận để quan sát (read, ít rủi ro hơn).
- **Publish** có thể làm thay đổi projection/OLTP — **chỉ** máy/canary đã phê duyệt.

## 3. Topic prefix
- Dùng `{{mqttTopicPrefix}}` nhất quán với broker ACL và `internal/platform/mqtt/topics.go` + `docs/api/mqtt-contract.md`.

## 4. Payload & ACK
- Mẫu payload: `mqtt/mqtt_request_templates.json`.
- QoS/retained: xem cột ma trận; nhiều chỗ ghi `unknown` nếu code không cố định.

## 5. Evidence
- Lưu: topic thực tế, thời điểm, `event_id`/`dedupe_key` trong payload, log ingest backend (nếu có quyền).

## 6. Vì sao bắt buộc canary khi publish
- Sai `machine_id` / `event_type` có thể ghi sai kho, commerce, audit — không publish vào fleet thật khi chưa được phê duyệt.
