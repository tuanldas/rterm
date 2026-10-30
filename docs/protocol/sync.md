# Đồng bộ agent–server

> **Bản nháp v0.** Xem [README](README.md) cho quy tắc version, capability và registry. Schema: [sync.schema.json](schema/sync.schema.json). Ví dụ: [examples/sync](examples/sync).

Mỗi agent giữ **một kết nối đi ra** tới server. Qua kết nối đó server đẩy snapshot quyền xuống và agent báo trạng thái, kiểm toán và hỏi xác minh khẩn cấp. Server không nằm trên đường dữ liệu terminal; nó chỉ là nguồn sự thật về danh tính và quyền.

## Vận chuyển

- WebSocket trên TLS tại `GET /v1/agent/sync`. Agent là bên kết nối nên không cần mở cổng vào và đi qua được NAT.
- Mỗi thông điệp WebSocket là một khung **text** chứa đúng một thông điệp JSON-RPC 2.0 (không NDJSON, không batch). Khung nhị phân bị từ chối. Giới hạn kích thước một khung lấy từ cấu hình giới hạn (mặc định 1 MiB); vượt giới hạn thì bên nhận đóng kết nối.
- Server gửi ping WebSocket định kỳ (không quá 25 giây một lần) để reverse proxy không cắt kết nối nhàn rỗi; bên nào không nhận được pong thì đóng.
- Mỗi máy có tối đa một kết nối đã xác thực: kết nối mới xác thực xong thay kết nối cũ, và kết nối cũ đóng muộn không được gỡ nhầm kết nối mới.
- **Nối lại:** agent nối lại khi mất kết nối, theo lùi mũ có jitter đầy đủ từ 1 giây tới trần 60 giây, đặt lại khi kết nối ổn định từ 60 giây, và tôn trọng `retry_after` nếu server nêu.
- Lỗi dùng mã chuẩn và các mã riêng `-32000`…`-32004` ([registry](README.md#mã-lỗi-json-rpc-riêng)); `error.data` của `-32001`…`-32004` là `ErrorData`.

## Bắt tay và xác thực

```
agent                                         server
  │── hello {proto, machine_id, applied, …} ───►│
  │◄── {proto, server_version, nonce, …} ───────│   nonce 128 bit ngẫu nhiên
  │── auth {sig} ──────────────────────────────►│   sig = host key ký thử thách
  │◄── {} ───────────────────────────────────────│   từ đây kết nối đã xác thực
  │◄── policy.apply {payload, sig, key_id} ──────│   server đẩy snapshot hiện hành
  │── {epoch, rev} ─────────────────────────────►│   xác nhận
  │── machine.status {…} ───────────────────────►│   (notification, định kỳ và khi đổi)
```

1. **`hello`** (agent → server, yêu cầu): tham số `SyncHello` gồm `proto`, `machine_id`, `agent_version`, `capabilities` và `applied` (snapshot agent đang áp dụng). Kết quả `SyncHelloResult` mang `nonce`. Server trả nonce kể cả khi không biết `machine_id`, để không lộ máy nào tồn tại.
2. **`auth`** (agent → server, yêu cầu): tham số `SyncAuth {sig}`. `sig` là chữ ký của host key (ngữ cảnh `sync-auth/1`) trên thử thách; cách dựng dữ liệu ký ở [Mã hoá chữ ký](#mã-hoá-chữ-ký). Server kiểm bằng host key đã lưu lúc thêm máy. Kết quả là `{}`.
3. Thất bại (máy lạ, chữ ký sai, `machine_id` khác lúc `hello`) đều trả `-32001` y hệt nhau. Trước khi `auth` thành công, mọi method đã biết khác `hello` và `auth` trả `-32001`; method không tồn tại luôn nhận `-32601` ([quy tắc 4](README.md#quy-tắc-version-và-capability)). Quá hạn xác thực (mặc định 10 giây kể từ lúc mở kết nối) thì server đóng.
4. Sau `auth`, server tin `applied` và `capabilities` trong `hello` và đẩy ngay snapshot hiện hành nếu `applied` đã cũ.

Thử thách ràng buộc chữ ký vào **origin** của server mà agent đã kết nối, vào `machine_id` và vào `proto`; vì vậy chữ ký lấy được từ một server giả không dùng lại được ở server thật. Không có bí mật nào đi trên dây.

Ví dụ: [SyncHello.full](examples/sync/SyncHello.full.json), [SyncHelloResult.full](examples/sync/SyncHelloResult.full.json), [SyncAuth.full](examples/sync/SyncAuth.full.json).

## Mã hoá chữ ký

Mọi chữ ký của hợp đồng dùng chung một khung:

```
dữ_liệu_ký = ngữ_cảnh ‖ 0x00 ‖ thông_điệp
```

- `ngữ_cảnh` là các byte ASCII của tên ngữ cảnh ([registry](README.md#ngữ-cảnh-chữ-ký)), ví dụ `sync-auth/1`. Ngữ cảnh không chứa byte `0x00`.
- `0x00` là một byte phân cách. Nhờ ngữ cảnh đứng đầu, chữ ký ở miền này không hợp lệ ở miền khác dù dùng chung khoá.
- `thông_điệp` xác định theo từng ngữ cảnh ở bảng dưới. `‖` trong dòng trên là **nối thô**; trong bảng dưới `‖` giữa các trường là nối các trường đã mã hoá như sau.
- **Trường nhiều thành phần** mã hoá theo kiểu của SSH (RFC 4251 §5), không nối thô để ranh giới giữa các trường không nhập nhằng: `string(x)` là độ dài của `x` dạng `uint32` big-endian rồi các byte của `x`; `uint32(n)` và `uint64(n)` là 4 và 8 byte big-endian.
- **Chữ ký** có dạng SSH (RFC 4253 §6.6): `string(định dạng)` ‖ `string(blob)`, rồi base64 chuẩn có đệm. Với `ssh-ed25519` blob là chữ ký 64 byte của Ed25519 trên `dữ_liệu_ký`. Với `ecdsa-sha2-nistp256` chữ ký tính trên SHA-256 của `dữ_liệu_ký` và blob là `mpint(r)` ‖ `mpint(s)` (RFC 5656 §3.1.2).

| Ngữ cảnh | Khoá ký | `thông_điệp` |
|---|---|---|
| `policy-snapshot/1` | khoá ký chính sách của server (Ed25519) | đúng các byte của `payload` **sau khi giải base64** (xem [Snapshot quyền](#snapshot-quyền-policyapply)) |
| `policy-key-rollover/1` | khoá ký chính sách đang tin | `string(key_id)` ‖ `string(public_key)` ‖ `uint64(not_before)`, với `public_key` là đúng văn bản UTF-8 của trường đó |
| `sync-auth/1` | host key của máy | `string(nonce)` ‖ `string(origin)` ‖ `string(machine_id)` ‖ `uint32(proto)`, với `nonce` là 16 byte đã giải base64 |
| `enroll-pop/1` | host key của máy | `string(device_code)` ‖ `string(origin)` |
| `device-key-pop/1` | khoá thiết bị của app (ECDSA P-256) | `string(nonce)` ‖ `string(origin)`, với `nonce` là 16 byte đã giải base64 |

**`origin`** là URL gốc của server dạng chuẩn hoá `scheme://host:port`: scheme và host viết thường, IPv6 trong ngoặc vuông, cổng **luôn ghi tường minh** (https mặc định 443), không có path, query hay dấu `/` cuối. Bên ký lấy từ URL server mà nó đang dùng; bên kiểm so với các URL công khai mà server được cấu hình, cũng chuẩn hoá như vậy.

### Test vector

Các ví dụ có chữ ký đều là chữ ký thật của khoá thử nghiệm công khai, để mỗi cài đặt dựng lại dữ liệu ký đúng như bảng trên rồi kiểm chữ ký, độc lập với nơi sinh ra ví dụ. Origin của các ví dụ là `https://hub.example.test:7443`.

| Chữ ký | Ví dụ | Đầu vào lấy từ |
|---|---|---|
| `policy-snapshot/1` | [SignedEnvelope.full](examples/sync/SignedEnvelope.full.json) | khoá `policy-key-1` ở [EnrollPollResult.full](examples/http-api/EnrollPollResult.full.json) |
| `sync-auth/1` | [SyncAuth.full](examples/sync/SyncAuth.full.json) | `nonce` ở [SyncHelloResult.full](examples/sync/SyncHelloResult.full.json); `machine_id` ở [SyncHello.full](examples/sync/SyncHello.full.json); host key ở [EnrollStartRequest.full](examples/http-api/EnrollStartRequest.full.json) |
| `policy-key-rollover/1` | [KeyEndorsement.full](examples/sync/KeyEndorsement.full.json) | khoá ký `policy-key-1` như trên |
| `enroll-pop/1` | [EnrollPollRequest.full](examples/http-api/EnrollPollRequest.full.json) | `device_code` ở [EnrollStartResult.full](examples/http-api/EnrollStartResult.full.json); host key như trên |
| `device-key-pop/1` | [RegisterKeyRequest.device](examples/http-api/RegisterKeyRequest.device.json) | `nonce` ở [KeyChallenge.full](examples/http-api/KeyChallenge.full.json); khoá thiết bị ở chính ví dụ đó |

**Khoá thử nghiệm.** Mọi khoá trong ví dụ, kể cả khoá chỉ xuất hiện như dữ liệu, suy ra tất định từ một hạt giống cố định. Ai cũng sinh lại được nên chúng không phải bí mật, không bảo vệ gì, và không được dùng ở nơi nào khác.

| Khoá | Kiểu | Hạt giống | Khai báo ở |
|---|---|---|---|
| khoá ký chính sách `policy-key-1` | Ed25519 | `test-vector/policy-key-1` | [EnrollPollResult.full](examples/http-api/EnrollPollResult.full.json) |
| khoá ký chính sách mới `policy-key-2` | Ed25519 | `test-vector/policy-key-2` | [KeyEndorsement.full](examples/sync/KeyEndorsement.full.json) |
| host key của máy | Ed25519 | `test-vector/host-key` | [EnrollStartRequest.full](examples/http-api/EnrollStartRequest.full.json) |
| khoá thiết bị | ECDSA P-256 | `test-vector/device-key` | [RegisterKeyRequest.device](examples/http-api/RegisterKeyRequest.device.json) |
| khoá SSH của chủ máy | Ed25519 | `test-vector/ssh-key-owner` | `grants[0]` của [PolicySnapshot.full](examples/sync/PolicySnapshot.full.json) |
| khoá SSH của thành viên | Ed25519 | `test-vector/ssh-key-member` | `grants[1]` của [PolicySnapshot.full](examples/sync/PolicySnapshot.full.json) |

- Hạt giống là chuỗi ASCII viết đúng như trong bảng.
- Ed25519: khoá riêng có seed 32 byte (RFC 8032) bằng SHA-256 của hạt giống.
- ECDSA P-256: khoá riêng là SHA-256 của hạt giống, đọc như số nguyên big-endian (SEC 1 §2.3.6). Chữ ký tất định theo RFC 6979 với SHA-256.
- `payload` của SignedEnvelope.full là base64 của PolicySnapshot.full ở dạng JSON gọn: bỏ khoảng trắng nằm ngoài chuỗi, giữ nguyên thứ tự trường.

Ed25519 vốn ký tất định và ECDSA ký theo RFC 6979, nên cùng đầu vào luôn cho cùng byte. Khi đầu vào đổi (thêm trường vào PolicySnapshot.full, đổi `nonce` hay `device_code`), chạy lệnh sau ở gốc repo để sinh lại khoá, vân tay, payload và chữ ký:

```sh
go test ./internal/protocol -run TestViDuCoChuKyKhopBoSinh -update
```

Bộ sinh đặt khoá vào nơi khai báo rồi thay mọi bản sao của khoá, vân tay, payload và chữ ký ở các ví dụ khác, kể cả ví dụ không hợp lệ; ngoài các giá trị đó, tệp giữ nguyên từng byte. Chạy không có `-update`, test đó đỏ khi:

- ví dụ đã commit lệch kết quả của bộ sinh;
- sau khi sinh, một nơi khai báo khoá, `payload` hay một chữ ký không mang đúng giá trị bộ sinh tính cho chính nó (ca chép ví dụ, xem dưới);
- một chuỗi bắt đầu bằng kiểu khoá (`ssh-ed25519 AAAA…`, `ecdsa-sha2-nistp256 AAAA…`) không phải khoá thử nghiệm, một chuỗi bắt đầu bằng `SHA256:` không phải vân tay của khoá thử nghiệm, hoặc một trường tên đúng `sig` mang chữ ký mà bộ sinh không tạo ra.

Phần kiểm cuối có giới hạn: nó chỉ thấy khoá và vân tay đứng đầu chuỗi, và chữ ký ở trường tên `sig` (mọi trường chữ ký của schema hiện đều tên như vậy). Khoá nằm giữa một chuỗi, hay chữ ký ở trường mang tên khác, thì lọt qua phần kiểm này.

Ví dụ mới cần khoá thì dùng khoá có sẵn trong bảng trên, hoặc thêm hạt giống và nơi khai báo vào `testKeys` của `internal/protocol/example_vectors_test.go` rồi chạy lệnh trên; ví dụ mới có chữ ký thì thêm chỗ ký vào `signedValues` của cùng tệp. Ví dụ mới chép từ ví dụ có sẵn thì vị trí mới còn mang đúng khoá hay chữ ký của vị trí cũ. Bộ sinh không biết bản sao nào thuộc vị trí nào nên dừng mà không ghi tệp nào, và in các vị trí trùng cùng giá trị đúng của từng vị trí: sửa tay các vị trí đó theo giá trị in ra (khoá thì sửa cả vân tay đi kèm) rồi chạy lại lệnh trên.

## Snapshot quyền: `policy.apply`

Server → agent, yêu cầu, cần `policy.v1`. Tham số là `SignedEnvelope {payload, sig, key_id}`; kết quả là `Applied {epoch, rev}` của snapshot **đang có hiệu lực sau khi xử lý**, cũng là lời xác nhận.

### Ký trên byte, không ký trên JSON đã phân tích

`payload` là **base64 chuẩn có đệm của đúng các byte JSON** mà server đã ký. Chữ ký phủ lên các byte đó (sau khi giải base64), không phủ lên đối tượng JSON đã phân tích. Hai bên nhờ vậy không phải đồng ý về cách chuẩn hoá JSON (thứ tự khoá, khoảng trắng, cách viết số). Bên nhận **phải kiểm chữ ký trước khi phân tích `payload`**, và phải lưu lại đúng các byte đó (không mã hoá lại từ đối tượng đã phân tích) để kiểm lại chữ ký khi nạp từ đĩa.

Schema diễn đạt ràng buộc này ngay trong `payload` bằng `contentEncoding` và `contentSchema`: payload phải giải ra một `PolicySnapshot`. Ví dụ: [SignedEnvelope.full](examples/sync/SignedEnvelope.full.json). Ca payload không phải snapshot: [SignedEnvelope.payload-khong-phai-snapshot.invalid](examples/sync/SignedEnvelope.payload-khong-phai-snapshot.invalid.json).

### Nội dung snapshot

`PolicySnapshot` là toàn bộ quyền truy cập của **một máy** (một cặp máy và user hệ điều hành):

| Trường | Ý nghĩa |
|---|---|
| `proto` | `1` |
| `machine_id`, `os_user` | máy và user hệ điều hành mà snapshot dành cho |
| `epoch`, `rev` | phiên bản; so theo thứ tự từ điển `(epoch, rev)` |
| `issued_at` | thời điểm ký, chỉ để tham khảo |
| `full` | luôn `true` trong v1 |
| `grants[]` | `{account_id, role, keys[{fp, public_key}]}` |
| `emergency` | `{enabled, account_ids}` |
| `clear_local_revocations[]` | mã các thu hồi cục bộ mà server ký lệnh gỡ |

Quy tắc:

- **`full: true` luôn luôn, và `grants` rỗng là trạng thái hợp lệ nghĩa là không ai được vào**. Không có "không đổi" hay delta ở v1. (Một triển khai từng coi danh sách rỗng là "không có thay đổi" nên người đã bị gỡ vẫn vào được mãi.) `grants` luôn có mặt, kể cả khi rỗng: người gửi phát `[]`, không phát `null`.
- `(epoch, rev)` mới hơn khi `epoch` lớn hơn, hoặc `epoch` bằng nhau và `rev` lớn hơn. Agent từ chối bản không mới hơn bản đang áp dụng. `epoch` tăng khi server khôi phục từ bản sao lưu cũ để `rev` lùi không làm agent kẹt quyền cũ.
- Tính hợp lệ **không phụ thuộc đồng hồ của agent**: chỉ `(epoch, rev)` được dùng. `issued_at` không tham gia kiểm tra.
- `role` là `owner`, `admin` hoặc `member`; giá trị lạ được coi là `member`, không bao giờ nâng quyền.
- `keys[].fp` là vân tay của `keys[].public_key`. Agent nhận khoá đăng nhập bằng cách so vân tay rồi so lại byte khoá.
- Snapshot **không chứa email hay tên hiển thị**: nó đi qua agent mà mọi tài khoản được chia sẻ đều đọc được.

### Thứ tự kiểm của agent

Agent dừng ở lỗi đầu tiên và trả lỗi `-32004` (`conflict`) với `error.data.reason`:

| # | Kiểm | `reason` khi sai |
|---|---|---|
| 1 | `key_id` có trong danh sách khoá ký đang tin | `unknown_key_id` |
| 2 | chữ ký đúng trên byte của `payload`, ngữ cảnh `policy-snapshot/1` | `bad_signature` |
| 3 | `payload` giải ra được `PolicySnapshot` | `bad_payload` |
| 4 | `proto` được hỗ trợ | `bad_proto` |
| 5 | `machine_id` là của máy này; `os_user` là của máy này | `wrong_machine`, `wrong_os_user` |
| 6 | `full` là `true` | `not_full` |
| 7 | `(epoch, rev)` mới hơn bản đang áp dụng | `stale` |

Qua cả bảy bước thì agent ghi snapshot xuống đĩa **nguyên tử**, áp dụng, rồi trả `Applied`. Danh sách `reason` là tập mở: bên gửi coi lý do lạ là "bị từ chối" và hiển thị nguyên văn. Agent ghi lần từ chối gần nhất (`epoch`, `rev`, `reason`, `at`) và báo qua `machine.status.last_reject`.

Server coi **xác nhận** là bằng chứng quyền đã áp dụng. Không có xác nhận trong thời hạn thì máy ở trạng thái `pending` và server thử lại cho tới khi có xác nhận hoặc kết nối bị thay. Lỗi `stale` do gửi lại đúng bản agent đã áp dụng không phải sự cố: server đối chiếu `applied` trong `hello` và `machine.status`.

Ví dụ: [PolicySnapshot.full](examples/sync/PolicySnapshot.full.json), [PolicySnapshot.rong-hop-le](examples/sync/PolicySnapshot.rong-hop-le.json), [PolicySnapshot.khan-cap](examples/sync/PolicySnapshot.khan-cap.json), [ErrorData.stale](examples/rpc/ErrorData.stale.json).

## Xoay khoá ký chính sách: `policy.keys`

Server → agent, yêu cầu, cần `policy.v1`. Tham số `KeyEndorsement`; kết quả `{}`.

Agent ghim sẵn danh sách khoá ký chính sách lúc thêm máy ([EnrollPollResult](examples/http-api/EnrollPollResult.full.json)). Khoá ký mới chỉ được nhận khi có **chứng thực** của một khoá đang tin: `signer_key_id` chọn khoá đang tin, `sig` ký (ngữ cảnh `policy-key-rollover/1`) lên `key_id`, `public_key` và `not_before` của khoá mới. Agent kiểm `signer_key_id` (`unknown_key_id`), kiểm chữ ký (`bad_signature`); thành công thì thêm khoá mới vào danh sách tin, dùng được từ `not_before`. Khoá cũ giữ lại để kiểm cho tới khi người vận hành gỡ.

Khoá ký bị lộ: không có đường tự động; người vận hành gắn lại khoá tin cậy trên từng máy có xác nhận.

Ví dụ: [KeyEndorsement.full](examples/sync/KeyEndorsement.full.json).

## Trạng thái máy: `machine.status`

Agent → server, **notification**, cần `status.v1`. Tham số `MachineStatus`. Gửi khi nội dung đổi (gom trong khoảng 1 giây) và định kỳ khoảng 60 giây như nhịp sống.

- `session_count`, `session_accounts`: số phiên và các tài khoản có phiên. Server dùng `session_accounts` để từ chối bật đăng nhập khẩn cấp khi máy còn tài khoản khác.
- `addresses`: các địa chỉ `host:port` agent lắng nghe. Server **bỏ** địa chỉ nằm ngoài các dải mạng mà chủ tài khoản đã xác nhận lúc thêm máy.
- `applied`, `last_reject`: để server tính trạng thái áp dụng quyền của máy. Nếu `applied` vượt phiên bản trong cơ sở dữ liệu (server vừa khôi phục bản sao lưu cũ), server tăng `epoch` rồi đẩy lại snapshot.
- `local_revocations`: các thu hồi cục bộ còn hạn, để chủ máy thấy và gỡ từ app.

Vì là notification, mất một lần cập nhật không gây hại: lần sau (tối đa khoảng 60 giây) sẽ bù.

Ví dụ: [MachineStatus.full](examples/sync/MachineStatus.full.json), [MachineStatus.toi-thieu](examples/sync/MachineStatus.toi-thieu.json).

## Kiểm toán: `audit.report`

Agent → server, yêu cầu, cần `audit.v1`. Tham số `AuditReport {events[]}`; kết quả `AuditAck {acked[]}`.

- Mỗi `AuditEvent` có `event_id` là 128 bit ngẫu nhiên (32 chữ số hex thường) do agent sinh. Server khử trùng theo `(máy, event_id)` nên gửi lại an toàn (giao ít nhất một lần).
- `acked` liệt kê **mọi** sự kiện server đã xử lý xong, gồm cả sự kiện trùng và sự kiện bị bỏ vì hành động nằm ngoài danh sách server cho phép. Agent chỉ xoá khỏi hàng đợi những sự kiện có mặt trong `acked`; sự kiện chưa được xác nhận không bao giờ bị xoá.
- Vượt hạn mức nhận sự kiện: `-32003` với `retry_after`; agent giữ hàng đợi và gửi lại sau.
- Khi nhiều sự kiện giống nhau được gom (ví dụ nhiều lần đăng nhập sai từ cùng địa chỉ trong một phút) thì `count` lớn hơn 1. `meta` chỉ mang giá trị chuỗi.
- Server coi đây là dữ liệu "do máy báo", không tin tuyệt đối: nó áp danh sách hành động cho phép, hạn mức và giới hạn kích thước theo từng máy.

Ví dụ: [AuditReport.full](examples/sync/AuditReport.full.json), [AuditAck.full](examples/sync/AuditAck.full.json).

## Đăng nhập khẩn cấp: `emergency.verify`

Agent → server, yêu cầu, cần `emergency.v1` ở cả hai bên: server chỉ trả lời agent đã quảng bá nó trong `hello` (agent chưa quảng bá nhận `-32601`), và agent chỉ gọi khi server cũng quảng bá. Tham số `EmergencyVerifyParams {email, emergency_secret, totp, os_user, remote_ip}`; kết quả `EmergencyVerifyResult {account_id}`.

- Dùng khi ai đó đăng nhập SSH vào máy đang bật chế độ khẩn cấp bằng `keyboard-interactive` (email, bí mật khẩn cấp, mã TOTP) mà không có khoá công khai hợp lệ.
- **Không có trường mật khẩu tài khoản, và schema cấm trường `password`** (có ví dụ không hợp lệ chứng minh điều đó). Agent là tiến trình mà mọi tài khoản được chia sẻ cùng user hệ điều hành đều thay được, nên nó chỉ được thấy **bí mật khẩn cấp** dùng riêng cho đúng cặp (tài khoản, máy) này. Bí mật đó không đăng nhập được app hay API.
- Chỉ nhận mã TOTP, không nhận mã khôi phục.
- Mọi thất bại (email lạ, bí mật sai, TOTP sai, máy chưa bật khẩn cấp, tài khoản không có quyền) trả `-32001` y hệt nhau để không thành công cụ dò. Hạn mức theo (tài khoản, nguồn, máy): vượt thì `-32003` với `retry_after`.
- Thành công trả `account_id`; agent phải đối chiếu với `emergency.account_ids` của snapshot đang áp dụng trước khi cho vào. Agent không liên lạc được server thì từ chối ngay, không có đường kiểm cục bộ.
- Bí mật, mã TOTP và email không được ghi vào log hay sự kiện kiểm toán, ở cả hai phía.

Ví dụ: [EmergencyVerifyParams.full](examples/sync/EmergencyVerifyParams.full.json), [EmergencyVerifyParams.co-mat-khau-tai-khoan.invalid](examples/sync/EmergencyVerifyParams.co-mat-khau-tai-khoan.invalid.json), [EmergencyVerifyResult.full](examples/sync/EmergencyVerifyResult.full.json).

## Tóm tắt lỗi

| Tình huống | Mã | `reason` |
|---|---|---|
| Chưa xác thực / xác thực thất bại | `-32001` | không nêu |
| Snapshot hoặc chứng thực bị agent từ chối | `-32004` | `stale`, `bad_signature`, `unknown_key_id`, `bad_payload`, `bad_proto`, `wrong_machine`, `wrong_os_user`, `not_full` |
| Vượt hạn mức | `-32003` | `retry_after` (giây) |
| Method không có (capability không quảng bá) | `-32601` | |

## Tương thích

Bên nào cũng chỉ gọi method của bên kia khi bên kia đã quảng bá capability; method lạ trả `-32601` và coi là không hỗ trợ. Server cũ với agent mới: agent không gửi `machine.status` hay `audit.report` nếu server không quảng bá `status.v1`/`audit.v1`, và vẫn chạy với quyền từ snapshot. Agent cũ với server mới: server không gọi method chưa được quảng bá.
