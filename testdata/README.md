# testdata

Tài sản test dùng chung giữa các thành phần (Go, Dart, và Python nếu server cần). Go bỏ qua thư mục tên `testdata` khi biên dịch; phía Dart đọc trực tiếp từ đây (`app/test`).

Tài sản chỉ dùng cho một gói Go đặt ở `testdata/` của chính gói đó (ví dụ `internal/protocol/testdata/`). Ở đây chỉ để thứ mà **nhiều ngôn ngữ** cùng kiểm.

## Bố cục dự kiến

| Thư mục | Nội dung | Dùng để |
|---|---|---|
| `vt-corpus/` | luồng byte thô ghi từ `vim`, `htop`, `less`, `git log --graph`, chuỗi tiếng Việt; kèm corpus tấn công (truy vấn nguy hiểm) và corpus đổi cỡ | kiểm snapshot màn hình ở agent và hiển thị ở app |
| `ssh-sig-vectors/` | chữ ký và khoá SSH do Go sinh | kiểm Dart đổi chữ ký ECDSA và khoá P-256 sang dạng SSH khớp Go |
| `tls-spki-vectors/` | chứng chỉ và pin SPKI do Go sinh | kiểm Dart tính pin SPKI khớp Go |

Hợp đồng giao thức có ví dụ riêng ở [`../docs/protocol/examples`](../docs/protocol/examples); không đặt ở đây.

## Quy ước

- Mỗi thư mục có mô tả cách sinh dữ liệu và tệp nào là đầu vào, tệp nào là kết quả mong đợi.
- Vector sinh bằng mã (Go) thì dữ liệu sinh ra được commit và test kiểm lại; có cờ `-update` để sinh lại có chủ đích.
- **Chỉ khoá công khai thử nghiệm sinh từ chuỗi cố định.** Không khoá thật, không token, không dữ liệu cá nhân, không ghi âm phiên thật.
- **Tệp byte thô** (luồng terminal có `\r\n`, mã escape) không được để Git chuyển dòng. `.gitattributes` của repo đặt `* text=auto eol=lf`, nên khi thêm corpus phải đánh dấu chúng `binary` trong `.gitattributes` (ví dụ đuôi `*.vt binary`); nếu không, Git có thể đổi `\r\n` thành `\n` làm hỏng dữ liệu.
