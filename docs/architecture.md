# rterm — Kiến trúc

**Cập nhật:** 30/10/2026 · Trạng thái: thiết kế; phần hiện thực xem [cuối trang](#trạng-thái-hiện-thực).

## Mục tiêu

Người dùng tự host một server tài khoản và cài agent lên các máy Ubuntu, macOS, Windows. Từ app (Android trước) hoặc SSH client bất kỳ, đăng nhập tài khoản là thấy mọi máy mình có quyền, mở terminal và thao tác như ngồi trước máy. Phiên không mất khi rớt mạng hay tắt app. Chia sẻ được máy cho tài khoản hoặc nhóm khác. Đây là nền cho các tính năng sau, nên giao thức và API có version và capability từ ngày đầu.

Ràng buộc:

- Tự host toàn bộ (server, agent, app); không có dịch vụ trung tâm của bên thứ ba.
- Dữ liệu terminal đi thẳng app ↔ agent qua VPN có sẵn của người dùng; **không đi qua server**. Server chỉ lo danh tính và quyền.
- Không phụ thuộc API riêng của VPN nào. Vào được VPN chưa có nghĩa là có quyền.
- Agent là Go: một tệp chạy, `CGO_ENABLED=0`. Server tài khoản là Python 3.14 (thư mục `server/`, FastAPI + Uvicorn, SQLite, phát hành dạng image Docker). App là Flutter.
- Chỉ dùng thư viện giấy phép dễ dãi.

Không làm ở v1: tính năng riêng cho AI agent, chạy lệnh trên nhiều máy, relay hay xuyên NAT, tự viết terminal emulator, chia pane, remote desktop, quản lý file, xem phiên chỉ đọc, agent chạy root/SYSTEM, bản SaaS, app iOS hay desktop, mở khoá bằng sinh trắc học. Chỉ chuẩn bị điểm cắm, không làm sẵn tính năng.

## Tổng quan

```
                  HTTPS: đăng nhập, danh sách máy + host key, chia sẻ
App Flutter ─────────────────────────────────────────► Server tài khoản (tự host)
OpenSSH / Termius / Blink                               Python (FastAPI, SQLite)
   │ SSH (khoá công khai) qua VPN, không qua server        ▲
   ▼                                                       │ agent giữ một kết nối ra:
Daemon agent (Go, chạy như service) ───────────────────────┘ nhận snapshot quyền, báo trạng thái,
   │ IPC cục bộ (Unix socket / named pipe)                   hỏi xác minh khẩn cấp
   ▼
Holder × N: PTY / ConPTY + trạng thái màn hình, sống qua lúc daemon khởi động lại hay cập nhật
```

Hai đường tách bạch: **đường điều khiển** (HTTPS tới server, WebSocket từ agent tới server) mang danh tính và quyền; **đường dữ liệu** (SSH từ app hoặc client SSH thẳng tới agent) mang terminal. Server sập thì đường dữ liệu vẫn chạy bằng bản sao quyền mà agent giữ.

## Thành phần

### Server tài khoản

Viết bằng Python 3.14, ở thư mục `server/`, quản lý phụ thuộc bằng uv. Stack: FastAPI + Uvicorn một worker (hub WebSocket và hạn mức nằm trong bộ nhớ), `websockets`, Pydantic v2, SQLite qua `sqlite3` với một luồng ghi, `cryptography` cho TLS và chữ ký, argon2-cffi cho mật khẩu, pyotp cho TOTP. Phát hành dạng image Docker. Khi chạy trực tiếp, server chỉ lắng nghe trên địa chỉ VPN hoặc localhost; ra Internet phải qua reverse proxy, vì Uvicorn không giới hạn thời gian đọc header. Các mục dưới đây là yêu cầu về hành vi, không phụ thuộc cách hiện thực.

- **Tài khoản:** email, mật khẩu (argon2id), TOTP bắt buộc, mã khôi phục dùng một lần. Bước đầu của đăng nhập đi qua một điểm cắm nhà cung cấp danh tính để thêm OIDC sau mà không đổi luồng TOTP. Token mờ: access ngắn hạn, refresh xoay vòng dùng một lần.
- **Máy:** thêm máy bằng mã giống đăng nhập trên TV (RFC 8628) nhưng cấp danh tính máy, không cấp token người dùng. Server lưu host key từng máy (duy nhất) để app kiểm host key thay vì hỏi người dùng.
- **Khoá:** khoá thiết bị do app tạo trong kho khoá của Android, kèm khoá SSH dán tay; server đẩy khoá tới các agent có liên quan qua snapshot quyền.
- **Quyền:** deny-by-default, kiểm ở một điểm. Nhóm, vai trò `owner`/`admin`/`member`, chia sẻ cho tài khoản hoặc nhóm có từ thiết kế; quyền gắn với cặp (máy, user hệ điều hành).
- **Kiểm toán:** bảng chỉ thêm, ghi cùng giao dịch với thay đổi và phát JSON ra stdout; sự kiện nổi bật hiện thành cảnh báo trong app, không phụ thuộc SMTP.
- **Bí mật** (khoá ký chính sách, khoá mã hoá TOTP, khoá TLS) nằm ở thư mục riêng tách khỏi dữ liệu; server từ chối khởi động nếu cơ sở dữ liệu có dữ liệu mà bí mật thiếu hoặc lệch, thay vì sinh lại im lặng.
- **TLS:** mặc định tự ký, app và agent ghim SPKI; chạy sau CA hợp lệ thì không ghim.

### Daemon agent

Một tiến trình service chạy dưới user hệ điều hành thường, không root.

- **SSH server nhúng:** chỉ kênh `session` và subsystem `agent-rpc`; từ chối SFTP, chuyển tiếp cổng, chuyển tiếp agent, X11. Thuật toán khai báo tường minh, không dùng mặc định của thư viện. Xác thực bằng khoá công khai tra trong bản sao quyền.
- **Chỉ lắng nghe giao diện VPN và loopback.** Chọn theo tên giao diện **và** dải địa chỉ, không theo dải riêng: `100.64.0.0/10` còn là dải CGNAT của nhà mạng, nên địa chỉ trong dải đó trên giao diện không nhận ra là VPN thì không bind. Quét lại giao diện định kỳ để lắng nghe lại khi VPN lên muộn hoặc máy ngủ rồi thức. Không bao giờ wildcard trừ khi cấu hình tường minh.
- **Bề mặt trước xác thực hẹp:** hạn chót bắt tay, hạn mức kết nối chưa xác thực theo IP và toàn cục, tách khỏi hạn mức kết nối đã xác thực.
- **Đồng bộ:** giữ một kết nối đi ra tới server để nhận snapshot quyền và báo trạng thái. Xem [Xác thực và quyền](#xác-thực-và-quyền).
- **RPC và sự kiện:** `agent-rpc` cho app liệt kê, tạo, đóng phiên và nhận sự kiện.

### Holder

Mỗi phiên là một tiến trình riêng (kiểu dtach/abduco): giữ PTY (Unix) hay ConPTY (Windows) và trạng thái màn hình, daemon chỉ chuyển tiếp. Holder sống qua lúc daemon khởi động lại hay cập nhật nên phiên không mất.

- **Phiên mới attach:** holder gửi snapshot đủ trạng thái (màn hình, scrollback, con trỏ, các mode) rồi output trực tiếp, không mất và không lặp byte. Client chậm không bị ngắt mà nhận snapshot mới.
- **Trạng thái màn hình không nằm trên đường byte:** lỗi của emulator làm phiên chuyển sang chế độ suy giảm, PTY và client vẫn sống.
- **Nhiều client cùng phiên:** có một chủ cỡ; chủ cỡ chỉ chuyển khi chủ hiện tại im đủ lâu hoặc có yêu cầu lấy cỡ, tránh cỡ nhảy qua lại.
- **Độ bền** tính lúc tạo phiên và hiện ra cho người dùng, không hạ thầm lặng: Linux dùng `systemd --user` với `KillMode=process` và bắt buộc linger; macOS dùng LaunchDaemon (có sudo, chạy trước đăng nhập) hoặc LaunchAgent (mất phiên khi đăng xuất GUI, có cảnh báo); Windows dùng Service chạy bằng tài khoản user hoặc Task lúc đăng nhập, holder tách khỏi job của service.

### App Flutter

Tầng: `transport` (SSH) → `api_client` (server) và `agent_client` (RPC) → `domain` → `features` → UI.

- Khoá SSH nằm trong Android Keystore, ký qua `SSHIdentity` tuỳ biến; host key kiểm theo danh sách từ server.
- Terminal bọc emulator sau một interface; hàng phím phụ; ô nhập riêng cho tiếng Việt; tự nối lại đúng phiên.
- **Server tắt không khoá người dùng khỏi máy:** app giữ bản sao danh sách máy trong kho an toàn và nối máy bằng bản sao đó.

## Giao thức

Ba giao thức, đều có `proto` và capability, mô tả tại [protocol/](protocol/README.md):

| Giao thức | Vận chuyển | Tài liệu |
|---|---|---|
| RPC agent | JSON-RPC 2.0 trên subsystem SSH `agent-rpc` (NDJSON) | [rpc.md](protocol/rpc.md) |
| Đồng bộ agent–server | JSON-RPC 2.0 trên WebSocket `GET /v1/agent/sync` | [sync.md](protocol/sync.md) |
| HTTP API | REST/JSON dưới `/v1` | [http-api.md](protocol/http-api.md) |

Kênh terminal là SSH `session` chuẩn nên mọi client SSH dùng được. Hợp đồng không gắn với ngôn ngữ nào: mỗi thành phần có bản cài đặt riêng, cùng được kiểm bằng bộ ví dụ trong `protocol/examples`. Phía agent (Go): `internal/jsonrpc` là thư viện JSON-RPC hai chiều dùng chung cho RPC agent và đồng bộ (vận chuyển nằm sau interface `Conn`), còn `internal/protocol` là bản Go của hợp đồng.

## Xác thực và quyền

| Luồng | Cách chạy |
|---|---|
| Đăng nhập app | email + mật khẩu + TOTP → app tạo khoá trong kho khoá thiết bị → đăng ký vào tài khoản bằng chứng minh sở hữu |
| Thêm máy | `rterm enroll` hiện mã → duyệt trong app (đối chiếu vân tay host key) → server lưu máy + host key |
| Khoá SSH dán tay | thêm vào tài khoản; được đẩy tới mọi máy tài khoản có quyền |
| Chia sẻ, thu hồi | chia sẻ máy cho tài khoản hoặc nhóm với vai trò; agent online nhận thay đổi trong vài giây, agent offline nhận khi nối lại |
| Khẩn cấp | bí mật khẩn cấp riêng cho (tài khoản, máy) + TOTP qua `keyboard-interactive`; tắt mặc định từng máy; không liên lạc được server thì từ chối |
| Server sập | agent dùng bản sao quyền: phiên và đăng nhập bằng khoá đã biết vẫn chạy; chỉ dừng đăng nhập mới bằng khoá chưa biết, thay đổi quyền, khẩn cấp |

**Snapshot quyền.** Server dựng và ký snapshot đầy đủ cho từng máy (Ed25519, ký trên byte chứ không trên JSON đã phân tích), có `(epoch, rev)` tăng đơn điệu. Agent từ chối bản không mới hơn, kiểm chữ ký cả lúc nhận lẫn lúc nạp từ đĩa, và xác nhận từng bản đã áp dụng để server biết máy nào còn chậm. Snapshot rỗng là "không ai được vào", không phải "không đổi". Khoá ký xoay được qua chứng thực của khoá đang tin.

**Thu hồi.** Với máy online, thu hồi có hiệu lực xấp xỉ một vòng RTT: agent ngắt kết nối SSH của khoá bị thu hồi, và khi tài khoản không còn quyền nào trên máy thì đóng cả các phiên của tài khoản đó. Thu hồi một khoá khi tài khoản còn quyền chỉ ngắt kết nối dùng khoá đó. Chủ máy còn thu hồi cục bộ được ngay trên máy, có hạn, thắng snapshot của server cho tới khi hết hạn.

**Server mất liên lạc.** Agent đếm từ lần đồng bộ thành công cuối: quá `max_stale` (mặc định 7 ngày, cấu hình được theo máy) thì từ chối mọi đăng nhập mới, còn kết nối và phiên đang mở vẫn chạy. Cảnh báo khi kết nối ra server lỗi quá một giờ.

**Đã thêm máy thì không quay về chế độ cục bộ.** Máy chưa từng thêm vào server chạy với file `authorized_keys` cục bộ; đã thêm rồi thì mất cấu hình server là đóng cửa (không lắng nghe SSH) thay vì lùi về khoá cục bộ. Thoát chế độ đã thêm chỉ qua lệnh gỡ có xác nhận.

**Member ≈ chủ user hệ điều hành.** Người được chia sẻ chạy lệnh dưới đúng user hệ điều hành đã cài agent, nên đọc ghi được trạng thái của agent và gọi được socket điều khiển cục bộ. Vai trò `member` chỉ là phân quyền phía server, không phải ranh giới bảo mật trên máy. Muốn hạn chế thì cài agent dưới user hệ điều hành riêng và chia sẻ cặp (máy, user) đó; ứng dụng báo rõ điều này khi chia sẻ.

## Quyết định khó đảo ngược

1. SSH làm nền; RPC và API có version và capability → thêm tính năng không phá bản cũ.
2. Holder tách khỏi daemon → tự cập nhật không làm mất phiên.
3. Server là nguồn sự thật về danh tính và quyền nhưng không nằm trên đường dữ liệu; agent giữ bản sao quyền.
4. Mọi credential quy về khoá công khai gắn với tài khoản; quyền gắn với (máy, user hệ điều hành); nhóm và vai trò có từ đầu.
5. Server lưu host key từng máy → app kiểm host key, không hỏi "có tin máy này không".
6. Chỉ phụ thuộc giấy phép dễ dãi → giữ mọi lựa chọn giấy phép cho sản phẩm.
7. Định danh trên dây tách khỏi tên sản phẩm → đổi tên không làm vỡ client, agent và chữ ký đã phát hành.

## Điểm mở rộng

Chỉ chuẩn bị điểm cắm, không làm sẵn tính năng:

| Tính năng sau này | Cắm vào đâu |
|---|---|
| Truyền file, chuyển tiếp cổng | subsystem SFTP, `direct-tcpip` của SSH; bật theo vai trò |
| Thông báo (lệnh xong, bell) | bus sự kiện và `events.subscribe` |
| Xem phiên chỉ đọc | vai trò + attach chỉ đọc ở agent |
| Theo dõi CPU/RAM/đĩa | RPC `metrics.*` qua capability mới |
| Đăng nhập OIDC (Google, GitHub, Authentik, Keycloak) | điểm cắm nhà cung cấp danh tính ở server |
| PostgreSQL | engine thứ hai cho lớp lưu trữ, SQL theo [quy ước di động](code-standards.md#sql) |
| Relay cho người không có VPN | transport thứ hai; host key vẫn kiểm theo server |
| Ghi và phát lại phiên | bus sự kiện + kiểm toán |
| App iOS, desktop | cùng codebase Flutter, UI tách khỏi domain và transport |

## Trạng thái hiện thực

Đã có:

- hợp đồng giao thức (`docs/protocol/`);
- phần Go dùng chung của agent (`internal/protocol`, `internal/jsonrpc`, `internal/product`, lệnh `rterm version`);
- khung server Python (`server/`: model Pydantic của hợp đồng miền sync và HTTP, lệnh `rterm-server version`);
- khung app Flutter (`app/`: model Dart của hợp đồng miền RPC và thân lỗi HTTP `ErrorResponse`, chuỗi giao diện qua ARB);
- conformance test ở cả ba ngôn ngữ, cổng giấy phép cho Go, Dart, PyPI và CI cho từng thành phần.

Chưa có tính năng nào của agent, server hay app. Phần còn lại của kiến trúc ở trên là thiết kế chưa hiện thực. Cập nhật mục này khi có thành phần mới.
