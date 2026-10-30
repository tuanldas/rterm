// Package jsonrpc là thư viện JSON-RPC 2.0 hai chiều dùng chung cho kênh RPC
// của agent và kênh đồng bộ agent–server. Gói không biết gì về nghiệp vụ: vận
// chuyển (NDJSON, WebSocket) nằm sau interface Conn, mã lỗi riêng của sản phẩm
// nằm ở gói protocol.
package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Mã lỗi chuẩn của JSON-RPC 2.0 và mã lỗi của chính thư viện.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	// CodeServerBusy thuộc dải "lỗi do bên cài đặt định nghĩa" (-32000..-32099)
	// của JSON-RPC. Dải -32001..-32004 dành cho mã riêng của sản phẩm
	// (gói protocol), nên thư viện chỉ dùng -32000.
	CodeServerBusy = -32000
)

var (
	// ErrClosed báo kết nối đã đóng; lỗi gốc (nếu có) đi kèm qua errors.Is/As.
	ErrClosed = errors.New("kết nối JSON-RPC đã đóng")

	// ErrMessageTooLarge báo thông điệp vượt giới hạn kích thước của kết nối.
	ErrMessageTooLarge = errors.New("thông điệp JSON-RPC vượt giới hạn kích thước")

	// ErrInvalidMessage báo thông điệp không gửi được qua kết nối, ví dụ có
	// ký tự xuống dòng làm vỡ khung NDJSON.
	ErrInvalidMessage = errors.New("thông điệp JSON-RPC không hợp lệ")

	// ErrAlreadyRunning báo Peer.Run đã được gọi; mỗi Peer chỉ chạy một lần.
	ErrAlreadyRunning = errors.New("Peer đã chạy, mỗi Peer chỉ chạy một lần")
)

// Error là lỗi JSON-RPC đi trên dây. Handler trả *Error để quyết định mã lỗi
// gửi cho bên gọi; bất kỳ lỗi nào khác bị che thành CodeInternalError để không
// lộ chi tiết nội bộ.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("lỗi JSON-RPC %d: %s", e.Code, e.Message)
}

// NewError tạo lỗi không kèm dữ liệu.
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewErrorData tạo lỗi kèm dữ liệu có cấu trúc. Dữ liệu không mã hoá được thì
// bỏ đi chứ không thay đổi mã lỗi mà bên gọi đang chờ.
func NewErrorData(code int, message string, data any) *Error {
	e := &Error{Code: code, Message: message}
	if b, err := json.Marshal(data); err == nil {
		e.Data = b
	}
	return e
}

// CodeOf trả mã lỗi JSON-RPC nếu err (hoặc lỗi nó bọc) là *Error.
func CodeOf(err error) (code int, ok bool) {
	var re *Error
	if errors.As(err, &re) {
		return re.Code, true
	}
	return 0, false
}

// IsMethodNotFound cho biết bên kia không hỗ trợ method. Đây là tín hiệu để bên
// gọi coi tính năng là "không có" và dùng tiếp phần còn lại của kết nối, không
// phải lý do để ngắt.
func IsMethodNotFound(err error) bool {
	code, ok := CodeOf(err)
	return ok && code == CodeMethodNotFound
}

// DecodeParams giải mã params của một yêu cầu vào v. Trường lạ bị bỏ qua để
// bên gửi bản mới hơn vẫn nói chuyện được với bên nhận bản cũ. Params vắng mặt
// hoặc null giữ nguyên giá trị zero của v; bắt buộc trường nào là việc của handler.
func DecodeParams(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	err := json.Unmarshal(raw, v)
	if err == nil {
		return nil
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return NewError(CodeInvalidParams, fmt.Sprintf("tham số sai kiểu ở trường %q", typeErr.Field))
	}
	// Không đưa nguyên văn lỗi của encoding/json: nó nêu tên kiểu Go nội bộ.
	return NewError(CodeInvalidParams, "tham số không đúng định dạng")
}
