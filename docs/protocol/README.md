# Hợp đồng giao thức

> **Bản nháp v0.** Nội dung còn sửa tự do cho tới khi đóng băng (lúc phát hành bản beta). Sau đó chỉ được cộng thêm theo các quy tắc dưới đây. Định danh trên dây dùng major `1` ngay từ bây giờ (`/v1`, `proto: 1`) nên khi đóng băng không phải đổi tên gì; "v0" chỉ là nhãn của tài liệu.

Thư mục này là **nguồn sự thật, không gắn với ngôn ngữ nào**, cho mọi bên nói chuyện với nhau: agent, server, app và client SSH bất kỳ. Mã không định nghĩa giao thức; mã của từng thành phần tuân theo tài liệu này và được kiểm bằng cùng một bộ ví dụ, viết bằng ngôn ngữ nào cũng vậy ([Cài đặt và kiểm tra](#cài-đặt-và-kiểm-tra)).

| Tài liệu | Nội dung |
|---|---|
| [rpc.md](rpc.md) | RPC của agent qua subsystem SSH `agent-rpc`, kênh terminal (phiên, snapshot, kích thước, mã thoát) |
| [sync.md](sync.md) | Kênh đồng bộ agent–server qua WebSocket: bắt tay, snapshot quyền có chữ ký, trạng thái máy, kiểm toán, đăng nhập khẩn cấp |
| [http-api.md](http-api.md) | HTTP API của server cho app và cho agent lúc thêm máy |
| [schema/](schema/) | JSON Schema 2020-12 của mọi thông điệp JSON (`rpc`, `sync`, `http-api`) |
| [examples/](examples/) | Ví dụ cho từng thông điệp, gồm ca không hợp lệ |

## Quy tắc version và capability

1. Mỗi giao thức có một số `proto` nguyên (hiện là `1`): RPC agent, đồng bộ, và `api` của HTTP. **Đổi `proto` là thay đổi phá vỡ và bị cấm trong v1.**
2. Tính năng mới là một **capability** dạng `<miền>.v<n>`. Hai bên trao đổi danh sách capability lúc bắt tay, mỗi bên liệt kê mọi thứ nó hiểu (dù đóng vai gọi hay vai được gọi). Chỉ được gọi method của một nhóm khi bên kia đã quảng bá capability của nhóm đó.
3. Bên nhận **bỏ qua trường lạ**. Không được xoá trường cũ, đổi kiểu hay đổi nghĩa của trường cũ. Giá trị enum lạ được coi là "không biết" (vai trò lạ coi là `member`, trạng thái lạ coi là `pending`, chủ đề sự kiện lạ thì bỏ qua), không phải lỗi. Vì vậy schema không dùng `additionalProperties: false` và không đóng các tập giá trị.
4. **Method lạ trả `-32601`**, ở mọi thời điểm của kết nối, kể cả trước khi bắt tay xong: mã riêng của giai đoạn bắt tay (`-32600` ở RPC agent, `-32001` ở kênh đồng bộ) chỉ dành cho method đã biết, để `-32601` luôn chỉ có một nghĩa. Bên gọi coi đó là "bên kia không hỗ trợ" và dùng tiếp phần còn lại của kết nối; không được ngắt kết nối vì thế.
5. HTTP: mọi endpoint nằm dưới tiền tố `/v1`; `GET /v1/meta` trả `{server_version, api, capabilities, setup_required}`; thêm endpoint không đổi tiền tố. Tính năng HTTP thêm sau bản `v0.1.0` phải có capability trong `GET /v1/meta` để app ẩn tính năng khi gặp server cũ. `GET /v1/machines` trả `agent_capabilities` của từng máy để app ẩn tính năng mà agent cũ không có.
6. **Định danh trên dây không suy ra từ tên sản phẩm** (tên biến môi trường mang mã phiên, ngữ cảnh chữ ký, tên method và capability). Sản phẩm có thể đổi tên; client và agent đã phát hành thì không đổi theo. Tên biến môi trường chỉ được thêm alias, không xoá.
7. **Tên trường phân biệt hoa thường.** `Proto` không phải `proto`. Bên gửi phải dùng đúng tên như schema, và bên nhận không được dựa vào việc gộp hoa thường. `encoding/json` của Go khớp tên trường không phân biệt hoa thường, còn Python và Dart thì phân biệt, nên thông điệp viết sai hoa thường vẫn được bên Go hiểu nhưng bị bên Python và Dart coi là trường lạ; lỗi chỉ lộ ra khi gặp cài đặt ở ngôn ngữ khác.

Hệ quả cho người viết mã:

- Gọi method mới: kiểm `Has(capabilities của bên kia, <capability>)` trước; nếu bên kia vẫn trả `-32601` thì coi là "không hỗ trợ".
- Thêm method mới: đặt vào capability mới (hoặc thêm `v<n+1>` cho nhóm cũ), thêm vào registry dưới đây; không đụng `proto`.
- Đổi hành vi một method đã có theo cách client cũ không hiểu: không được; đó là capability mới với method mới.

## Quy ước chung

- **Mã hoá:** JSON UTF-8; tên trường `snake_case`.
- **Thời gian:** số nguyên Unix **mili-giây** UTC, tên trường kết thúc bằng `_at` (trừ `ts`, `not_before`).
- **Mã đối tượng** (`ID`): chuỗi không trong suốt khớp `^[A-Za-z0-9_-]{1,64}$`. Bộ ký tự hẹp vì mã phiên đi vào biến môi trường, tên tệp và tên pipe.
- **Danh sách:** trường kiểu danh sách vắng mặt nghĩa là **rỗng** (trừ khi tài liệu nói khác, ví dụ `grants` của snapshot quyền luôn có mặt). Người gửi không cần phân biệt `[]` với vắng; người nhận không được coi `null` là "không có thông tin".
- **Base64:** chuẩn (RFC 4648 §4) có đệm `=`, trừ vân tay khoá (không đệm).
- **Khoá công khai:** một dòng định dạng `authorized_keys` không kèm tuỳ chọn hay ghi chú: `<kiểu khoá> <base64>`.
- **Vân tay khoá:** `SHA256:` cộng base64 không đệm của SHA-256 trên blob khoá, giống `ssh-keygen -lf`.
- **Chữ ký:** dạng SSH (RFC 4253 §6.6: `string(định dạng)` ghép `string(blob)`) rồi base64 chuẩn. Dữ liệu được ký và ngữ cảnh xem [sync.md](sync.md#mã-hoá-chữ-ký).
- **JSON-RPC:** bản 2.0, không hỗ trợ batch. Yêu cầu được xử lý đồng thời nên không có bảo đảm thứ tự giữa các yêu cầu. **Notification là best-effort**: bên nhận có thể bỏ khi quá tải, nên mọi thứ không được phép mất phải đi bằng yêu cầu có xác nhận (vì vậy `policy.apply` và `audit.report` là yêu cầu).
- **Bí mật** (mật khẩu, mã TOTP, token, bí mật khẩn cấp) không bao giờ được ghi vào log hay sự kiện kiểm toán.

## Registry

Registry khởi đầu dưới đây; về sau **chỉ được thêm**. Mỗi cài đặt nên có test đối chiếu định danh trong mã của mình với các bảng này.

### Capability

| Capability | Miền | Mở khoá | Ai quảng bá |
|---|---|---|---|
| `session.v1` | RPC agent | `session.list`, `session.create`, `session.close`, `session.size` | agent trong `hello` |
| `events.v1` | RPC agent | `events.subscribe` và notification `event` | agent trong `hello` |
| `policy.v1` | đồng bộ | `policy.apply`, `policy.keys` | cả hai bên trong `hello` |
| `status.v1` | đồng bộ | `machine.status` | cả hai bên trong `hello` |
| `audit.v1` | đồng bộ | `audit.report` | cả hai bên trong `hello` |
| `emergency.v1` | đồng bộ, HTTP | `emergency.verify`; API khẩn cấp của server | cả hai bên trong `hello`; server còn nêu trong `GET /v1/meta` |
| `devices.v1` | HTTP | thiết bị, khoá thiết bị, đăng xuất (có ngay từ bản đầu) | server trong `GET /v1/meta` |
| `groups.v1` | HTTP | nhóm và lời mời (dành sẵn, chưa có hợp đồng chi tiết) | server trong `GET /v1/meta` |
| `sharing.v1` | HTTP | chia sẻ máy (dành sẵn) | server trong `GET /v1/meta` |
| `ssh_keys.v1` | HTTP | khoá SSH dán tay, `known_hosts` (dành sẵn) | server trong `GET /v1/meta` |

Không có `oidc.v1` ở v1: đăng nhập OIDC thêm sau theo quy tắc chỉ cộng thêm.

### Method RPC của agent

Subsystem SSH `agent-rpc`. Chi tiết ở [rpc.md](rpc.md).

| Method | Capability | Hướng |
|---|---|---|
| `hello` | (không) | client → agent |
| `session.list` | `session.v1` | client → agent |
| `session.create` | `session.v1` | client → agent |
| `session.close` | `session.v1` | client → agent |
| `session.size` | `session.v1` | client → agent |
| `events.subscribe` | `events.v1` | client → agent |
| `event` (notification) | `events.v1` | agent → client |

### Chủ đề sự kiện

`session.created`, `session.closed`, `session.attached`, `session.detached`, `session.resized` — gọi bằng mẫu `events.subscribe` như `session.*`. Hình dạng `data` của từng chủ đề ở [rpc.md](rpc.md#sự-kiện).

### Kênh terminal (SSH)

| Tên | Giá trị | Ý nghĩa |
|---|---|---|
| Subsystem | `agent-rpc` | mang RPC của agent |
| Biến phiên | `RTERM_SESSION` | tên chính; client gửi bằng yêu cầu `env`, agent đặt vào môi trường shell. Agent nhận mọi alias. Chỉ được thêm alias |
| Giá trị đặc biệt | `new` | biến phiên bằng giá trị này nghĩa là tạo phiên mới |
| Mã thoát | `66` | `ExitSessionNotFound`: mã phiên trong biến phiên không còn |
| Dòng stderr | `session_not_found` | đi kèm mã thoát `66` |

### Method của kênh đồng bộ

WebSocket `GET /v1/agent/sync`. Chi tiết ở [sync.md](sync.md).

| Method | Capability | Hướng | Loại |
|---|---|---|---|
| `hello` | (không) | agent → server | yêu cầu |
| `auth` | (không) | agent → server | yêu cầu |
| `policy.apply` | `policy.v1` | server → agent | yêu cầu (kết quả là xác nhận) |
| `policy.keys` | `policy.v1` | server → agent | yêu cầu |
| `machine.status` | `status.v1` | agent → server | notification |
| `audit.report` | `audit.v1` | agent → server | yêu cầu |
| `emergency.verify` | `emergency.v1` | agent → server | yêu cầu |

### HTTP

| Mục | Giá trị |
|---|---|
| Tiền tố | `/v1` |
| Endpoint đồng bộ | `/v1/agent/sync` |
| Capability trong `GET /v1/meta` | `devices.v1`; dành sẵn `groups.v1`, `sharing.v1`, `ssh_keys.v1`, `emergency.v1` |
| Lỗi | `{"error":{"code":"<miền>.<lý_do>","message":"…"}}` |

Danh sách endpoint và mã lỗi HTTP ở [http-api.md](http-api.md).

### Mã lỗi JSON-RPC riêng

Dùng cho cả RPC agent lẫn kênh đồng bộ, và đã đóng băng như mọi định danh trên dây khác (tên trong bảng là tên trong bản đóng băng). `error.data` của `-32001`…`-32004` là `ErrorData` (`reason`, `retry_after`, đều tuỳ chọn).

| Mã | Tên | Dùng khi |
|---|---|---|
| `-32000` | `server_busy` | bên nhận đang xử lý quá nhiều yêu cầu đồng thời; thư viện JSON-RPC trả mã này, không kèm `error.data`, bên gọi thử lại sau |
| `-32001` | `unauthorized` | chưa xác thực hoặc không đủ quyền |
| `-32002` | `not_found` | đối tượng không tồn tại hoặc nằm ngoài phạm vi của người gọi |
| `-32003` | `limit_exceeded` | vượt giới hạn; `retry_after` (giây) khi chờ được |
| `-32004` | `conflict` | trạng thái hiện tại không cho phép; `reason` nêu lý do |

Mã chuẩn của JSON-RPC (`-32700`, `-32600`, `-32601`, `-32602`, `-32603`) giữ nguyên nghĩa.

### Ngữ cảnh chữ ký

Mỗi loại chữ ký có ngữ cảnh riêng để chữ ký ở miền này không dùng lại được ở miền khác. Ngữ cảnh trung tính, không chứa tên sản phẩm. Cách ghép vào dữ liệu ký xem [sync.md](sync.md#mã-hoá-chữ-ký).

| Ngữ cảnh | Ký cái gì | Ai ký |
|---|---|---|
| `policy-snapshot/1` | payload snapshot quyền | server (khoá ký chính sách) |
| `policy-key-rollover/1` | chứng thực khoá ký chính sách mới | khoá ký chính sách đang tin |
| `sync-auth/1` | thử thách của kênh đồng bộ | agent (host key) |
| `enroll-pop/1` | chứng minh sở hữu host key khi thêm máy | agent (host key) |
| `device-key-pop/1` | chứng minh sở hữu khoá thiết bị | app (khoá trong kho khoá thiết bị) |
| `release-keyset/1` | bộ khoá phát hành (namespace SSHSIG) | khoá gốc phát hành, ngoại tuyến |
| `release-manifest/1` | manifest bản phát hành (namespace SSHSIG) | khoá phát hành, ngoại tuyến |

### Quy tắc thêm vào registry

1. Thêm định nghĩa vào schema và ví dụ (hợp lệ và không hợp lệ).
2. Thêm tên vào bảng ở trên.
3. Cập nhật từng cài đặt (kiểu dữ liệu, bản đóng băng định danh, bảng conformance); test của cài đặt nào bị bỏ sót sẽ đỏ.
4. Không sửa, không xoá mục đã có. Cần hành vi khác thì thêm mục mới.

## Schema và ví dụ

```
schema/<miền>.schema.json            miền: rpc | sync | http-api
examples/<miền>/<Def>.<biến thể>.json
```

- Ví dụ `examples/<miền>/<Def>.<biến thể>.json` được kiểm với `schema/<miền>.schema.json#/$defs/<Def>`. Giữa các tệp schema dùng `$ref` tương đối (`rpc.schema.json#/$defs/ID`).
- Đuôi `.invalid.json` đánh dấu ví dụ **không hợp lệ**: schema phải từ chối nó, và tên biến thể nói lý do (`HelloParams.proto-2.invalid.json`). Chúng chứng minh schema không bị lỏng; mỗi ví dụ như vậy chỉ sai đúng một điểm.
- Biến thể `unknown-fields` là ví dụ hợp lệ có thêm trường lạ (đặt tên `x_…`) ở nhiều tầng; bên nhận phải phân tích được và giữ nguyên phần đã biết.
- Mỗi thông điệp gốc (def không được def khác tham chiếu) phải có ít nhất một ví dụ hợp lệ. Ví dụ hợp lệ viết ở dạng chuẩn của người gửi: trường tuỳ chọn không có giá trị thì vắng mặt. Nhờ đó mỗi cài đặt kiểm được rằng đọc rồi ghi lại cho ra đúng ví dụ.
- **Chữ ký trong ví dụ là chữ ký thật** của khoá thử nghiệm công khai (không phải bí mật, không bảo vệ gì), nên dùng được làm test vector: mỗi cài đặt có thể dựng lại dữ liệu được ký đúng như [sync.md](sync.md#mã-hoá-chữ-ký) mô tả rồi kiểm chữ ký. Mọi khoá trong ví dụ suy ra tất định từ hạt giống ghi ở [Test vector](sync.md#test-vector) (ví dụ `test-vector/host-key`). Đổi đầu vào của chữ ký (như thêm trường vào `PolicySnapshot.full`) thì sinh lại khoá, payload và chữ ký bằng `go test ./internal/protocol -run TestViDuCoChuKyKhopBoSinh -update`; không có `-update`, test đó đỏ khi ví dụ lệch bộ sinh. Dữ liệu ví dụ khác như token, mật khẩu và mã TOTP đều là giá trị giả.

## Cài đặt và kiểm tra

Hợp đồng không gắn với ngôn ngữ nào. Mỗi thành phần có bản cài đặt riêng, chạy conformance trên cùng bộ `examples/` cho các miền nó dùng, và so định danh trên dây mà nó khai báo với bản đóng băng `internal/protocol/testdata/wire_names.golden.json`:

| Thành phần | Ngôn ngữ | Miền | Conformance | Bản đóng băng |
|---|---|---|---|---|
| agent | Go | `rpc`, `sync`, `http-api` | `internal/protocol/conformance_test.go` | `internal/protocol/wire_names_test.go` |
| app | Dart | `rpc`, `http-api` | `app/test/protocol/conformance_test.dart` | `app/test/protocol/wire_names_test.dart` |
| server | Python | `sync`, `http-api` | `server/tests/test_protocol_conformance.py` | `server/tests/test_wire_names.py` |

Conformance gồm bốn việc: ví dụ hợp lệ được nhận; đọc rồi ghi lại giữ nguyên trường đã biết; trường lạ không làm hỏng việc đọc; ví dụ `.invalid.json` bị từ chối. Cách kiểm khác nhau theo công cụ của từng ngôn ngữ:

- Go và Python kiểm ví dụ bằng JSON Schema. Thư viện của Python bỏ qua `contentSchema`, nên test tự giải `payload` của `SignedEnvelope` rồi kiểm với `PolicySnapshot`.
- Dart không chạy JSON Schema mà kiểm bằng model (model mang ràng buộc của schema), và chỉ với thông điệp app đã có model; thông điệp còn lại của các miền app dùng phải nằm trong danh sách chưa có model của test.
- Đọc rồi ghi lại dùng kiểu dữ liệu của chính cài đặt: kiểu Go (giải mã nghiêm ngặt, nên kiểu Go phải khai đủ trường của ví dụ), model Pydantic (test còn so tập trường và trường bắt buộc với schema), model Dart.
- Chỉ phía Go kiểm chữ ký thật trong ví dụ theo [Mã hoá chữ ký](sync.md#mã-hoá-chữ-ký) và giữ bộ sinh [test vector](sync.md#test-vector).
- Bản đóng băng: Go và Python so mọi định danh mình khai báo; Dart hiện so chủ đề sự kiện và `proto` của RPC, vì app chưa khai báo hằng nào khác. Python và Dart xếp từng mục của bản đóng băng vào nhóm được so hoặc nhóm không dùng (kèm lý do), nên mục mới làm test của hai bên đỏ cho tới khi được xếp chỗ.

## Thêm một thông điệp mới

1. Viết def trong `schema/<miền>.schema.json`; chỉ `required` những trường mà mọi người gửi bản v1 đều có.
2. Viết ít nhất một ví dụ hợp lệ và một ví dụ `.invalid.json` cho điều kiện quan trọng nhất.
3. Cập nhật kiểu dữ liệu ở từng cài đặt, khớp tên trường và cách xử lý trường tuỳ chọn.
4. Mô tả trong tài liệu của miền (method, capability, lỗi).
5. Nếu thêm method hoặc capability: cập nhật registry như trên.
