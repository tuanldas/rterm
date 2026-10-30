package jsonrpc

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
)

// HandlerFunc xử lý một yêu cầu hoặc notification. Kết quả được mã hoá thành
// JSON; lỗi kiểu *Error giữ nguyên mã gửi về, lỗi khác bị che thành lỗi nội bộ.
// ctx bị huỷ khi Peer dừng, handler phải tôn trọng nó để Peer dừng được gọn.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

// Router ánh xạ tên method sang handler. Giá trị zero dùng được.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// NewRouter tạo router rỗng.
func NewRouter() *Router { return &Router{} }

// Handle đăng ký handler cho method. Đăng ký sai (tên rỗng, tiền tố rpc. mà
// JSON-RPC dành riêng, handler nil, trùng tên) là lỗi lập trình lộ ngay lúc khởi
// động nên panic, giống http.ServeMux.
func (r *Router) Handle(method string, h HandlerFunc) {
	switch {
	case method == "":
		panic("jsonrpc: tên method rỗng")
	case strings.HasPrefix(method, "rpc."):
		panic("jsonrpc: tiền tố rpc. dành riêng cho JSON-RPC, không đăng ký: " + method)
	case h == nil:
		panic("jsonrpc: handler nil cho method " + method)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.handlers[method]; dup {
		panic("jsonrpc: method đã đăng ký: " + method)
	}
	if r.handlers == nil {
		r.handlers = make(map[string]HandlerFunc)
	}
	r.handlers[method] = h
}

// lookup chịu được router nil: Peer chỉ đóng vai bên gọi không cần router.
func (r *Router) lookup(method string) (HandlerFunc, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[method]
	return h, ok
}
