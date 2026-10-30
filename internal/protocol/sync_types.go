package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Kiểu dữ liệu của kênh đồng bộ agent–server (docs/protocol/sync.md,
// schema/sync.schema.json).

// Vai trò của một tài khoản trên một máy. Tập mở: vai trò lạ phải được coi là
// RoleMember, không bao giờ nâng quyền.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Applied là phiên bản snapshot quyền mà agent đang áp dụng. Thứ tự là từ điển:
// epoch lớn hơn thì mới hơn, bằng epoch thì rev lớn hơn là mới hơn.
type Applied struct {
	Epoch int64 `json:"epoch"`
	Rev   int64 `json:"rev"`
}

// SyncHello là tham số của hello (agent → server), yêu cầu đầu tiên của kênh.
type SyncHello struct {
	Proto        int          `json:"proto"`
	MachineID    string       `json:"machine_id"`
	AgentVersion string       `json:"agent_version"`
	Capabilities []Capability `json:"capabilities,omitempty"`
	Applied      Applied      `json:"applied"`
}

// SyncHelloResult là kết quả của hello: thử thách để agent ký trong auth.
type SyncHelloResult struct {
	Proto         int    `json:"proto"`
	ServerVersion string `json:"server_version"`

	// Nonce là 16 byte ngẫu nhiên (128 bit) mã hoá base64 chuẩn.
	Nonce        string       `json:"nonce"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// SyncAuth là tham số của auth (agent → server).
type SyncAuth struct {
	// Sig là chữ ký của host key trên thử thách, ngữ cảnh ContextSyncAuth.
	Sig string `json:"sig"`
}

// SignedEnvelope là snapshot quyền kèm chữ ký. Chữ ký phủ lên đúng các byte
// của Payload sau khi giải base64, không phủ lên JSON đã phân tích: nhờ vậy hai
// bên không phải đồng ý về cách chuẩn hoá JSON (thứ tự khoá, khoảng trắng, số).
// Bên nhận phải kiểm chữ ký TRƯỚC khi phân tích Payload.
type SignedEnvelope struct {
	// Payload là base64 chuẩn (có đệm) của các byte JSON của PolicySnapshot.
	Payload string `json:"payload"`

	// Sig là chữ ký, ngữ cảnh ContextPolicySnapshot.
	Sig string `json:"sig"`

	// KeyID chọn khoá kiểm chữ ký trong danh sách khoá ký đang được tin.
	KeyID string `json:"key_id"`
}

// EncodePayload mã hoá các byte cần ký thành giá trị của SignedEnvelope.Payload.
func EncodePayload(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// PayloadBytes trả các byte gốc mà chữ ký phủ lên.
func (e SignedEnvelope) PayloadBytes() ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("payload của snapshot không phải base64 hợp lệ: %w", err)
	}
	return b, nil
}

// PolicySnapshot là toàn bộ quyền truy cập của một máy tại một thời điểm. Luôn
// là bản đầy đủ (Full): danh sách Grants rỗng nghĩa là KHÔNG AI được vào, chứ
// không phải "không đổi".
type PolicySnapshot struct {
	Proto     int    `json:"proto"`
	MachineID string `json:"machine_id"`
	OSUser    string `json:"os_user"`

	// Epoch và Rev tăng đơn điệu; agent từ chối bản không mới hơn bản đang áp dụng.
	Epoch int64 `json:"epoch"`
	Rev   int64 `json:"rev"`

	// IssuedAt chỉ để tham khảo; tính hợp lệ của snapshot không phụ thuộc đồng hồ của agent.
	IssuedAt int64 `json:"issued_at"`
	Full     bool  `json:"full"`

	Grants    []Grant   `json:"grants"`
	Emergency Emergency `json:"emergency"`

	// ClearLocalRevocations là mã các thu hồi cục bộ mà server ký lệnh gỡ.
	ClearLocalRevocations []string `json:"clear_local_revocations,omitempty"`
}

// MarshalJSON bảo đảm Grants rỗng ra "[]" chứ không phải "null": rỗng là trạng
// thái hợp lệ, còn null có thể bị bên nhận coi là "không có thông tin".
func (s PolicySnapshot) MarshalJSON() ([]byte, error) {
	type alias PolicySnapshot
	if s.Grants == nil {
		s.Grants = []Grant{}
	}
	return json.Marshal(alias(s))
}

// Grant là quyền của một tài khoản trên máy. Không chứa email: snapshot đi qua
// agent mà mọi người được chia sẻ đều đọc được.
type Grant struct {
	AccountID string     `json:"account_id"`
	Role      string     `json:"role"`
	Keys      []GrantKey `json:"keys,omitempty"`
}

// GrantKey là một khoá công khai được phép đăng nhập của tài khoản.
type GrantKey struct {
	// Fingerprint có dạng SHA256:<base64 không đệm>.
	Fingerprint string `json:"fp"`

	// PublicKey là một dòng định dạng authorized_keys không kèm tuỳ chọn hay ghi chú.
	PublicKey string `json:"public_key"`
}

// Emergency là trạng thái đăng nhập khẩn cấp của máy.
type Emergency struct {
	Enabled    bool     `json:"enabled"`
	AccountIDs []string `json:"account_ids,omitempty"`
}

// PolicyKey là một khoá công khai ký snapshot quyền mà agent tin.
type PolicyKey struct {
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

// KeyEndorsement là tham số của policy.keys (server → agent): chứng thực của một
// khoá đang tin cho một khoá ký mới, ngữ cảnh ContextPolicyKeyRollover.
type KeyEndorsement struct {
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`

	// NotBefore là thời điểm sớm nhất khoá mới được dùng để ký snapshot.
	NotBefore int64 `json:"not_before"`

	// SignerKeyID là khoá đang tin đã ký chứng thực này.
	SignerKeyID string `json:"signer_key_id"`
	Sig         string `json:"sig"`
}

// MachineStatus là tham số của notification machine.status (agent → server).
type MachineStatus struct {
	SessionCount    int      `json:"session_count"`
	SessionAccounts []string `json:"session_accounts,omitempty"`

	// Addresses là các địa chỉ host:port agent đang lắng nghe.
	Addresses      []string `json:"addresses,omitempty"`
	HolderVersions []string `json:"holder_versions,omitempty"`
	AgentVersion   string   `json:"agent_version"`
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`

	Applied          Applied           `json:"applied"`
	LastReject       *Reject           `json:"last_reject,omitempty"`
	LocalRevocations []LocalRevocation `json:"local_revocations,omitempty"`
}

// Reject ghi lại lần agent từ chối snapshot gần nhất.
type Reject struct {
	Epoch  int64  `json:"epoch"`
	Rev    int64  `json:"rev"`
	Reason string `json:"reason"`
	At     int64  `json:"at"`
}

// LocalRevocation là một thu hồi do người dùng đặt ngay trên máy, thắng snapshot
// của server cho tới khi hết hạn hoặc server ký lệnh gỡ. Đặt theo khoá hoặc theo tài khoản.
type LocalRevocation struct {
	ID        string `json:"id"`
	KeyFP     string `json:"key_fp,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
}

// AuditReport là tham số của audit.report (agent → server): một lô sự kiện.
type AuditReport struct {
	Events []AuditEvent `json:"events"`
}

// AuditEvent là một sự kiện kiểm toán do máy báo. Server coi là "do máy báo",
// không tin tuyệt đối.
type AuditEvent struct {
	// EventID là 128 bit ngẫu nhiên dạng 32 chữ số hex thường; server khử trùng theo (máy, EventID).
	EventID   string `json:"event_id"`
	TS        int64  `json:"ts"`
	Action    string `json:"action"`
	AccountID string `json:"account_id,omitempty"`
	Target    string `json:"target,omitempty"`
	Result    string `json:"result,omitempty"`
	IP        string `json:"ip,omitempty"`

	// Count lớn hơn 1 khi nhiều sự kiện giống nhau được gom lại; vắng nghĩa là 1.
	Count int64 `json:"count,omitempty"`

	// Meta chỉ mang giá trị chuỗi, khoá nằm trong danh sách server cho phép.
	Meta map[string]string `json:"meta,omitempty"`
}

// AuditAck là kết quả của audit.report. Acked liệt kê mọi sự kiện server đã xử
// lý xong (ghi nhận, trùng lặp, hoặc bỏ vì ngoài danh sách cho phép); agent
// chỉ xoá khỏi hàng đợi những sự kiện có mặt ở đây. Người dựng nên khởi tạo
// Acked khác nil để kết quả ra "[]" chứ không phải "null".
type AuditAck struct {
	Acked []string `json:"acked"`
}

// EmergencyVerifyParams là tham số của emergency.verify (agent → server). Cố ý
// KHÔNG có trường mật khẩu tài khoản: agent là tiến trình mà mọi người được chia
// sẻ cùng user hệ điều hành đều thay được, nên nó chỉ được thấy bí mật khẩn cấp
// dùng riêng cho đúng máy này.
type EmergencyVerifyParams struct {
	Email           string `json:"email"`
	EmergencySecret string `json:"emergency_secret"`
	TOTP            string `json:"totp"`
	OSUser          string `json:"os_user"`
	RemoteIP        string `json:"remote_ip"`
}

// EmergencyVerifyResult là kết quả khi xác minh thành công. Agent phải đối chiếu
// AccountID với Emergency.AccountIDs của snapshot trước khi cho vào.
type EmergencyVerifyResult struct {
	AccountID string `json:"account_id"`
}
