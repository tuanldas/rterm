// Package protocol là bản Go của hợp đồng giao thức ở docs/protocol: hằng số,
// kiểu dữ liệu và định danh trên dây dùng chung cho agent, server và test.
//
// Gói này KHÔNG import internal/product. Định danh trên dây (tên method,
// capability, biến môi trường mang mã phiên, ngữ cảnh chữ ký) phải sống qua lần
// đổi tên sản phẩm vì client và agent cũ vẫn chạy; test TestDinhDanhTrenDay…
// chặn việc suy chúng từ tên sản phẩm.
package protocol

// Mỗi giao thức có một số proto nguyên. Đổi proto là thay đổi phá vỡ nên cấm
// trong v1: tính năng mới đi qua capability (xem capability.go), method lạ trả
// -32601 và bên gọi coi là "không hỗ trợ".
const (
	// RPCProto là phiên bản giao thức RPC của agent (subsystem agent-rpc).
	RPCProto = 1

	// SyncProto là phiên bản giao thức đồng bộ agent–server.
	SyncProto = 1

	// APIProto là phiên bản HTTP API, khớp với APIPrefix.
	APIProto = 1
)

const (
	// APIPrefix là tiền tố mọi endpoint HTTP của server. Thêm endpoint không
	// đổi tiền tố.
	APIPrefix = "/v1"

	// AgentSyncPath là endpoint WebSocket mà agent giữ kết nối ra.
	AgentSyncPath = APIPrefix + "/agent/sync"
)
