# HTTP API của server

> **Bản nháp v0.** Xem [README](README.md) cho quy tắc version, capability và registry. Schema: [http-api.schema.json](schema/http-api.schema.json). Ví dụ: [examples/http-api](examples/http-api). Chỉ gồm endpoint mà app và agent dùng ở bản đầu; nhóm, chia sẻ, khoá SSH dán tay và khẩn cấp mới có tên dành sẵn ([cuối trang](#dành-sẵn-cho-bản-sau)).

## Quy ước

- **Gốc:** mọi endpoint nằm dưới `/v1` (trừ `/health/*`). Thêm endpoint không đổi tiền tố.
- **TLS:** server dùng HTTPS. Mặc định server tự ký chứng chỉ; app và agent ghim SHA-256 của **SPKI** (không phải của chứng chỉ lá) dạng `sha256:<hex>`, nên gia hạn chứng chỉ giữ nguyên khoá thì pin không đổi. Server đứng sau chứng chỉ CA hợp lệ thì client không ghim.
- **Nội dung:** thân yêu cầu và phản hồi là `application/json` UTF-8. Phản hồi thành công không có thân dùng `204`.
- **Xác thực:** `Authorization: Bearer <access_token>`. Token là chuỗi không trong suốt. Access token sống ngắn; refresh token dùng một lần (dùng lại thu hồi cả họ token).
- **Xác thực lại gần đây (step-up):** thao tác nhạy cảm cần người dùng nhập TOTP trong vòng vài phút gần nhất (`POST /v1/auth/step-up`); chưa thì `403` mã `auth.step_up_required`.
- **Lỗi:** thân là `ErrorResponse`: `{"error": {"code": "<miền>.<lý_do>", "message": "…", "details": {…}}}`. `code` ổn định để client ánh xạ sang thông báo hiển thị; `message` dành cho người vận hành. `details` tuỳ mã lỗi.
- **Giới hạn tần suất:** `429` mã `rate.limited` kèm tiêu đề `Retry-After` (giây). Quá tải tạm thời (ví dụ hàng đợi băm mật khẩu đầy): `503` mã `server.busy` kèm `Retry-After`.
- **Danh sách:** các endpoint danh sách trả trọn bộ, chưa phân trang. Thêm phân trang sau này chỉ là thêm trường (ví dụ `next_cursor`).
- **Thời gian** là Unix mili-giây UTC; **mã đối tượng** là chuỗi không trong suốt ([README](README.md#quy-ước-chung)).
- **Đổi quyền:** endpoint làm thay đổi quyền truy cập của máy nào đó trả `affected_machine_ids` để app hiện "đã áp trên N/M máy" bằng cách theo dõi `policy_state` của các máy đó.

## Capability và `GET /v1/meta`

`GET /v1/meta` không cần đăng nhập. App gọi nó đầu tiên để biết server hiểu những gì.

| Trường | Ý nghĩa |
|---|---|
| `server_version` | phiên bản server |
| `api` | `1`; đổi số này bị cấm trong v1 |
| `capabilities` | capability HTTP server hỗ trợ: hiện `devices.v1` |
| `setup_required` | `true` khi chưa có tài khoản nào và server đang chờ thiết lập lần đầu |

App chỉ hiện tính năng khi server (qua `capabilities`) và, với tính năng cần agent, khi máy (qua `agent_capabilities` của `GET /v1/machines`) quảng bá capability tương ứng. Thiếu thì ẩn tính năng, không hiện lỗi chung.

Ví dụ: [Meta.full](examples/http-api/Meta.full.json), [Meta.can-thiet-lap](examples/http-api/Meta.can-thiet-lap.json).

## Thiết lập lần đầu và xác thực

| Method và đường dẫn | Xác thực | Thân yêu cầu | Phản hồi |
|---|---|---|---|
| `POST /v1/setup` | mã thiết lập trong thân | `SetupRequest` | `201` `SetupResult` |
| `POST /v1/auth/login` | không | `LoginRequest` | `200` `TokenPair` |
| `POST /v1/auth/refresh` | không | `RefreshRequest` | `200` `TokenPair` |
| `POST /v1/auth/logout` | Bearer | không | `204` |
| `POST /v1/auth/step-up` | Bearer | `StepUpRequest` | `204` |
| `POST /v1/auth/totp/enroll` | Bearer (token thường hoặc token đăng ký TOTP) | không | `200` `TOTPEnrollment` |
| `POST /v1/auth/totp/confirm` | Bearer (như trên) | `TOTPConfirmRequest` | `200` `TOTPConfirmResult` |

**Thiết lập lần đầu.** Server mới in mã thiết lập (chỉ khi chưa có tài khoản) và đường dẫn dạng `<scheme>://setup?u=<url>&t=<mã>`. `GET /v1/meta` báo `setup_required: true`; app gọi `POST /v1/setup` với mã, email và mật khẩu để tạo chủ sở hữu đầu tiên. Phản hồi mang `totp_enrollment_token`, token tạm chỉ dùng để đăng ký TOTP; TOTP là bắt buộc.

**Đăng ký TOTP.** `POST /v1/auth/totp/enroll` trả khoá bí mật và URI `otpauth://`; app hiện cho người dùng quét hoặc nhập. `POST /v1/auth/totp/confirm` nhận mã đầu tiên. Thành công thì người dùng đã đăng nhập: phản hồi là `TokenPair` cộng `recovery_codes` (hiện đúng một lần).

**Đăng nhập** là một yêu cầu `LoginRequest`: email, mật khẩu, và đúng một trong `totp` hoặc `recovery_code`, kèm `device_name`.

- Mọi thất bại (email lạ, mật khẩu sai, TOTP sai) trả `401` `auth.invalid_credentials` y hệt, kể cả thời gian phản hồi, để không thành công cụ dò tài khoản. Tài khoản bị khoá mềm cũng trả lỗi y hệt; chỉ mã khôi phục còn dùng được.
- Tài khoản chưa đăng ký TOTP: `403` `auth.totp_enrollment_required` với `details.totp_enrollment_token` để tiếp tục luồng đăng ký.
- Mỗi đăng nhập tạo một **thiết bị** (phiên đăng nhập có tên). Thu hồi thiết bị thu hồi cả token và khoá thiết bị gắn với nó.

Ví dụ: [LoginRequest.totp](examples/http-api/LoginRequest.totp.json), [TokenPair.full](examples/http-api/TokenPair.full.json), [ErrorResponse.co-details](examples/http-api/ErrorResponse.co-details.json), [TOTPConfirmResult.full](examples/http-api/TOTPConfirmResult.full.json).

## Tài khoản hiện tại

Cần Bearer. Yêu cầu có dấu (*) cần xác thực lại gần đây (step-up).

| Method và đường dẫn | Thân yêu cầu | Phản hồi |
|---|---|---|
| `GET /v1/me` | | `Me` |
| `POST /v1/me/password` | `ChangePasswordRequest` | `204` |
| `GET /v1/me/devices` | | `DeviceList` |
| `DELETE /v1/me/devices/{id}` (*) | | `AffectedMachines` |
| `GET /v1/me/alerts?since=<ms>` | | `AlertList` |
| `DELETE /v1/me/soft-lock` (*) | | `204` |
| `POST /v1/me/keys/challenge` | | `KeyChallenge` |
| `GET /v1/me/keys` | | `KeyList` |
| `POST /v1/me/keys` | `RegisterKeyRequest` | `201` `RegisterKeyResult` |
| `DELETE /v1/me/keys/{id}` (*) | | `AffectedMachines` |

- **Cảnh báo** (`/v1/me/alerts`) lấy từ các sự kiện kiểm toán nổi bật: khoá mềm, khoá mới, thiết bị mới (về sau thêm khẩn cấp). Không phụ thuộc email. `since` lọc theo `ts`; `action` là tập mở, app hiện thông báo chung cho hành động lạ. `DELETE /v1/me/soft-lock` mở khoá từ một thiết bị đã đăng nhập.
- **Khoá thiết bị.** App tạo cặp khoá ECDSA P-256 trong kho khoá của thiết bị và đăng ký khoá công khai để agent cho phép đăng nhập SSH. Đăng ký cần chứng minh sở hữu: gọi `POST /v1/me/keys/challenge` lấy `nonce` (16 byte, dùng một lần, hết hạn nhanh), ký lên `nonce` và origin của server (ngữ cảnh `device-key-pop/1`, [sync.md](sync.md#mã-hoá-chữ-ký)) rồi gửi `RegisterKeyRequest` với `kind: "device"`, `nonce`, `sig`. Ràng buộc origin để chữ ký lấy được qua một server khác (server giả, hoặc server thứ hai mà app cũng đăng nhập) không dùng lại được để đăng ký khoá ở server này. `hardware_backed` và `security_level` do app tự khai, chỉ là thông tin. Vân tay khoá là duy nhất toàn hệ thống; trùng thì `409` `resource.conflict`.
- `kind` hiện có `device` và `ssh` (khoá dán tay, dành cho bản sau). `public_key` trong yêu cầu là dòng `authorized_keys` có thể kèm ghi chú; phản hồi trả bản chuẩn hoá.

Ví dụ: [Me.full](examples/http-api/Me.full.json), [DeviceList.full](examples/http-api/DeviceList.full.json), [AlertList.full](examples/http-api/AlertList.full.json), [RegisterKeyRequest.device](examples/http-api/RegisterKeyRequest.device.json), [RegisterKeyResult.full](examples/http-api/RegisterKeyResult.full.json).

## Thêm máy

Theo khung RFC 8628 (device authorization), nhưng thứ được cấp là **danh tính máy**, không bao giờ là token người dùng.

```
agent                          server                           app (đã đăng nhập)
  │── POST /v1/enroll/start ───►│  device_code, user_code
  │◄─────────────────────────────│
  │ hiện user_code XXXX-XXXX                                      │
  │                              │◄─ GET  /v1/enroll/{user_code} ─│  người dùng nhập mã
  │                              │── EnrollInfo ──────────────────►│  đối chiếu vân tay host key
  │                              │◄─ POST …/approve ──────────────│  (step-up)
  │── POST /v1/enroll/poll ─────►│  lặp mỗi `interval` giây
  │◄── machine_id, policy_keys ──│
```

| Method và đường dẫn | Xác thực | Thân yêu cầu | Phản hồi |
|---|---|---|---|
| `POST /v1/enroll/start` | không (có giới hạn tần suất) | `EnrollStartRequest` | `200` `EnrollStartResult` |
| `POST /v1/enroll/poll` | chữ ký trong thân | `EnrollPollRequest` | `200` `EnrollPollResult` hoặc `400` lỗi `enroll.*` |
| `GET /v1/enroll/{user_code}` | Bearer | | `EnrollInfo` |
| `POST /v1/enroll/{user_code}/approve` (*) | Bearer | `EnrollApproveRequest` | `201` `Machine` |
| `POST /v1/enroll/{user_code}/deny` | Bearer | | `204` |

- `user_code` gồm 8 ký tự lấy từ bảng 20 phụ âm `BCDFGHJKLMNPQRSTVWXZ`, hiển thị `XXXX-XXXX`. Server chấp nhận cả chữ thường và không có dấu gạch. Mã sống 10 phút và chỉ thử được vài lần trước khi bị vô hiệu.
- `device_code` chỉ agent giữ, không hiển thị.
- **Chứng minh sở hữu.** Mỗi lần poll, agent ký bằng host key (ngữ cảnh `enroll-pop/1`, [sync.md](sync.md#mã-hoá-chữ-ký)) lên `device_code` và origin của server. Chỉ khi chữ ký đúng thì mới được duyệt. Host key phải duy nhất trong hệ thống: máy nhân bản từ ảnh VM có cùng host key bị từ chối, buộc sinh host key mới.
- **Lỗi khi poll** theo RFC 8628, trạng thái `400`: `enroll.authorization_pending` (chưa duyệt, tiếp tục), `enroll.slow_down` (tăng khoảng cách poll, `details.interval` là khoảng mới), `enroll.expired_token` (hết hạn), `enroll.access_denied` (bị từ chối).
- **Duyệt.** Người duyệt thấy `hostname`, `os`, `os_user`, `requested_ip`, các dải mạng agent khai (`allowed_cidrs`) và vân tay host key, phải đối chiếu vân tay với màn hình máy. Máy mới thuộc tài khoản duyệt với vai trò `owner`; server chỉ giữ các địa chỉ agent báo nằm trong các dải đã duyệt.
- `EnrollPollResult.policy_keys` là các khoá ký snapshot quyền mà agent sẽ tin ([sync.md](sync.md#xoay-khoá-ký-chính-sách-policykeys)).

Ví dụ: [EnrollStartRequest.full](examples/http-api/EnrollStartRequest.full.json), [EnrollStartResult.full](examples/http-api/EnrollStartResult.full.json), [EnrollPollRequest.full](examples/http-api/EnrollPollRequest.full.json), [EnrollPollResult.full](examples/http-api/EnrollPollResult.full.json), [EnrollInfo.full](examples/http-api/EnrollInfo.full.json).

## Máy

Cần Bearer. Chỉ trả máy mà tài khoản có quyền.

| Method và đường dẫn | Thân yêu cầu | Phản hồi |
|---|---|---|
| `GET /v1/machines` | | `MachineList` |
| `GET /v1/machines/{id}` | | `Machine` |
| `PATCH /v1/machines/{id}` | `MachinePatch` | `Machine` |
| `DELETE /v1/machines/{id}` (*) | | `204` |
| `POST /v1/machines/{id}/local-revocations/{rid}/clear` (*) | | `AffectedMachines` |

`Machine` gộp dữ liệu lưu trên server với trạng thái trực tiếp từ kết nối của agent:

| Trường | Ý nghĩa |
|---|---|
| `addresses` | địa chỉ `host:port` để app kết nối SSH, đã lọc theo `allowed_cidrs` |
| `host_key_fp` | vân tay host key; app **bắt buộc** khớp khi kết nối SSH, không hỏi "có tin máy này không" |
| `online`, `session_count`, `last_seen_at` | từ kết nối đồng bộ; `last_seen_at` vắng nếu chưa từng thấy |
| `policy_state` | `in_sync`, `pending` hoặc `rejected`: agent đã xác nhận snapshot mới nhất chưa. Tập mở, giá trị lạ coi là `pending` |
| `agent_version`, `agent_capabilities` | để app ẩn tính năng mà agent cũ không có |
| `role` | vai trò của người gọi trên máy (`owner`, `admin`, `member`) |
| `local_revocations` | thu hồi đặt ngay trên máy còn hạn, để chủ máy thấy và gỡ |

Xoá máy: server gửi snapshot rỗng rồi đóng kết nối, nên máy không còn ai được vào. Gỡ thu hồi cục bộ làm tăng phiên bản snapshot với `clear_local_revocations` do server ký.

Ví dụ: [Machine.full](examples/http-api/Machine.full.json), [Machine.moi-tao-chua-ket-noi](examples/http-api/Machine.moi-tao-chua-ket-noi.json), [MachineList.full](examples/http-api/MachineList.full.json).

## Đồng bộ và sức khoẻ

| Đường dẫn | Mô tả |
|---|---|
| `GET /v1/agent/sync` | nâng cấp lên WebSocket cho agent; xem [sync.md](sync.md) |
| `GET /health/live` | `200` khi tiến trình sống; không cần đăng nhập |
| `GET /health/ready` | `200` khi sẵn sàng phục vụ (cơ sở dữ liệu mở được); không cần đăng nhập |

Thân của `/health/*` do server quyết định; client không được dựa vào nó, chỉ dựa vào mã trạng thái.

## Mã lỗi khởi đầu

Tập mở: client gặp mã lạ hiện thông báo chung.

| `code` | Trạng thái | Nghĩa |
|---|---|---|
| `request.invalid` | 400 | thân hoặc tham số không hợp lệ |
| `request.too_large` | 413 | thân vượt giới hạn |
| `auth.unauthenticated` | 401 | thiếu, hết hạn hoặc đã bị thu hồi token |
| `auth.invalid_credentials` | 401 | thông tin đăng nhập không đúng (đồng nhất cho mọi nguyên nhân) |
| `auth.totp_enrollment_required` | 403 | chưa đăng ký TOTP; `details.totp_enrollment_token` |
| `auth.step_up_required` | 403 | cần xác thực lại gần đây |
| `access.forbidden` | 403 | đã xác thực nhưng không có quyền |
| `resource.not_found` | 404 | không có hoặc ngoài phạm vi |
| `resource.conflict` | 409 | trùng hoặc vi phạm ràng buộc |
| `rate.limited` | 429 | vượt giới hạn tần suất; có `Retry-After` |
| `server.busy` | 503 | quá tải tạm thời; có `Retry-After` |
| `setup.invalid_token` | 403 | mã thiết lập sai, hoặc đã có tài khoản |
| `key.pop_invalid` | 400 | chứng minh sở hữu khoá sai |
| `enroll.authorization_pending` | 400 | thêm máy: chưa duyệt |
| `enroll.slow_down` | 400 | thêm máy: poll nhanh quá; `details.interval` |
| `enroll.expired_token` | 400 | thêm máy: mã hết hạn |
| `enroll.access_denied` | 400 | thêm máy: bị từ chối |

## Dành sẵn cho bản sau

Chỉ đăng ký tên; chưa có hợp đồng chi tiết. Server chỉ nêu capability trong `GET /v1/meta` khi đã hiện thực, và app ẩn màn hình tương ứng khi thiếu capability.

| Capability | Tài nguyên dự kiến |
|---|---|
| `groups.v1` | `/v1/groups`, `/v1/groups/{id}/members`, `/v1/invites` |
| `sharing.v1` | `/v1/machines/{id}/grants` |
| `ssh_keys.v1` | khoá `kind: "ssh"` ở `/v1/me/keys`, `/v1/me/known_hosts` |
| `emergency.v1` | `/v1/machines/{id}/emergency-secret`, cờ khẩn cấp của máy |
