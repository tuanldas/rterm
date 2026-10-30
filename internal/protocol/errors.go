package protocol

import (
	"encoding/json"
	"fmt"
)

// Mã lỗi JSON-RPC riêng của sản phẩm, dùng cho cả RPC agent lẫn kênh đồng bộ.
// Mã chuẩn (-32700..-32600) và -32000 thuộc gói jsonrpc.
const (
	CodeUnauthorized  = -32001 // chưa xác thực hoặc không đủ quyền với thao tác
	CodeNotFound      = -32002 // đối tượng được nhắc tới không tồn tại (hoặc ngoài phạm vi)
	CodeLimitExceeded = -32003 // vượt giới hạn; ErrorData.RetryAfter cho biết chờ bao lâu
	CodeConflict      = -32004 // trạng thái hiện tại không cho phép; ErrorData.Reason nêu lý do
)

// ErrorData là nội dung "data" của các lỗi CodeUnauthorized..CodeConflict. Cả hai
// trường đều tuỳ chọn.
type ErrorData struct {
	// Reason là mã lý do ổn định, máy đọc được (xem Reason… bên dưới).
	Reason string `json:"reason,omitempty"`

	// RetryAfter là số giây nên chờ trước khi thử lại.
	RetryAfter int `json:"retry_after,omitempty"`
}

// Lý do từ chối snapshot quyền (CodeConflict của policy.apply). Tập mở: bên gửi
// coi lý do lạ là "bị từ chối" và hiện nguyên văn.
const (
	ReasonStale        = "stale"          // (epoch, rev) không lớn hơn bản đang áp dụng
	ReasonBadSignature = "bad_signature"  // chữ ký sai hoặc khoá không được tin
	ReasonUnknownKeyID = "unknown_key_id" // key_id không có trong danh sách khoá đang tin
	ReasonBadPayload   = "bad_payload"    // payload không giải mã được thành snapshot
	ReasonBadProto     = "bad_proto"      // proto của snapshot không được hỗ trợ
	ReasonWrongMachine = "wrong_machine"  // machine_id không phải của máy này
	ReasonWrongOSUser  = "wrong_os_user"  // os_user không phải của máy này
	ReasonNotFull      = "not_full"       // snapshot không mang cờ full
)

// APIError là lỗi của HTTP API. Thân phản hồi lỗi là ErrorResponse.
type APIError struct {
	// Code có dạng <miền>.<lý_do>, ổn định, để client ánh xạ sang thông báo hiển thị.
	Code string `json:"code"`

	// Message là mô tả cho người vận hành, không dành để hiển thị trực tiếp cho người dùng cuối.
	Message string `json:"message"`

	// Details là dữ liệu bổ sung riêng của từng mã lỗi (xem http-api.md), có thể vắng.
	Details json.RawMessage `json:"details,omitempty"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorResponse là thân JSON của mọi phản hồi lỗi HTTP.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// Mã lỗi HTTP khởi đầu. Tập mở: client gặp mã lạ hiện thông báo chung.
const (
	APIErrRequestInvalid     = "request.invalid"
	APIErrRequestTooLarge    = "request.too_large"
	APIErrUnauthenticated    = "auth.unauthenticated"
	APIErrInvalidCredentials = "auth.invalid_credentials"
	APIErrTOTPEnrollRequired = "auth.totp_enrollment_required"
	APIErrStepUpRequired     = "auth.step_up_required"
	APIErrForbidden          = "access.forbidden"
	APIErrNotFound           = "resource.not_found"
	APIErrConflict           = "resource.conflict"
	APIErrRateLimited        = "rate.limited"
	APIErrServerBusy         = "server.busy"
	APIErrSetupTokenInvalid  = "setup.invalid_token"
	APIErrPoPInvalid         = "key.pop_invalid"
	APIErrEnrollPending      = "enroll.authorization_pending"
	APIErrEnrollSlowDown     = "enroll.slow_down"
	APIErrEnrollExpired      = "enroll.expired_token"
	APIErrEnrollDenied       = "enroll.access_denied"
)
