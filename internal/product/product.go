// Package product gom tên sản phẩm và mọi giá trị suy ra từ tên đó vào một chỗ,
// để đổi tên sản phẩm chỉ phải sửa gói này. Gói phục vụ mã Go (agent và CLI);
// server (Python) và app (Dart) khai báo tên ở nơi riêng của chúng. Cả ba cùng
// đối chiếu với product.json ở gốc repo bằng test, nên đổi tên mà sót chỗ nào
// thì test của chỗ đó đỏ.
//
// Định danh trên dây (tên method, capability, biến môi trường mang mã phiên,
// ngữ cảnh chữ ký) KHÔNG lấy từ đây. Chúng thuộc hợp đồng giao thức, phải sống
// qua lần đổi tên vì client và agent cũ vẫn đang chạy; xem gói protocol.
package product

import "slices"

const (
	// Name là tên sản phẩm hiển thị cho người dùng.
	Name = "rterm"

	// AgentBinary là tên tệp chạy của agent và CLI.
	AgentBinary = "rterm"

	// ServerImage là tên image Docker của server tài khoản, cũng là tên lệnh
	// bên trong image. Server chỉ phát hành dạng image Docker và không viết bằng
	// Go, nên không có tệp chạy server nào ở đây; hằng này giữ để mọi giá trị
	// gắn với tên sản phẩm nằm cùng chỗ.
	ServerImage = "rterm-server"

	// AppID là định danh gói của app Android.
	AppID = "com.tuanldas.rterm"

	// DefaultAgentPort là cổng SSH mặc định của agent. Khác cổng 22 để chạy
	// song song với sshd có sẵn trên cùng máy. Đổi được bằng cấu hình.
	DefaultAgentPort = 7022

	// DefaultServerPort là cổng HTTPS mặc định của server tài khoản. Đổi được
	// bằng cấu hình.
	DefaultServerPort = 7443

	// EnvPrefix là tiền tố biến môi trường cấu hình của server.
	EnvPrefix = "RTERM_"

	// TokenPrefix là tiền tố token do server cấp. Chỉ để công cụ quét bí mật
	// nhận ra token bị lộ; client coi token là chuỗi không trong suốt.
	TokenPrefix = "rt_"
)

// Version là phiên bản bản build, gán lúc build bằng
// -ldflags "-X github.com/tuanldas/rterm/internal/product.Version=<giá trị>".
// Phải là biến chứ không phải hằng thì cờ -X mới ghi đè được.
var Version = "dev"

// legacyNames là các tên sản phẩm đã dùng trước đây. Khi đổi tên, thêm tên cũ
// vào đây để daemon vẫn tìm lại được thư mục trạng thái, thư mục chạy và pipe
// của holder đang sống dưới tên cũ thay vì bỏ rơi chúng.
var legacyNames []string

// StateDirName là tên thư mục trạng thái của agent trong thư mục dữ liệu người dùng.
func StateDirName() string { return Name }

// URLScheme là scheme của deep link (lời mời, thiết lập server).
func URLScheme() string { return Name }

// LegacyNames trả các tên sản phẩm cũ, rỗng khi chưa từng đổi tên. Mỗi lần gọi
// trả một bản sao nên người gọi sửa kết quả không ảnh hưởng lần gọi sau.
func LegacyNames() []string { return slices.Clone(legacyNames) }
