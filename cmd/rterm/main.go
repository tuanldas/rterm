// Lệnh rterm là agent và CLI của người dùng.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tuanldas/rterm/internal/product"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run tách khỏi main để gọi được từ test và trả mã thoát thay vì gọi os.Exit
// giữa chừng (làm bỏ qua defer).
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		if _, err := fmt.Fprintln(stdout, product.Version); err != nil {
			return 1
		}
		return 0
	}
	// Lỗi ghi ra stderr không còn kênh nào báo tiếp, bỏ qua có chủ ý.
	_, _ = fmt.Fprintf(stderr, "cách dùng: %s version\n", product.AgentBinary)
	return 2
}
