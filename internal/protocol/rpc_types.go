package protocol

import "encoding/json"

// Kiểu dữ liệu của RPC agent (docs/protocol/rpc.md, schema/rpc.schema.json).
//
// Quy ước chung: trường danh sách vắng mặt nghĩa là rỗng; thời gian là Unix
// mili-giây UTC; bên nhận bỏ qua trường lạ.

// PeerInfo mô tả một đầu của kết nối.
type PeerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// HelloParams là tham số của hello, yêu cầu đầu tiên của client.
type HelloParams struct {
	Proto        int          `json:"proto"`
	Client       PeerInfo     `json:"client"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// HelloResult là kết quả của hello.
type HelloResult struct {
	Proto int      `json:"proto"`
	Agent PeerInfo `json:"agent"`

	// MachineID rỗng khi agent chưa từng được thêm vào server.
	MachineID    string       `json:"machine_id"`
	OSUser       string       `json:"os_user"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// SessionInfo mô tả một phiên terminal đang sống trên máy.
type SessionInfo struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`

	// AccountID là tài khoản sở hữu phiên.
	AccountID    string `json:"account_id"`
	CreatedAt    int64  `json:"created_at"`
	LastActiveAt int64  `json:"last_active_at"`
	Cols         int    `json:"cols"`
	Rows         int    `json:"rows"`

	// Clients là số client đang attach.
	Clients int `json:"clients"`

	// Durable cho biết phiên có sống tiếp qua lúc đăng xuất hoặc khởi động lại
	// daemon không; false khi môi trường không bảo đảm được.
	Durable bool `json:"durable"`
}

// SizeInfo là kích thước PTY của phiên và client đang giữ quyền đặt kích thước.
type SizeInfo struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`

	// OwnerClient là mã không trong suốt của client đang giữ quyền chủ cỡ; vắng
	// khi phiên không có client.
	OwnerClient string `json:"owner_client,omitempty"`
}

// SessionListResult là kết quả của session.list.
type SessionListResult struct {
	Sessions []SessionInfo `json:"sessions,omitempty"`
}

// SessionCreateParams là tham số của session.create; kết quả là SessionInfo.
type SessionCreateParams struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// SessionCloseParams là tham số của session.close; kết quả là đối tượng rỗng.
type SessionCloseParams struct {
	SessionID string `json:"session_id"`
}

// SessionSizeParams là tham số của session.size; kết quả là SizeInfo. Take đòi
// quyền chủ cỡ cho kết nối đang gọi.
type SessionSizeParams struct {
	SessionID string `json:"session_id"`
	Take      bool   `json:"take,omitempty"`
}

// EventsSubscribeParams là tham số của events.subscribe. Mỗi phần tử là một chủ
// đề, hoặc mẫu dạng "<tiền tố>.*" khớp mọi chủ đề bắt đầu bằng "<tiền tố>.".
type EventsSubscribeParams struct {
	Topics []string `json:"topics"`
}

// EventsSubscribeResult liệt kê các mẫu agent chấp nhận; mẫu agent không biết bị bỏ qua.
type EventsSubscribeResult struct {
	Topics []string `json:"topics,omitempty"`
}

// Event là tham số của notification event. Data là đối tượng JSON mà hình dạng
// phụ thuộc Topic: SessionInfo cho session.created|closed|attached|detached,
// SessionResizedData cho session.resized. Chủ đề lạ phải được bỏ qua.
type Event struct {
	Topic string          `json:"topic"`
	TS    int64           `json:"ts"`
	Data  json.RawMessage `json:"data"`
}

// SessionResizedData là Data của sự kiện session.resized.
type SessionResizedData struct {
	ID string `json:"id"`
	SizeInfo
}
