# rterm — Quy ước mã nguồn

**Cập nhật:** 30/10/2026

Kế thừa quy ước chung của các dự án Go của tác giả (moneylio), bỏ phần đặc thù PostgreSQL và tiền tệ, thêm phần riêng của một sản phẩm nhiều thành phần chạy trên nhiều hệ điều hành. Phần Go áp dụng cho agent; server viết bằng Python và app viết bằng Dart có mục riêng bên dưới.

## Quy tắc bất di bất dịch

Năm quy tắc dưới đây không có ngoại lệ. Vi phạm là lỗi nghiêm trọng, không phải chuyện phong cách.

### 1. Tên sản phẩm chỉ khai báo một chỗ

Tên sản phẩm, tên tệp chạy, mã gói app, tiền tố biến môi trường, tiền tố token, scheme của deep link, tên thư mục trạng thái và cổng mặc định nằm ở `internal/product` (mã Go). Mã Go khác lấy từ đó, không viết lại chuỗi. Mỗi thành phần khác giữ một chỗ khai báo riêng: app Dart ở `app/lib/core/product.dart`, server Python ở `server/src/rterm_server/product.py` (test `test_hang_san_pham_khop_product_json` so từng hằng, tên gói và tên lệnh console với `product.json` ở gốc repo). Tên sản phẩm là tên tạm và sẽ đổi trước khi phát hành, nên trong mỗi thành phần đổi tên phải là sửa một chỗ.

`TestTenSanPhamChiKhaiBaoMotCho` duyệt mọi string literal trong mã Go của repo và đỏ nếu thấy tên sản phẩm (hoặc tên cũ trong `LegacyNames()`) ngoài `internal/product`. Chỉ miễn import path và `internal/protocol/wire_names.go` (quy tắc 2). Module lồng nhau như `spikes/` nằm ngoài phạm vi. Phía Python, `test_ten_san_pham_chi_khai_bao_mot_cho` duyệt string literal trong `server/src` bằng `ast` (kể cả docstring, phần cố định của f-string, bytes), không phân biệt hoa thường; chỉ miễn `product.py`, `protocol/wire_names.py` và chuỗi đúng dạng đường dẫn gói (`rterm_server.migrations`).

### 2. Định danh trên dây không suy ra từ tên sản phẩm

Tên method, capability, biến môi trường mang mã phiên, ngữ cảnh chữ ký và mã thoát nằm ở `internal/protocol`, là hằng viết thẳng và **không import `internal/product`**. Dart và Python giữ định danh trên dây theo cách tương đương: hằng viết thẳng, tách khỏi nơi khai báo tên sản phẩm (phía Python là `server/src/rterm_server/protocol/wire_names.py`, chỉ import thư viện chuẩn). Client và agent đã phát hành vẫn chạy sau khi sản phẩm đổi tên; nếu định danh đổi theo, chữ ký cũ mất hiệu lực và `ssh` cũ bị đẩy sang nhánh tạo phiên mới thay vì vào lại phiên đang chạy.

Tên biến môi trường chỉ được thêm alias, không xoá. `TestDinhDanhTrenDayKhongPhuThuocTenSanPham` và bản đóng băng `internal/protocol/testdata/wire_names.golden.json` giữ quy tắc này. Bản đóng băng là mốc chung của các ngôn ngữ: Go (`TestDinhDanhTrenDayDuocDongBang`), Python (`test_dinh_danh_tren_day_khop_ban_dong_bang`) và Dart (`app/test/protocol/wire_names_test.dart`) cùng so với nó, nên thêm định danh vào registry mà quên một phía thì test phía đó đỏ. Go so mọi mục. Python và Dart chỉ so những mục phía mình khai báo; mục còn lại phải được liệt kê kèm lý do, nên mục mới của bản đóng băng chưa được xếp chỗ cũng làm test phía đó đỏ.

### 3. Giới hạn tài nguyên đi qua một điểm tập trung

Không viết con số giới hạn thẳng trong bộ xử lý: số phiên tối đa, kích thước thông điệp, số lần thử, hạn chót, hạn mức tần suất, độ dài hàng đợi. Chúng nằm ở `internal/limits` (Go), `app/lib/core/limits.dart` (Dart) và `server/src/rterm_server/limits.py` (Python); nơi dùng nhận giá trị từ đó, và các thư viện cấp thấp (như `internal/jsonrpc`) nhận giới hạn qua tham số hoặc tuỳ chọn.

```go
// Đúng
if n >= lim.MaxSessionsPerAccount {
    return ErrVuotHanMuc
}

// Sai: đổi giới hạn sẽ phải đi tìm và sửa từng chỗ như thế này
if n >= 16 {
    return ErrVuotHanMuc
}
```

Giá trị mặc định mà một thư viện tự đặt (như trần đồng thời của `jsonrpc`) chỉ là lưới an toàn, được đặt tên và chú thích; nơi dùng phải truyền giá trị thật.

### 4. Hợp đồng giao thức là nguồn sự thật và chỉ cộng thêm

`docs/protocol/` định nghĩa giao thức, không gắn với ngôn ngữ nào; mã Go, Python và Dart tuân theo và được kiểm bằng cùng một bộ ví dụ. Thay đổi giao thức là thêm capability, trường hoặc method mới; không xoá, không đổi nghĩa, không đổi `proto`. Chi tiết ở [docs/protocol/README.md](protocol/README.md).

### 5. Không có bí mật trong repo

Không commit `.env`, khoá riêng, keystore, token, mật khẩu hay dữ liệu cá nhân. `.gitignore` chặn `*.key`, `*.pem`, `*.jks`, `.env*`, `key.properties` và thư mục dữ liệu chạy thử `/data/`, `/server/data/`; mẫu thư mục được neo theo vị trí để không nuốt mã nguồn nằm trong thư mục cùng tên (ví dụ `app/lib/data/`). Khoá thử nghiệm dùng trong ví dụ và test là khoá công khai sinh từ chuỗi cố định, không bảo vệ gì; phải ghi rõ ở nơi dùng. Bí mật, mã TOTP, token và mật khẩu không bao giờ vào log hay sự kiện kiểm toán.

## Go (agent)

### Đặt tên

Tệp `snake_case.go`. Gói đặt tên theo danh từ chỉ lĩnh vực (`session`, `authz`, `jsonrpc`), không dùng `utils` hay `helpers`. Định danh trong mã (biến, hàm, kiểu, trường, bảng, cột) giữ tiếng Anh theo thông lệ; chỉ **tên test** và nội dung chú thích, thông báo lỗi, tài liệu viết tiếng Việt.

### Xử lý lỗi

Trả `error` thay vì hoảng loạn. Bọc lỗi kèm ngữ cảnh bằng `%w` để `errors.Is` và `errors.As` vẫn hoạt động ở tầng gọi. Thông báo lỗi viết tiếng Việt có dấu, mô tả chuyện gì đã xảy ra dưới góc nhìn người vận hành:

```go
return fmt.Errorf("đọc snapshot quyền %q: %w", path, err)
```

Lỗi gửi cho phía bên kia của kết nối (JSON-RPC, HTTP) không được lộ chi tiết nội bộ: handler trả lỗi có mã cho điều bên kia cần biết, mọi lỗi khác bị che thành lỗi nội bộ còn chi tiết ghi vào log.

Panic chỉ dành cho lỗi lập trình lộ ngay lúc khởi động (đăng ký handler trùng tên). Handler chạy theo yêu cầu của người ngoài phải bắt panic để một yêu cầu hỏng không làm chết cả daemon.

### Kiểm thử

Chỉ dùng thư viện chuẩn (`testing`), không testify. Hàm hỗ trợ gọi `t.Helper()`. Tài sản kiểm thử không phải mã Go đặt trong `testdata/` (Go bỏ qua thư mục này khi biên dịch).

Tên test mô tả **hành vi được bảo đảm**, tiếng Việt không dấu, không mô tả hàm được gọi:

```go
func TestPhienSongSotKhiDaemonKhoiDongLai(t *testing.T)   // đúng: nói rõ bảo đảm gì
func TestAttach(t *testing.T)                              // thiếu thông tin
```

**Mức độ test và quy tắc "làm hỏng có chủ đích".** Mỗi test thuộc một mức:

| Mức | Bảo vệ | Quy tắc |
|---|---|---|
| critical | bất biến an toàn, hợp đồng đóng băng, nguy cơ mất dữ liệu hay vỡ tương thích | **Bắt buộc** chứng minh bắt được lỗi |
| high | hành vi quan trọng của người dùng, giới hạn tài nguyên | khuyến khích |
| medium | còn lại | khuyến khích |

Chứng minh bắt được lỗi nghĩa là: viết xong, cố tình làm hỏng thứ test bảo vệ (đảo một điều kiện, bỏ một dòng kiểm, đổi một hằng), xác nhận test **đỏ**, khôi phục, xác nhận test **xanh**. Một test luôn xanh vì không kiểm tra gì còn tệ hơn không có test, vì nó tạo cảm giác an toàn giả. Ghi cách làm hỏng vào mô tả PR của test critical.

Test chạy trên cả ba hệ điều hành ở CI, có `-race` trên Linux và macOS (race detector của Windows cần cgo, mà dự án build với `CGO_ENABLED=0`). Test phụ thuộc thời gian chờ theo điều kiện chứ không ngủ cố định, hạn chót rộng để không chập chờn trên runner chậm.

### Chú thích

Chú thích giải thích **vì sao**, không lặp lại **cái gì** mã đã nói rõ. Viết tiếng Việt có dấu đầy đủ.

```go
// Đúng
// Đóng trước khi chờ để handler đang kẹt ở Write thoát ngay.
_ = p.conn.Close()

// Thừa
// Đóng kết nối
_ = p.conn.Close()
```

Không đưa mã kế hoạch, số giai đoạn hay nhãn rà soát vào chú thích, tên test, tên migration hay thông điệp commit. Diễn đạt thẳng bất biến hoặc hành vi.

### Phụ thuộc và giấy phép

Chỉ dùng thư viện có giấy phép dễ dãi: MIT, Apache-2.0, BSD-2/3, ISC, zlib và tương tự. Cấm copyleft (GPL, AGPL). Giấy phép nằm ngoài hai nhóm này (LGPL, MPL, …) làm cổng giấy phép đỏ, và phải hỏi người sở hữu dự án từng trường hợp trước khi dùng; không tự ý thêm ngoại lệ. Gói mà cổng không nhận ra được giấy phép cũng đỏ như vậy.

Người sở hữu dự án đồng ý thì ghi một dòng `exception` (hệ, gói, mã, phạm vi, lý do) vào `scripts/license_policy.txt`. Cả ba cổng áp ngoại lệ theo cùng quy tắc ghi ở đầu tệp đó:

- giấy phép cổng nhận ra được thì ngoại lệ phải ghi đúng mã đó, để gói đổi sang giấy phép khác vẫn bị chặn;
- phần giấy phép không nhận ra được (ví dụ văn bản tự do `License: Apache 2.0`) thì ngoại lệ đúng gói, đúng phạm vi gỡ được;
- mã bị cấm, dù nêu trong metadata hay ghi trong ngoại lệ, không ngoại lệ nào gỡ được.

**Giới hạn nhận diện hiện tại**, phải siết trước khi thêm phụ thuộc Go hay gói pub mới:

- cổng Go dựa vào go-licenses, vốn chỉ đọc tệp giấy phép đầu tiên nhận ra được của mỗi module. Module có `LICENSE` là MIT kèm `LICENSE-GPL` vẫn qua cổng, và điều khoản chèn thêm trong tệp đã nhận ra bị bỏ qua;
- cổng Dart nhận ra Apache-2.0 chỉ nhờ văn bản nhắc tên giấy phép, nên một giấy phép không dễ dãi có nhắc "Apache License, Version 2.0" (như BUSL) vẫn qua. Văn bản không nhận ra nằm cạnh một giấy phép đã nhận ra cũng bị bỏ qua; họ GPL thì vẫn bị chặn;
- cả hai cổng không đọc giấy phép đặt trong thư mục con của gói (ví dụ `third_party/`).

Trong phụ thuộc hiện tại, chỉ gói pub `characters` rơi vào ca cuối:
- Bảng dữ liệu của gói được biên dịch vào app và theo Unicode License v3. Giấy phép này đã nằm trong `allow` của `scripts/license_policy.txt`.
- Tệp CC-BY-SA-3.0 trong cùng thư mục `third_party/` chỉ dùng cho test của gói, không vào app.

Không gói nào rơi vào hai ca đầu.

**Phạm vi cổng giấy phép:** phụ thuộc Go, gói pub và gói PyPI, trực tiếp lẫn gián tiếp (cách kiểm phía Python ở mục [Python (server)](#python-server)). Engine Flutter nằm ngoài phạm vi vì không phải gói pub: SDK Flutter được phân phối nguyên khối, không chọn được qua `pubspec`, phần bên thứ ba của engine đều giấy phép dễ dãi và được liệt kê qua màn giấy phép của app.

Phụ thuộc chạy thật (runtime) giữ ít nhất có thể: không thêm gì ngoài thư viện chuẩn nếu chưa thật cần. Thư viện chỉ dùng trong test (như `santhosh-tekuri/jsonschema` kiểm schema) cũng phải qua cổng giấy phép.

## SQL

Migration đánh số bốn chữ số tăng dần, đặt tên theo lĩnh vực: `0002_machines.up.sql`. Chúng nằm trong gói Python `server/src/rterm_server/migrations/` (đóng vào wheel, đọc bằng `importlib.resources`) và chạy bằng runner riêng của server trên `sqlite3` của thư viện chuẩn, không qua ORM. Migration **không sửa lại sau khi đã chạy trên máy chủ thật**; thay đổi lược đồ luôn là migration mới, và chỉ cộng thêm (SQLite hạn chế `ALTER`). Ràng buộc đặt tên rõ nghĩa để thông báo lỗi tự giải thích:

```sql
CONSTRAINT machines_host_key_unique UNIQUE (host_key)
```

Phiên bản đầu (v1) chỉ chạy SQLite, nhưng viết SQL **theo quy ước di động** để port sang PostgreSQL ở v1.1 chỉ là thêm một engine rồi chạy lại bộ test hợp đồng của lớp lưu trữ:

| Chủ đề | Quy ước |
|---|---|
| Khoá và mã | `TEXT`; UUIDv7 sinh ở tầng ứng dụng; không dùng `AUTOINCREMENT`, không dựa vào mã tự sinh của cơ sở dữ liệu |
| Thời gian | `BIGINT`, Unix mili-giây UTC; không `DATETIME`, `TIMESTAMP` |
| Luận lý | `INTEGER` 0 hoặc 1; không `BOOLEAN` |
| Tham số | placeholder `?` |
| JSON | `TEXT` |
| So khớp không phân biệt hoa thường | cột chuẩn hoá (`email_norm`) tính ở tầng ứng dụng; không `COLLATE NOCASE`, `citext` |
| Hàm và kiểu | không dùng hàm hay kiểu riêng của SQLite; không `INSERT OR REPLACE` |

Test tĩnh quét thư mục migration để giữ quy ước này.

## Dart và Flutter

Tệp `snake_case.dart`. Mọi chuỗi hiển thị cho người dùng đi qua ARB và `intl`, không viết thẳng trong widget (test chặn `Text('…')` cứng). Giới hạn ở `app/lib/core/limits.dart`; tên sản phẩm và scheme ở `app/lib/core/product.dart`, tách khỏi định danh trên dây giống phía Go. Dart định dạng bằng `dart format`, phân tích bằng `flutter analyze`.

## Python (server)

Server tài khoản viết bằng Python 3.14 (`requires-python = "==3.14.*"`, `.python-version`), là một dự án uv ở `server/`: mã trong `src/rterm_server/`, test trong `server/tests/`, đóng gói bằng `uv_build`. Server chỉ phát hành dạng image Docker. Nguyên tắc chung kế thừa từ phần Go; mục này ghi phần riêng của Python.

### Môi trường và phụ thuộc

Phụ thuộc nằm hết ở `[dependency-groups]`, ghim chính xác (`==`): nhóm `app` là đúng thứ nằm trong image, nhóm `dev` chỉ để phát triển và kiểm thử. Phụ thuộc gián tiếp do `uv.lock` giữ; cài bằng `uv sync --frozen`, CI kiểm thêm `uv lock --check` để `uv.lock` không lệch `pyproject.toml`. Thêm hay nâng một gói là việc có chủ đích: `uv add --group app <gói>==<bản>`, đọc changelog, chạy lại cổng giấy phép. Phụ thuộc runtime giữ ít nhất có thể: không dùng extra kéo thêm gói khi không cần (`uvicorn` không `[standard]`, `jsonschema` không `[format]` vì kéo gói GPL). Mọi gói phải có wheel cho Linux amd64/arm64, macOS arm64 và Windows; CI cài với `--no-build` để lộ gói thiếu wheel.

### Đặt tên

Tệp và module `snake_case.py`; gói đặt tên theo lĩnh vực (`protocol`, `store`), không `utils` hay `helpers`; không trùng tên module thư viện chuẩn (`server_secrets`, `tokens` thay cho `secrets`, `token`). Tên sản phẩm chỉ ở `product.py`, định danh trên dây chỉ ở `protocol/wire_names.py` (quy tắc 1, 2). Định danh trong mã giữ tiếng Anh.

### Định dạng, lint và kiểu

- `ruff format`, `line-length` 100.
- `ruff check` với luật `E`, `F`, `I`, `B`, `UP` cộng `S` (bảo mật: SQL ghép chuỗi, bí mật viết thẳng), `ASYNC` (gọi hàm chặn trong `async def`), `DTZ` (datetime không múi giờ). `S101` (assert) chỉ được bỏ qua trong `tests/`. Bỏ qua một luật tại chỗ phải kèm lý do: `# noqa: S105 - tiền tố công khai, không phải bí mật`.
- mypy `strict` (cộng `warn_unreachable`, plugin `pydantic.mypy`) trên `src/` và `scripts/check_python_licenses.py`; test không kiểm kiểu. Trong `src/`, `Any` chỉ dùng khi có chú thích nói vì sao; JSON tự do dùng `pydantic.JsonValue`.

### Mã bất đồng bộ và giới hạn

- Không gọi `sqlite3` hay hàm băm mật khẩu trực tiếp trong `async def`: chạy qua executor riêng, nếu không event loop đứng và mọi kết nối cùng chờ. `ruff` `ASYNC` không bắt được hết các ca này nên review phải để ý.
- Giới hạn tài nguyên (hạn mức tần suất, số lần thử, kích thước, hạn chót, độ dài hàng đợi) đi qua `server/src/rterm_server/limits.py` (quy tắc 3).
- Thời gian trên dây là Unix mili-giây UTC; trong mã dùng datetime có múi giờ.

### Hợp đồng giao thức

Model thông điệp ở `rterm_server.protocol` (`sync`, `http_api`), tên lớp trùng tên `$defs` của schema, kế thừa `WireMessage`: bỏ qua trường lạ, kiểu chặt (không đổi `"1"` thành `1`), danh sách tuỳ chọn mặc định rỗng, và `to_wire()` ghi ra dạng chuẩn của người gửi (trường tuỳ chọn không có giá trị thì vắng mặt). `tests/test_protocol_conformance.py` chạy trên `docs/protocol/examples` (miền `sync`, `http-api`): ví dụ hợp lệ qua schema và đọc–ghi lại không đổi, trường lạ bị bỏ qua, ví dụ `.invalid.json` bị từ chối, model khớp trường và trường bắt buộc của schema. `jsonschema` không kiểm `contentSchema`, nên test tự giải base64 `payload` của `SignedEnvelope` rồi kiểm như `PolicySnapshot`; chỗ dùng `contentSchema` mới phải được khai vào test.

### Kiểm thử

pytest, dữ liệu thật, không mock. Test bất đồng bộ dùng plugin pytest của `anyio` (có sẵn qua Starlette), không thêm pytest-asyncio. Tên test mô tả bảo đảm, tiếng Việt không dấu: `test_ma_totp_dung_lai_bi_tu_choi`, không phải `test_verify_totp`. Mỗi module test mở đầu bằng docstring tiếng Việt có dấu nêu các bảo đảm nó kiểm. Danh sách tham số rỗng làm test lỗi lúc thu thập (`empty_parameter_set_mark`), để đọc nhầm thư mục ví dụ không thành xanh giả. Quy tắc "làm hỏng có chủ đích" áp như phần Go.

### Lệnh

Chạy từ `server/`:

```sh
uv sync --frozen                    # cài đủ nhóm app + dev
uv run ruff check --config pyproject.toml . ../scripts/check_python_licenses.py
uv run ruff format --check --config pyproject.toml . ../scripts/check_python_licenses.py
uv run mypy
uv run pytest
uv run rterm-server version

# Cổng giấy phép: hai lượt, mỗi lượt trong đúng môi trường của nó
uv sync --frozen --no-default-groups --group app
uv run --no-sync python ../scripts/check_python_licenses.py --scope runtime
uv sync --frozen
uv run --no-sync python ../scripts/check_python_licenses.py --scope dev
```

**Cổng giấy phép PyPI** (`scripts/check_python_licenses.py`, chỉ thư viện chuẩn) đọc metadata của môi trường đang chạy theo thứ tự `License-Expression` (PEP 639) → classifier → trường `License` nếu là biểu thức SPDX, rồi xếp loại theo `scripts/license_policy.txt`: `AND` lấy mức tệ nhất, `OR` lấy mức tốt nhất. Giấy phép ngoài danh sách cho phép hoặc không nhận ra được là đỏ, trừ khi có ngoại lệ theo quy tắc chung ở mục [Phụ thuộc và giấy phép](#phụ-thuộc-và-giấy-phép). Mã vẫn nhận ra được trong metadata đọc dở (classifier đã ánh xạ đi kèm classifier chưa ánh xạ) được xét như giấy phép đọc được, nên GPL nêu ở đó vẫn thắng ngoại lệ. Lượt runtime chặt, ngoại lệ phạm vi `dev` không áp; lượt dev áp các ngoại lệ đã ghi. `pip-audit` (chạy qua `uvx`, ghim bản) quét lỗ hổng closure runtime. Công cụ chạy qua `uvx` không phải phụ thuộc nên ngoài phạm vi cổng giấy phép, chỉ ghim bản. Cổng đọc môi trường đang cài nên chỉ thấy gói của nền tảng đang chạy. CI chạy cổng trên Linux, vì vậy phụ thuộc chỉ có trên Windows không qua cổng; hiện chỉ có `colorama` (BSD-3-Clause, pytest kéo vào khi chạy trên Windows). Thêm phụ thuộc có marker nền tảng thì phải tự xem giấy phép của nó.

## Git

Conventional commits, mô tả bằng tiếng Việt:

```
feat(agent): thêm holder giữ phiên qua lúc daemon khởi động lại
fix(jsonrpc): đánh thức lời gọi đang chờ khi kết nối đóng
```

Phần thân giải thích **vì sao** thay đổi, không liệt kê lại tệp đã sửa; `git diff` đã nói điều đó. Commit nhỏ, tập trung một việc. Không đưa mã kế hoạch, số giai đoạn hay nhãn rà soát vào commit. Không commit bí mật (quy tắc 5).

## Ngôn ngữ

Chú thích, docstring, thông báo lỗi, tài liệu, thông điệp commit và tên test viết tiếng Việt có dấu đầy đủ (tên test Go và Python không dấu vì là định danh). Định danh trong mã giữ tiếng Anh.
