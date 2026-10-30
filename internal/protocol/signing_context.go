package protocol

// SigningContext tách miền chữ ký: mỗi loại chữ ký có một ngữ cảnh riêng, được
// ghép vào đầu dữ liệu trước khi ký (ngữ cảnh, byte 0x00, dữ liệu). Nhờ đó một
// chữ ký hợp lệ ở miền này không dùng lại được ở miền khác, kể cả khi hai miền
// dùng chung một khoá (host key vừa ký đăng nhập SSH vừa ký chứng minh sở hữu).
//
// Ngữ cảnh trung tính, không chứa tên sản phẩm, để đổi tên không làm mất hiệu
// lực chữ ký đã phát hành. Hậu tố /1 là phiên bản của chính ngữ cảnh.
type SigningContext string

const (
	// ContextPolicySnapshot ký payload của snapshot quyền (server ký, agent kiểm).
	ContextPolicySnapshot SigningContext = "policy-snapshot/1"

	// ContextPolicyKeyRollover ký chứng thực một khoá ký chính sách mới bằng khoá đang tin.
	ContextPolicyKeyRollover SigningContext = "policy-key-rollover/1"

	// ContextSyncAuth ký thử thách của kênh đồng bộ bằng host key của máy.
	ContextSyncAuth SigningContext = "sync-auth/1"

	// ContextEnrollPoP ký chứng minh sở hữu host key khi thêm máy.
	ContextEnrollPoP SigningContext = "enroll-pop/1"

	// ContextDeviceKeyPoP ký chứng minh sở hữu khi đăng ký khoá thiết bị.
	ContextDeviceKeyPoP SigningContext = "device-key-pop/1"

	// ContextReleaseKeyset và ContextReleaseManifest là namespace SSHSIG của
	// bộ khoá phát hành và manifest bản phát hành.
	ContextReleaseKeyset   SigningContext = "release-keyset/1"
	ContextReleaseManifest SigningContext = "release-manifest/1"
)
