# rterm

Nền tảng remote terminal tự host: một agent Go giữ phiên terminal bền và nói SSH qua VPN có sẵn của bạn, một server tài khoản tự host (Python) lo danh tính và quyền, và app Flutter (Android trước) để mở terminal từ điện thoại. `rterm` là tên tạm; sẽ đổi trước khi phát hành.

- **Phiên không mất** khi rớt mạng, đổi Wi-Fi sang 4G, tắt app hay cập nhật agent.
- **Windows là công dân hạng nhất:** không cần cài OpenSSH Server.
- **Một tài khoản thấy mọi máy được phép**, chia sẻ được cho tài khoản hoặc nhóm khác, thu hồi có hiệu lực trong vài giây.
- **Tự host hoàn toàn.** Dữ liệu terminal đi thẳng giữa app và agent qua VPN, không qua server.
- **Mở:** dùng được cả với OpenSSH, Termius, Blink bằng khoá công khai dán vào tài khoản.

> **Trạng thái: bản nháp, đang phát triển.** Đã có hợp đồng giao thức (bản nháp v0, không gắn với ngôn ngữ nào), phần Go dùng chung của agent (kiểu Go của hợp đồng, thư viện JSON-RPC hai chiều, gói tên sản phẩm), khung server Python và khung app Flutter, cùng conformance test ở cả ba ngôn ngữ. Chưa có tính năng nào cho người dùng: lệnh `rterm` và `rterm-server` mới chỉ có `version`. Chưa có giấy phép được chọn, nên mặc định giữ mọi quyền.

## Bố cục

```
cmd/rterm/           agent và CLI (Go)
internal/product/    hằng tên sản phẩm cho mã Go
internal/protocol/   kiểu Go của hợp đồng, định danh trên dây, test conformance
internal/jsonrpc/    JSON-RPC 2.0 hai chiều (NDJSON, thông điệp)
server/              server tài khoản (Python 3.14, uv, FastAPI; hiện mới có khung)
app/                 app Android (Flutter; hiện mới có khung)
product.json         định danh sản phẩm trung lập; Go, Python, Dart có test chéo so với tệp này
docs/protocol/       hợp đồng giao thức: nguồn sự thật cho mọi ngôn ngữ (schema, ví dụ)
docs/                kiến trúc, quy ước mã nguồn
scripts/             cổng giấy phép cho Go, Dart, PyPI và chính sách dùng chung
testdata/            tài sản test dùng chung giữa các thành phần
spikes/              thí nghiệm kỹ thuật, module Go riêng, ngoài build chính
.github/workflows/   CI: go.yml (agent), server.yml (server), app.yml (app)
```

## Build và test

### Agent (Go 1.26 trở lên)

`go.mod` ghim `toolchain go1.26.10`, bản vá mới nhất của Go 1.26 tại thời điểm viết. Máy có Go cũ hơn sẽ tự tải bản này; máy không có mạng thì đặt `GOTOOLCHAIN=local`.

```sh
go build ./...                 # build mọi gói
go test -race ./...            # test (bỏ -race trên Windows)
go vet ./...
gofmt -l .                     # phải in ra rỗng
golangci-lint run ./...

# Bản chạy: gán version lúc build
go build -ldflags "-X github.com/tuanldas/rterm/internal/product.Version=0.1.0" -o rterm ./cmd/rterm
./rterm version

# Cross-build không cgo
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build ./...
```

### Server (Python 3.14, quản lý bằng uv)

```sh
cd server
uv sync --frozen               # cài đúng phiên bản trong uv.lock
uv run ruff check --config pyproject.toml . ../scripts/check_python_licenses.py
uv run ruff format --check --config pyproject.toml . ../scripts/check_python_licenses.py
uv run mypy                    # strict trên src/ và cổng giấy phép PyPI
uv run pytest
uv run rterm-server version
```

Server phát hành dạng image Docker. Khi chạy trực tiếp, chỉ cho lắng nghe trên địa chỉ VPN hoặc localhost; muốn mở ra Internet thì đặt sau reverse proxy (ví dụ Caddy), vì Uvicorn không giới hạn thời gian đọc header.

### App (Flutter 3.47, Android)

```sh
cd app
flutter pub get
dart format --output=none --set-exit-if-changed .
flutter analyze
flutter test
flutter build apk --debug
```

### Cổng giấy phép

Ba cổng cùng đọc `scripts/license_policy.txt`. Chúng chỉ nhận giấy phép dễ dãi và cấm GPL/AGPL. Giấy phép khác, hoặc gói mà cổng không nhận ra được giấy phép, thì đỏ để người dùng quyết. Người dùng đồng ý thì ghi một dòng ngoại lệ đúng gói, đúng phạm vi; ba cổng áp ngoại lệ theo cùng quy tắc ghi ở đầu tệp chính sách. Mục bị cấm luôn thắng ngoại lệ.

```sh
go run ./scripts                                       # Go
dart run scripts/check_dart_licenses.dart              # Dart (sau flutter pub get trong app/)

# PyPI: mỗi lượt chạy trong đúng môi trường của nó. `uv run` mặc định cài lại cả nhóm dev,
# nên lượt runtime phải cài riêng nhóm app rồi chạy với --no-sync.
cd server
uv sync --frozen --no-default-groups --group app
uv run --no-sync python ../scripts/check_python_licenses.py --scope runtime
uv sync --frozen                                       # trả lại môi trường đủ nhóm dev
uv run --no-sync python ../scripts/check_python_licenses.py --scope dev
```

### Một vài test cần lưu ý

- Mỗi ngôn ngữ chạy conformance trên cùng bộ `docs/protocol/examples`. Go và Python kiểm bằng JSON Schema: ví dụ hợp lệ qua schema, ví dụ `.invalid.json` bị từ chối. Dart kiểm bằng model của app: đọc được ví dụ hợp lệ, từ chối ví dụ không hợp lệ. Cả ba cùng kiểm rằng đọc rồi ghi lại giữ nguyên trường đã biết và trường lạ không làm hỏng việc đọc.
- Bản đóng băng định danh trên dây (`internal/protocol/testdata/wire_names.golden.json`) được Go, Python và Dart cùng so, mỗi bên với những định danh nó khai báo. Đổi một định danh ở một phía mà không sửa golden thì test phía đó đỏ.
- Test chéo `product.json` ở Go, Python và Dart đỏ nếu hằng tên sản phẩm của một ngôn ngữ lệch khỏi tệp này.
- `internal/product` (Go) và test tương ứng phía Python đỏ nếu tên sản phẩm xuất hiện trong string literal ngoài chỗ khai báo.

## Tài liệu

- [Kiến trúc](docs/architecture.md)
- [Quy ước mã nguồn](docs/code-standards.md)
- [Hợp đồng giao thức](docs/protocol/README.md): [RPC agent](docs/protocol/rpc.md) · [đồng bộ agent–server](docs/protocol/sync.md) · [HTTP API](docs/protocol/http-api.md)

## Đóng góp

Đọc [quy ước mã nguồn](docs/code-standards.md) trước. Ba điều quan trọng nhất: định danh sản phẩm nằm ở `product.json` (mỗi ngôn ngữ giữ hằng riêng, test chéo bắt chỗ lệch), hợp đồng giao thức chỉ được cộng thêm, và không commit bí mật.
