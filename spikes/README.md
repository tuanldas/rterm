# spikes

Thí nghiệm kỹ thuật dùng một lần để trả lời một câu hỏi rủi ro trước khi viết mã thật (ví dụ: giữ ConPTY qua lúc daemon khởi động lại trên Windows, chụp lại màn hình đúng trạng thái giữa chừng `vim`).

**Thư mục này nằm ngoài build chính.** Nó là module Go riêng (`go.mod` riêng), nên `go build ./...`, `go test ./...`, `go vet ./...` và `golangci-lint run ./...` chạy ở gốc repo đều bỏ qua nó, và CI không build nó.

## Quy tắc

- Mỗi spike một thư mục con (`windows-conpty/`, `unix-holder/`, …) trả lời một câu hỏi, có tiêu chí đạt và không đạt viết trước khi làm.
- Mã spike là mã bỏ đi: không cần đẹp, không được dùng ngược lại trong mã thật. Thứ đáng giữ là **kết luận có số đo**; ghi vào báo cáo, không để trong mã.
- Không commit bí mật, tệp ghi âm chứa dữ liệu thật hay khoá riêng. Khoá thử nghiệm phải sinh từ chuỗi cố định và ghi rõ là vô giá trị.
- Dùng được gói của module chính bằng cách thêm vào `go.mod` ở đây: `replace github.com/tuanldas/rterm => ../`. Chỉ làm khi thật cần.
- Tài sản dùng chung sau này (ví dụ corpus terminal) chuyển sang [`../testdata`](../testdata/README.md) khi spike xong.
- Bỏ cả thư mục `spikes/` không ảnh hưởng gì tới sản phẩm.

Quy ước mã của dự án ([docs/code-standards.md](../docs/code-standards.md)) áp dụng ở mức vừa đủ: tên tệp, không bí mật, chú thích nói vì sao. Bộ quét tên sản phẩm bỏ qua thư mục này.
