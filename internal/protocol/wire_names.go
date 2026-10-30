package protocol

// Tệp này gom định danh xuất hiện trên dây mà không phải là kiểu JSON. Chúng là
// hợp đồng đóng băng: chỉ được thêm, không xoá, không đổi giá trị, và không suy
// ra từ internal/product. Tệp này là nơi duy nhất ngoài product được phép chứa
// tên sản phẩm (xem TestTenSanPhamChiKhaiBaoMotCho).

// SessionEnvVars liệt kê tên biến môi trường mà client gửi (yêu cầu env của
// SSH) để chọn phiên khi mở kênh terminal. Giá trị là mã phiên hoặc
// SessionEnvNew. Agent nhận MỌI tên trong danh sách; phần tử đầu là tên chính,
// được đặt vào môi trường của shell với giá trị là mã phiên.
//
// Đây là hằng của giao thức chứ không suy ra từ tên sản phẩm: app và lệnh ssh
// đã phát hành vẫn gửi đúng tên này sau khi sản phẩm đổi tên. Nếu suy từ tên
// sản phẩm, client cũ sẽ bị đẩy sang nhánh "không có biến" và mở phiên mới thay
// vì vào lại phiên đang chạy. Chỉ được thêm alias, không xoá.
var SessionEnvVars = []string{"RTERM_SESSION"}

// SessionEnvNew là giá trị đặc biệt của biến phiên: tạo phiên mới. Mã phiên
// thật không bao giờ trùng giá trị này.
const SessionEnvNew = "new"

// ExitSessionNotFound là exit-status của kênh terminal khi mã phiên trong biến
// phiên không còn tồn tại, đi kèm một dòng stderr StderrSessionNotFound.
//
// Chọn 66 (EX_NOINPUT trong sysexits.h: thứ được yêu cầu để làm đầu vào không có).
// Các giá trị bị loại vì shell và công cụ đã dùng sẵn: 0 (thành công), 1 (lỗi
// chung), 2 (dùng sai), 126 (không thực thi được), 127 (không tìm thấy lệnh),
// 128+n (chết vì tín hiệu n, tức 129..192 với tín hiệu thông dụng) và 255 (mã
// ssh tự trả khi chính nó lỗi, cũng là mã "ngoài dải").
//
// Shell của người dùng vẫn có thể tự thoát mã 66, nên client chỉ coi phiên đã
// mất khi thấy cả mã này lẫn dòng stderr, trước khi nhận byte stdout đầu tiên.
const ExitSessionNotFound = 66

// StderrSessionNotFound là dòng stderr đi kèm ExitSessionNotFound.
const StderrSessionNotFound = "session_not_found"

// SubsystemAgentRPC là tên subsystem SSH mang RPC của agent.
const SubsystemAgentRPC = "agent-rpc"

// Method RPC của agent (subsystem SubsystemAgentRPC).
const (
	RPCMethodHello           = "hello"
	RPCMethodSessionList     = "session.list"
	RPCMethodSessionCreate   = "session.create"
	RPCMethodSessionClose    = "session.close"
	RPCMethodSessionSize     = "session.size"
	RPCMethodEventsSubscribe = "events.subscribe"

	// RPCNotifyEvent là notification agent gửi cho client đã events.subscribe.
	RPCNotifyEvent = "event"
)

// Chủ đề sự kiện của notification RPCNotifyEvent.
const (
	TopicSessionCreated  = "session.created"
	TopicSessionClosed   = "session.closed"
	TopicSessionAttached = "session.attached"
	TopicSessionDetached = "session.detached"
	TopicSessionResized  = "session.resized"
)

// Method của kênh đồng bộ agent–server (WebSocket tại AgentSyncPath).
const (
	SyncMethodHello           = "hello"
	SyncMethodAuth            = "auth"
	SyncMethodPolicyApply     = "policy.apply"
	SyncMethodPolicyKeys      = "policy.keys"
	SyncMethodMachineStatus   = "machine.status"
	SyncMethodAuditReport     = "audit.report"
	SyncMethodEmergencyVerify = "emergency.verify"
)
