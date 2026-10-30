# RPC của agent và kênh terminal

> **Bản nháp v0.** Xem [README](README.md) cho quy tắc version, capability và registry. Schema: [rpc.schema.json](schema/rpc.schema.json). Ví dụ: [examples/rpc](examples/rpc).

Agent phục vụ hai thứ trên cùng một SSH server, chỉ sau khi client đã xác thực bằng khoá công khai:

1. **Kênh terminal**: kênh SSH `session` thông thường (`pty-req`, `env`, `shell`). Mọi client SSH (OpenSSH, Termius, Blink) dùng được.
2. **RPC**: subsystem SSH `agent-rpc`, JSON-RPC 2.0 để liệt kê, tạo, đóng phiên và nhận sự kiện. Dành cho app.

Kết nối SSH chỉ cho hai thứ này. Agent từ chối subsystem `sftp`, kênh `direct-tcpip` và `x11`, yêu cầu `tcpip-forward` và `auth-agent-req@openssh.com`.

## Vận chuyển của RPC

- Mỗi kênh subsystem `agent-rpc` là một kết nối JSON-RPC hai chiều riêng. Một kết nối SSH có thể mở nhiều kênh.
- **NDJSON**: mỗi thông điệp là một dòng JSON UTF-8 kết thúc bằng `\n`, không chứa ký tự xuống dòng bên trong. Dòng trống bị bỏ qua.
- **Giới hạn kích thước** một thông điệp lấy từ cấu hình giới hạn của agent (mặc định 1 MiB). Thông điệp vượt giới hạn làm đóng kết nối: không bên nào được gửi vượt giới hạn của bên kia.
- JSON-RPC 2.0, không batch. `id` của yêu cầu là số hoặc chuỗi và được trả nguyên văn. Yêu cầu chạy đồng thời; notification được xử lý tuần tự theo thứ tự đến nhưng là best-effort.
- Lỗi dùng mã chuẩn và các mã riêng `-32000`…`-32004` ([registry](README.md#mã-lỗi-json-rpc-riêng)); `error.data` của `-32001`…`-32004` là `ErrorData`.

## Bắt tay: `hello`

`hello` phải là yêu cầu đầu tiên. Trước khi `hello` thành công, mọi method đã biết khác trả `-32600`; method không tồn tại luôn nhận `-32601` ([quy tắc 4](README.md#quy-tắc-version-và-capability)).

| | |
|---|---|
| Tham số | `HelloParams`: `proto`, `client {name, version}`, `capabilities` |
| Kết quả | `HelloResult`: `proto`, `agent {name, version}`, `machine_id`, `os_user`, `capabilities` |
| Lỗi | `-32602` nếu `proto` không được hỗ trợ |

- `proto` hiện là `1`. Đổi `proto` bị cấm trong v1, nên lỗi này chỉ xảy ra với client rất lạ.
- `capabilities` là danh sách capability mà mỗi bên hiểu. Client chỉ gọi method của nhóm mà agent đã quảng bá; agent chỉ gửi notification `event` cho client đã quảng bá `events.v1` **và** đã `events.subscribe`.
- `machine_id` rỗng khi agent chưa được thêm vào server nào. `os_user` là tên đăng nhập SSH phải dùng (user hệ điều hành chạy agent); Windows không phân biệt hoa thường.

Ví dụ: [HelloParams.full](examples/rpc/HelloParams.full.json), [HelloResult.full](examples/rpc/HelloResult.full.json), [HelloResult.chua-enroll](examples/rpc/HelloResult.chua-enroll.json).

## Method

Phạm vi theo vai trò (lấy từ snapshot quyền, không phải từ client): `owner` và `admin` thấy và đóng được mọi phiên trên máy; `member` chỉ thấy phiên của chính mình. Phạm vi này để giữ riêng tư giữa các tài khoản, **không phải ranh giới bảo mật**: mọi tài khoản được chia sẻ chạy lệnh dưới cùng user hệ điều hành.

| Method | Capability | Tham số | Kết quả | Lỗi |
|---|---|---|---|---|
| `session.list` | `session.v1` | không có, hoặc `{}` | `SessionListResult` | |
| `session.create` | `session.v1` | `SessionCreateParams` (`cols`, `rows`) | `SessionInfo` | `-32003` khi chạm trần số phiên |
| `session.close` | `session.v1` | `SessionCloseParams` (`session_id`) | `{}` | `-32002` khi phiên không có hoặc ngoài phạm vi |
| `session.size` | `session.v1` | `SessionSizeParams` (`session_id`, `take`) | `SizeInfo` | `-32002` như trên |
| `events.subscribe` | `events.v1` | `EventsSubscribeParams` (`topics`) | `EventsSubscribeResult` | |

- `session.create` tạo phiên mới thuộc tài khoản của người gọi; để vào phiên đó, mở kênh terminal với biến phiên bằng mã trả về.
- `session.close` đóng cả cây tiến trình của phiên. `-32002` dùng cho cả "không có" lẫn "ngoài phạm vi" để không lộ sự tồn tại của phiên người khác.
- `session.size` đọc kích thước PTY và client đang giữ quyền chủ cỡ. `take: true` đòi quyền chủ cỡ cho kết nối đang gọi (xem [Kích thước](#kích-thước-và-chủ-cỡ)).
- `events.subscribe` nhận mẫu chủ đề: tên đầy đủ hoặc `<tiền-tố>.*`. Kết quả liệt kê các mẫu agent chấp nhận; mẫu agent không biết bị bỏ qua chứ không báo lỗi.

`SessionInfo.durable` là `false` khi môi trường không bảo đảm phiên sống tiếp qua lúc đăng xuất hoặc khởi động lại daemon; app nên cảnh báo người dùng.

Ví dụ: [SessionInfo.full](examples/rpc/SessionInfo.full.json), [SessionListResult.full](examples/rpc/SessionListResult.full.json), [SizeInfo.co-chu](examples/rpc/SizeInfo.co-chu.json), [SessionSizeParams.take](examples/rpc/SessionSizeParams.take.json).

## Sự kiện

Sau `events.subscribe`, agent gửi notification `event` với tham số `Event` `{topic, ts, data}`. `data` luôn là đối tượng JSON; chủ đề lạ phải được bỏ qua.

| Chủ đề | `data` | Phát khi |
|---|---|---|
| `session.created` | `SessionInfo` | phiên mới được tạo |
| `session.closed` | `SessionInfo` (trạng thái cuối) | phiên bị đóng hoặc shell thoát |
| `session.attached` | `SessionInfo` | một client attach |
| `session.detached` | `SessionInfo` | một client detach |
| `session.resized` | `SessionResizedData` `{id, cols, rows, owner_client}` | kích thước PTY đổi |

Sự kiện chỉ gồm phiên nằm trong phạm vi của người đăng ký. Vì notification là best-effort, client coi sự kiện là gợi ý làm mới và đối chiếu lại bằng `session.list` khi cần (sau khi nối lại luôn phải làm).

Ví dụ: [Event.session-created](examples/rpc/Event.session-created.json), [Event.session-resized](examples/rpc/Event.session-resized.json), [Event.chu-de-la](examples/rpc/Event.chu-de-la.json).

## Kênh terminal

Đây là kênh SSH `session` bình thường nên không có schema; hành vi mô tả ở đây.

### Trình tự

```
client                                   agent
  │── pty-req (term, cols, rows, …) ───────►│
  │── env  RTERM_SESSION=<mã | new>  ──────►│   (không bắt buộc; có thể nhiều env)
  │── shell ───────────────────────────────►│   chọn / tạo / vào lại phiên
  │◄──────────── snapshot màn hình ──────────│   khung đầu tiên: ANSI dựng lại màn hình
  │◄═══════════ output trực tiếp ═══════════►│   input ↔ output hai chiều
  │── window-change (cols, rows) ──────────►│   đề xuất kích thước
  │◄──────────── exit-status ────────────────│   khi shell thoát hoặc không vào được
```

- `env` chỉ nhận `LANG`, `LC_*` và mọi tên trong `SessionEnvVars` (hiện là `RTERM_SESSION`, [registry](README.md#kênh-terminal-ssh)); tên khác bị bỏ qua. Với `ssh` của OpenSSH: `ssh -o SetEnv=RTERM_SESSION=<mã> -p <cổng> <user>@<máy>`.
- `shell` có `pty-req` là phiên terminal bền (attach vào holder). `shell` không có pty, và `exec`, chạy lệnh không bền qua pipe và không dùng biến phiên.

### Chọn phiên

| Biến phiên | Hành vi |
|---|---|
| `<mã>` | vào lại phiên có mã đó, nếu thuộc tài khoản này. Không còn thì xem [Mã thoát](#mã-thoát-và-phiên-không-còn) |
| `new` | luôn tạo phiên mới |
| vắng | vào lại **phiên chưa có client có hoạt động gần nhất của tài khoản**; không có thì tạo mới (giống `tmux new -A`) |

Khi vắng biến, chọn phiên mới nhất mà không còn client nào attach để SSH thông thường vẫn giữ được phiên mà không tích phiên rác. Chạm trần số phiên của tài khoản mà không có phiên nào để vào lại thì agent từ chối: ghi `limit_exceeded` ra stderr và thoát với mã khác 0. Không có TTL tự đóng phiên theo mặc định.

### Dữ liệu: snapshot rồi output

- Byte đầu tiên client nhận sau `shell` là **snapshot**: chuỗi ANSI mở đầu bằng reset rồi dựng lại màn hình (scrollback, màn hiện tại, con trỏ, các mode đã bật), có thể gồm nhiều khung. Client xử lý nó như mọi output khác.
- Sau snapshot là output trực tiếp, liền mạch: không mất và không lặp byte so với lúc chụp.
- **Client chậm không bị ngắt.** Khi hàng đợi của client tràn, agent bỏ phần đã dồn và gửi một snapshot mới (lại mở đầu bằng reset) trên cùng kết nối. Client không cần xử lý gì đặc biệt vì reset xoá màn hình rồi dựng lại.
- Snapshot bị cắt theo ngân sách thì dòng cũ nhất bị bỏ và có một dòng ghi chú ở đầu.
- Truy vấn terminal (DA, CPR, báo màu…) do agent trả lời trong một danh sách cho phép cố định; client không phải trả lời, và phản hồi dạng report do client gửi bị lọc khỏi input.

### Kích thước và chủ cỡ

Một phiên chỉ có một cỡ PTY. Nhiều client cùng attach nên cần luật để cỡ không nhảy qua lại:

- Có đúng một **chủ cỡ**. `window-change` của chủ cỡ đổi cỡ PTY. Của client khác chỉ được coi là đề xuất, và có hiệu lực khi chủ cỡ im quá ngưỡng, hoặc client đó đòi cỡ bằng `session.size {take: true}`. Giữa hai lần đổi chủ có độ trễ tối thiểu. Attach không giành quyền chủ cỡ trừ khi phiên chưa có client.
- Mỗi lần cỡ PTY đổi, agent phát `session.resized` (nếu client đã đăng ký) để client không phải chủ cỡ giữ lưới hiển thị theo cỡ PTY thay vì tự suy từ màn hình của mình.
- `owner_client` là mã không trong suốt, ổn định trong lúc client giữ cỡ. Cách một client nhận ra mã của chính mình sẽ chốt khi hiện thực kênh attach; hợp đồng hiện chỉ bảo đảm `owner_client` đổi khi và chỉ khi chủ cỡ đổi.

### Mã thoát và phiên không còn

Khi shell thoát, agent gửi `exit-status` với mã thoát của shell rồi đóng kênh.

Khi biến phiên chỉ mã không còn tồn tại (đã đóng, hoặc chưa từng có), agent **không tạo phiên mới thầm lặng**. Nó ghi dòng `session_not_found` ra stderr, gửi `exit-status` = `ExitSessionNotFound` rồi đóng kênh. Nhờ vậy app biết phiên đã mất và hỏi người dùng trước khi mở phiên khác.

**Quyết định: `ExitSessionNotFound` = `66`.**

- 66 là `EX_NOINPUT` trong `sysexits.h` ("thứ được yêu cầu làm đầu vào không có"), đúng nghĩa: phiên được yêu cầu không có.
- Các mã sau bị loại vì shell và công cụ đã dùng: `0` (thành công), `1` (lỗi chung), `2` (dùng sai), `126` (không thực thi được), `127` (không tìm thấy lệnh), `128+n` (chết vì tín hiệu `n`) và `255` (mã `ssh` tự trả khi chính nó lỗi). `66` nằm ngoài tất cả.
- Shell của người dùng vẫn có thể tự `exit 66`. Vì vậy client chỉ kết luận "phiên đã mất" khi thấy **cả** mã `66` **lẫn** dòng stderr `session_not_found`, và khi chưa nhận byte stdout nào.
- OpenSSH trả nguyên mã này cho lệnh `ssh`, nên script cũng bắt được (`ssh … ; [ $? -eq 66 ]`).

Ví dụ phiên không còn, nhìn từ OpenSSH:

```
$ ssh -tt -o SetEnv=RTERM_SESSION=k3x9mqz7ab -p 7022 deploy@100.64.0.12
session_not_found
Connection to 100.64.0.12 closed.
$ echo $?
66
```

## Tương thích và mở rộng

- **Thêm method không đổi giao thức gốc.** Ví dụ thêm đo CPU: capability `metrics.v1`, method `metrics.cpu`. Client cũ không biết capability nên không gọi; client mới gọi agent cũ nhận `-32601` và coi là không hỗ trợ.
- Trường lạ trong tham số và kết quả bị bỏ qua; vì vậy thêm trường vào thông điệp đã có không cần capability, miễn trường cũ không đổi nghĩa.
- Agent cũ và app mới: app đọc `capabilities` từ `hello` và ẩn tính năng mà agent không có (như nút chủ cỡ khi thiếu `session.v1`).
