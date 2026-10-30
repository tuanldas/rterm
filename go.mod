module github.com/tuanldas/rterm

go 1.26.0

// Bản Go dùng để build, test và quét lỗ hổng. CI cài đúng bản này (setup-go
// đọc go-version-file: go.mod); máy dev có Go cũ hơn tự tải nó khi GOTOOLCHAIN
// là auto (mặc định). Nâng khi Go ra bản vá; dòng go ở trên là phiên bản ngôn
// ngữ tối thiểu nên giữ nguyên.
toolchain go1.26.10

require github.com/santhosh-tekuri/jsonschema/v6 v6.0.3

require golang.org/x/text v0.43.0 // indirect
