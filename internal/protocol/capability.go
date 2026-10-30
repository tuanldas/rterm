package protocol

import "slices"

// Capability là tên một nhóm tính năng có version, dạng <miền>.v<n>. Hai bên
// trao đổi danh sách capability lúc bắt tay; chỉ được gọi method của một nhóm
// khi bên kia đã quảng bá capability tương ứng.
//
// Registry chỉ được thêm: không xoá, không đổi nghĩa. Tính năng mới là capability
// mới, không phải proto mới.
type Capability string

// Capability của RPC agent (hello của subsystem agent-rpc).
const (
	CapSessionV1 Capability = "session.v1" // session.list|create|close|size
	CapEventsV1  Capability = "events.v1"  // events.subscribe và notification event
)

// Capability của kênh đồng bộ agent–server (hello của WebSocket sync).
const (
	CapPolicyV1 Capability = "policy.v1" // policy.apply, policy.keys
	CapStatusV1 Capability = "status.v1" // machine.status
	CapAuditV1  Capability = "audit.v1"  // audit.report
)

// CapEmergencyV1 dùng ở hai nơi: agent quảng bá trên kênh đồng bộ (cho phép
// emergency.verify) và server quảng bá trong GET /v1/meta (có API khẩn cấp).
const CapEmergencyV1 Capability = "emergency.v1"

// Capability HTTP trong GET /v1/meta. Tính năng HTTP thêm sau bản v0.1.0 phải
// có capability ở đây để app ẩn tính năng khi gặp server cũ.
const (
	CapDevicesV1 Capability = "devices.v1"
	CapGroupsV1  Capability = "groups.v1"
	CapSharingV1 Capability = "sharing.v1"
	CapSSHKeysV1 Capability = "ssh_keys.v1"
)

// Has cho biết caps có chứa c không. Danh sách capability luôn ngắn nên tìm tuyến tính.
func Has(caps []Capability, c Capability) bool {
	return slices.Contains(caps, c)
}
