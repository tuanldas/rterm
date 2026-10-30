package protocol

// Kiểu dữ liệu của HTTP API (docs/protocol/http-api.md,
// schema/http-api.schema.json). Thân yêu cầu và phản hồi đều là JSON UTF-8.
//
// Quy ước chung: trường danh sách vắng mặt nghĩa là rỗng; thời gian là Unix
// mili-giây UTC; bên nhận bỏ qua trường lạ.

// Loại khoá của tài khoản.
const (
	KeyKindDevice = "device" // khoá do app tạo trong kho khoá của thiết bị
	KeyKindSSH    = "ssh"    // khoá công khai do người dùng dán vào
)

// Trạng thái áp dụng quyền của một máy. Tập mở: client coi giá trị lạ là PolicyStatePending.
const (
	PolicyStateInSync   = "in_sync"
	PolicyStatePending  = "pending"
	PolicyStateRejected = "rejected"
)

// Meta là phản hồi của GET /v1/meta: không cần đăng nhập, cho client biết
// server hiểu những gì trước khi làm bất cứ việc gì khác.
type Meta struct {
	ServerVersion string       `json:"server_version"`
	API           int          `json:"api"`
	Capabilities  []Capability `json:"capabilities,omitempty"`

	// SetupRequired là true khi server chưa có tài khoản nào và đang chờ thiết lập.
	SetupRequired bool `json:"setup_required"`
}

// SetupRequest là thân của POST /v1/setup.
type SetupRequest struct {
	SetupToken string `json:"setup_token"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

// SetupResult là phản hồi của POST /v1/setup. Tài khoản chưa có TOTP nên chỉ
// nhận token tạm dùng để đăng ký TOTP.
type SetupResult struct {
	TOTPEnrollmentToken string `json:"totp_enrollment_token"`
}

// LoginRequest là thân của POST /v1/auth/login. Cần đúng một trong TOTP hoặc
// RecoveryCode, trừ tài khoản chưa đăng ký TOTP.
type LoginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	TOTP         string `json:"totp,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
	DeviceName   string `json:"device_name"`
}

// TokenPair là cặp token sau khi xác thực. Token là chuỗi không trong suốt.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`

	// AccessExpiresAt là thời điểm access token hết hạn.
	AccessExpiresAt int64 `json:"access_expires_at"`
}

// RefreshRequest là thân của POST /v1/auth/refresh; phản hồi là TokenPair mới.
// Refresh token dùng một lần: dùng lại thu hồi cả họ token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// StepUpRequest là thân của POST /v1/auth/step-up: xác thực lại bằng TOTP để
// làm thao tác nhạy cảm trong cửa sổ ngắn sau đó.
type StepUpRequest struct {
	TOTP string `json:"totp"`
}

// TOTPEnrollment là phản hồi của POST /v1/auth/totp/enroll.
type TOTPEnrollment struct {
	// Secret là khoá TOTP dạng base32, để nhập tay.
	Secret string `json:"secret"`

	// OTPAuthURI là URI otpauth://totp/… để app TOTP quét.
	OTPAuthURI string `json:"otpauth_uri"`
}

// TOTPConfirmRequest là thân của POST /v1/auth/totp/confirm.
type TOTPConfirmRequest struct {
	Code string `json:"code"`
}

// TOTPConfirmResult là phản hồi khi xác nhận TOTP thành công: người dùng đã đăng
// nhập, nên có TokenPair, kèm các mã khôi phục chỉ hiện đúng một lần.
type TOTPConfirmResult struct {
	TokenPair
	RecoveryCodes []string `json:"recovery_codes"`
}

// Me là phản hồi của GET /v1/me.
type Me struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	TOTPEnabled bool   `json:"totp_enabled"`

	// SoftLockedUntil có mặt khi tài khoản đang bị khoá mềm sau nhiều lần sai TOTP.
	SoftLockedUntil int64 `json:"soft_locked_until,omitempty"`
	CreatedAt       int64 `json:"created_at"`
}

// ChangePasswordRequest là thân của POST /v1/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Device là một thiết bị (phiên đăng nhập) của tài khoản.
type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt int64  `json:"last_used_at"`

	// Current là true cho thiết bị đang gọi API.
	Current bool `json:"current"`
}

// DeviceList là phản hồi của GET /v1/me/devices.
type DeviceList struct {
	Devices []Device `json:"devices,omitempty"`
}

// Alert là một cảnh báo hiện trong app, lấy từ các sự kiện kiểm toán nổi bật.
// App ánh xạ Action sang thông báo hiển thị; hành động lạ hiện thông báo chung.
type Alert struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	TS        int64  `json:"ts"`
	MachineID string `json:"machine_id,omitempty"`

	// Target là tên của thứ bị tác động (tên thiết bị, tên khoá), nếu có.
	Target string `json:"target,omitempty"`
}

// AlertList là phản hồi của GET /v1/me/alerts.
type AlertList struct {
	Alerts []Alert `json:"alerts,omitempty"`
}

// KeyChallenge là phản hồi của POST /v1/me/keys/challenge: nonce 16 byte (base64
// chuẩn) mà khoá thiết bị ký cùng origin của server (ContextDeviceKeyPoP) để
// chứng minh sở hữu.
type KeyChallenge struct {
	Nonce string `json:"nonce"`
}

// RegisterKeyRequest là thân của POST /v1/me/keys. Khoá thiết bị (KeyKindDevice)
// bắt buộc có Nonce và Sig, ngữ cảnh ContextDeviceKeyPoP; khoá dán tay
// (KeyKindSSH) không cần chứng minh sở hữu.
type RegisterKeyRequest struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Nonce     string `json:"nonce,omitempty"`
	Sig       string `json:"sig,omitempty"`

	// HardwareBacked và SecurityLevel do app tự khai, chỉ mang tính thông tin.
	HardwareBacked bool   `json:"hardware_backed,omitempty"`
	SecurityLevel  string `json:"security_level,omitempty"`
}

// AccountKey là một khoá công khai của tài khoản.
type AccountKey struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Fingerprint string `json:"fp"`
	PublicKey   string `json:"public_key,omitempty"`

	HardwareBacked bool  `json:"hardware_backed,omitempty"`
	CreatedAt      int64 `json:"created_at"`

	// DeviceID là thiết bị sở hữu khoá khi Kind là KeyKindDevice.
	DeviceID string `json:"device_id,omitempty"`
}

// RegisterKeyResult là phản hồi của POST /v1/me/keys.
type RegisterKeyResult struct {
	Key                AccountKey `json:"key"`
	AffectedMachineIDs []string   `json:"affected_machine_ids,omitempty"`
}

// KeyList là phản hồi của GET /v1/me/keys.
type KeyList struct {
	Keys []AccountKey `json:"keys,omitempty"`
}

// AffectedMachines là phản hồi của mọi thao tác đổi quyền không có kết quả riêng
// (xoá khoá, thu hồi thiết bị…): các máy có snapshot quyền thay đổi, để app hiện
// "đã áp trên N/M máy".
type AffectedMachines struct {
	AffectedMachineIDs []string `json:"affected_machine_ids,omitempty"`
}

// EnrollStartRequest là thân của POST /v1/enroll/start do agent gửi.
type EnrollStartRequest struct {
	// HostKey là host key công khai của agent, dòng định dạng authorized_keys.
	HostKey      string `json:"host_key"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	OSUser       string `json:"os_user"`
	AgentVersion string `json:"agent_version"`

	// AllowedCIDRs là dải địa chỉ VPN của giao diện agent đang lắng nghe, để chủ
	// tài khoản xác nhận khi duyệt.
	AllowedCIDRs []string `json:"allowed_cidrs,omitempty"`
}

// EnrollStartResult là phản hồi của POST /v1/enroll/start (RFC 8628 §3.2).
type EnrollStartResult struct {
	// DeviceCode chỉ agent giữ, không hiển thị cho người dùng.
	DeviceCode string `json:"device_code"`

	// UserCode có dạng XXXX-XXXX, người dùng nhập vào app để duyệt.
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`

	// ExpiresIn và Interval tính bằng giây.
	ExpiresIn int `json:"expires_in"`
	Interval  int `json:"interval"`
}

// EnrollPollRequest là thân của POST /v1/enroll/poll. Sig chứng minh agent giữ
// host key đã khai, ngữ cảnh ContextEnrollPoP.
type EnrollPollRequest struct {
	DeviceCode string `json:"device_code"`
	Sig        string `json:"sig"`
}

// EnrollPollResult là phản hồi khi máy đã được duyệt. Khi chưa xong, poll trả
// lỗi enroll.authorization_pending, enroll.slow_down, enroll.expired_token hoặc
// enroll.access_denied.
type EnrollPollResult struct {
	MachineID string `json:"machine_id"`

	// PolicyKeys là khoá ký snapshot quyền mà agent sẽ tin từ giờ.
	PolicyKeys []PolicyKey `json:"policy_keys"`
}

// EnrollInfo là phản hồi của GET /v1/enroll/{user_code} cho app đang duyệt.
type EnrollInfo struct {
	UserCode string `json:"user_code"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	OSUser   string `json:"os_user"`

	// RequestedIP là địa chỉ server thấy khi agent gọi enroll/start; có thể là
	// địa chỉ của proxy hoặc VPN.
	RequestedIP  string   `json:"requested_ip,omitempty"`
	AllowedCIDRs []string `json:"allowed_cidrs,omitempty"`

	// HostKeyFP là vân tay host key để người duyệt đối chiếu với màn hình máy.
	HostKeyFP   string `json:"host_key_fp"`
	RequestedAt int64  `json:"requested_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

// EnrollApproveRequest là thân của POST /v1/enroll/{user_code}/approve; phản hồi
// là Machine vừa tạo.
type EnrollApproveRequest struct {
	// Name là tên hiển thị của máy; vắng thì dùng hostname.
	Name string `json:"name,omitempty"`
}

// Machine là một máy (một cặp máy và user hệ điều hành) mà tài khoản có quyền.
type Machine struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	OSUser   string `json:"os_user"`

	// Addresses là các địa chỉ host:port agent lắng nghe, đã lọc theo AllowedCIDRs.
	Addresses []string `json:"addresses,omitempty"`
	HostKeyFP string   `json:"host_key_fp"`

	Online       bool `json:"online"`
	SessionCount int  `json:"session_count"`

	// LastSeenAt vắng khi server chưa từng thấy agent kết nối.
	LastSeenAt  int64  `json:"last_seen_at,omitempty"`
	PolicyState string `json:"policy_state"`

	AgentVersion      string       `json:"agent_version,omitempty"`
	AgentCapabilities []Capability `json:"agent_capabilities,omitempty"`
	AllowedCIDRs      []string     `json:"allowed_cidrs,omitempty"`

	// Role là vai trò của người gọi trên máy này.
	Role             string            `json:"role"`
	LocalRevocations []LocalRevocation `json:"local_revocations,omitempty"`
}

// MachineList là phản hồi của GET /v1/machines.
type MachineList struct {
	Machines []Machine `json:"machines,omitempty"`
}

// MachinePatch là thân của PATCH /v1/machines/{id}; phản hồi là Machine sau khi sửa.
type MachinePatch struct {
	Name string `json:"name,omitempty"`
}
