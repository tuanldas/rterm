package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
)

// Conn là vận chuyển theo thông điệp: mỗi Read/Write là đúng một thông điệp
// JSON. NDJSON trên luồng byte và WebSocket đều đáp ứng được.
//
// Read chỉ được gọi từ một goroutine; Write phải an toàn khi gọi đồng thời.
// Cả hai phải trả về khi ctx bị huỷ hoặc khi Close được gọi; Close phải gọi
// nhiều lần được.
type Conn interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, msg []byte) error
	Close() error
}

// DefaultMaxInflight là số yêu cầu xử lý đồng thời tối đa khi nơi dùng không
// chỉ định. Chỉ là lưới an toàn; giới hạn thật truyền vào từ gói limits.
const DefaultMaxInflight = 64

// PeerOption chỉnh Peer lúc tạo.
type PeerOption func(*peerConfig)

type peerConfig struct {
	maxInflight int
	log         *slog.Logger
}

// WithMaxInflight đặt số yêu cầu xử lý đồng thời tối đa, đồng thời là độ dài
// hàng đợi notification. Yêu cầu vượt trần nhận CodeServerBusy; notification
// vượt trần bị bỏ.
func WithMaxInflight(n int) PeerOption {
	return func(c *peerConfig) {
		if n > 0 {
			c.maxInflight = n
		}
	}
}

// WithLogger đặt logger. Mặc định Peer không ghi log.
func WithLogger(l *slog.Logger) PeerOption {
	return func(c *peerConfig) {
		if l != nil {
			c.log = l
		}
	}
}

// Peer là một đầu của kết nối JSON-RPC hai chiều: vừa phục vụ yêu cầu của bên
// kia theo Router, vừa gọi method của bên kia.
//
// Vòng đọc không bao giờ chặn vì ghi hay vì handler: yêu cầu chạy trong
// goroutine riêng, notification chạy tuần tự trong một goroutine riêng, và
// phản hồi lỗi do vòng đọc sinh ra đi qua hàng đợi riêng. Nhờ vậy handler gọi
// ngược Call được mà không tự khoá chết, và hai Peer chung một pipe không có
// bộ đệm cũng không khoá nhau.
type Peer struct {
	conn   Conn
	router *Router
	cfg    peerConfig

	nextID  atomic.Uint64
	started atomic.Bool

	mu       sync.Mutex
	pending  map[uint64]chan callResult
	closed   bool
	closeErr error
	done     chan struct{} // đóng khi Peer dừng, đánh thức mọi Call đang chờ
}

type callResult struct {
	result json.RawMessage
	err    *Error
}

// NewPeer tạo Peer trên c. router có thể nil khi Peer chỉ đóng vai bên gọi.
func NewPeer(c Conn, r *Router, opts ...PeerOption) *Peer {
	cfg := peerConfig{
		maxInflight: DefaultMaxInflight,
		log:         slog.New(slog.DiscardHandler),
	}
	for _, o := range opts {
		o(&cfg)
	}
	return &Peer{
		conn:    c,
		router:  r,
		cfg:     cfg,
		pending: make(map[uint64]chan callResult),
		done:    make(chan struct{}),
	}
}

// message là khung JSON-RPC nhận vào: yêu cầu, notification hoặc phản hồi.
// Trường lạ trong khung bị bỏ qua.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type outRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type outNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// outResponse luôn mang "id" (null khi không xác định được yêu cầu gốc).
type outResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

const version = "2.0"

type notifyJob struct {
	method string
	h      HandlerFunc
	params json.RawMessage
}

// replyJob là phản hồi lỗi do chính vòng đọc sinh ra (method lạ, quá tải, JSON hỏng).
type replyJob struct {
	id  json.RawMessage
	err *Error
}

// Run đọc và phân phối thông điệp cho tới khi kết nối hỏng hoặc ctx bị huỷ, rồi
// đóng Conn. Trả ctx.Err() khi bị huỷ, ngược lại trả lỗi đọc đã bọc (kể cả
// io.EOF khi bên kia đóng). Run chờ mọi handler xong trước khi trả về nên
// không để lại goroutine; handler phải tôn trọng ctx.
func (p *Peer) Run(ctx context.Context) error {
	if !p.started.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Đánh thức Read đang chờ khi ctx huỷ, kể cả với Conn không tự tôn trọng ctx.
	stopWake := context.AfterFunc(runCtx, func() { _ = p.conn.Close() })
	defer stopWake()

	var wg sync.WaitGroup
	notifyCh := make(chan notifyJob, p.cfg.maxInflight)
	replyCh := make(chan replyJob, p.cfg.maxInflight)
	sem := make(chan struct{}, p.cfg.maxInflight)
	wg.Go(func() { p.notificationWorker(runCtx, notifyCh) })
	wg.Go(func() { p.replyWorker(runCtx, replyCh) })

	var cause error
	for {
		raw, err := p.conn.Read(runCtx)
		if err != nil {
			cause = err
			break
		}
		p.dispatch(runCtx, raw, &wg, sem, notifyCh, replyCh)
	}
	if err := ctx.Err(); err != nil {
		cause = err
	} else {
		cause = fmt.Errorf("đọc từ kết nối JSON-RPC: %w", cause)
	}

	p.shutdown(cause)
	cancel()
	close(notifyCh)
	close(replyCh)
	// Đóng trước khi chờ để handler đang kẹt ở Write thoát ngay.
	_ = p.conn.Close()
	wg.Wait()
	return cause
}

func (p *Peer) dispatch(ctx context.Context, raw []byte, wg *sync.WaitGroup, sem chan struct{}, notifyCh chan<- notifyJob, replyCh chan<- replyJob) {
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		p.enqueueReply(replyCh, nil, NewError(CodeInvalidRequest, "không hỗ trợ batch"))
		return
	}
	var m message
	if err := json.Unmarshal(raw, &m); err != nil {
		p.enqueueReply(replyCh, nil, NewError(CodeParseError, "JSON sai cú pháp"))
		return
	}

	switch {
	case m.Method != "":
		if m.JSONRPC != version {
			p.enqueueReply(replyCh, m.ID, NewError(CodeInvalidRequest, `thiếu hoặc sai trường "jsonrpc": cần "2.0"`))
			return
		}
		p.dispatchRequest(ctx, m, wg, sem, notifyCh, replyCh)
	case m.ID != nil && (m.Result != nil || m.Error != nil):
		// Không trả lỗi cho một phản hồi hỏng: id của nó do ta cấp cho lời gọi của
		// ta, nằm trong không gian id của bên kia, nên lặp lại nó trong một lỗi có
		// thể bị ghép nhầm vào lời gọi khác của họ.
		if m.JSONRPC != version {
			p.cfg.log.Warn(`bỏ phản hồi thiếu hoặc sai trường "jsonrpc"`, "id", string(m.ID))
			return
		}
		p.deliver(m)
	default:
		p.enqueueReply(replyCh, nil, NewError(CodeInvalidRequest, "thông điệp không phải yêu cầu cũng không phải phản hồi"))
	}
}

func (p *Peer) dispatchRequest(ctx context.Context, m message, wg *sync.WaitGroup, sem chan struct{}, notifyCh chan<- notifyJob, replyCh chan<- replyJob) {
	h, found := p.router.lookup(m.Method)

	if m.ID == nil { // notification: không có phản hồi dù thành công hay lỗi
		if !found {
			return
		}
		select {
		case notifyCh <- notifyJob{method: m.Method, h: h, params: m.Params}:
		default:
			// Notification chỉ là best-effort; trạng thái không được mất thì đi bằng
			// request có xác nhận. Chặn vòng đọc ở đây sẽ khoá chết khi handler đang
			// chờ phản hồi mà phản hồi nằm sau chúng trong luồng.
			p.cfg.log.Warn("bỏ notification vì hàng đợi đầy", "method", m.Method)
		}
		return
	}

	if !found {
		p.enqueueReply(replyCh, m.ID, NewError(CodeMethodNotFound, "method không tồn tại: "+m.Method))
		return
	}
	select {
	case sem <- struct{}{}:
	default:
		p.enqueueReply(replyCh, m.ID, NewError(CodeServerBusy, "quá nhiều yêu cầu đang xử lý đồng thời"))
		return
	}
	wg.Go(func() {
		defer func() { <-sem }()
		p.handleRequest(ctx, h, m)
	})
}

// enqueueReply không chặn: hàng đợi đầy nghĩa là bên kia đang spam yêu cầu hỏng,
// bỏ phản hồi lỗi chỉ làm lời gọi của họ chờ tới hạn chót của chính họ.
func (p *Peer) enqueueReply(ch chan<- replyJob, id json.RawMessage, e *Error) {
	select {
	case ch <- replyJob{id: id, err: e}:
	default:
		p.cfg.log.Warn("bỏ phản hồi lỗi vì hàng đợi đầy", "code", e.Code)
	}
}

func (p *Peer) replyWorker(ctx context.Context, jobs <-chan replyJob) {
	for j := range jobs {
		if ctx.Err() != nil {
			continue // dừng: chỉ rút cạn hàng đợi
		}
		p.writeError(ctx, j.id, j.err)
	}
}

func (p *Peer) notificationWorker(ctx context.Context, jobs <-chan notifyJob) {
	for j := range jobs {
		if ctx.Err() != nil {
			continue
		}
		if _, err := p.invoke(ctx, j.h, j.method, j.params); err != nil {
			p.cfg.log.Warn("xử lý notification lỗi", "method", j.method, "error", err)
		}
	}
}

// invoke gọi handler và biến panic thành lỗi nội bộ: một handler hỏng không
// được làm chết cả daemon.
func (p *Peer) invoke(ctx context.Context, h HandlerFunc, method string, params json.RawMessage) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			p.cfg.log.Error("handler panic", "method", method, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			result, err = nil, NewError(CodeInternalError, "lỗi nội bộ")
		}
	}()
	return h(ctx, params)
}

func (p *Peer) handleRequest(ctx context.Context, h HandlerFunc, m message) {
	result, err := p.invoke(ctx, h, m.Method, m.Params)
	if err != nil {
		p.writeError(ctx, m.ID, p.toRPCError(m.Method, err))
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		p.cfg.log.Error("không mã hoá được kết quả", "method", m.Method, "error", err)
		p.writeError(ctx, m.ID, NewError(CodeInternalError, "lỗi nội bộ"))
		return
	}
	err = p.send(ctx, outResponse{JSONRPC: version, ID: m.ID, Result: raw})
	if errors.Is(err, ErrMessageTooLarge) {
		// Bên gọi đang chờ; im lặng sẽ làm họ treo tới hạn chót.
		p.writeError(ctx, m.ID, NewError(CodeInternalError, "kết quả vượt giới hạn kích thước"))
		return
	}
	if err != nil {
		p.cfg.log.Debug("không gửi được phản hồi", "method", m.Method, "error", err)
	}
}

// toRPCError giữ nguyên *Error của handler, che mọi lỗi khác để không lộ chi tiết nội bộ.
func (p *Peer) toRPCError(method string, err error) *Error {
	var re *Error
	if errors.As(err, &re) {
		return re
	}
	p.cfg.log.Error("handler trả lỗi không có mã JSON-RPC", "method", method, "error", err)
	return NewError(CodeInternalError, "lỗi nội bộ")
}

func (p *Peer) writeError(ctx context.Context, id json.RawMessage, e *Error) {
	if err := p.send(ctx, outResponse{JSONRPC: version, ID: id, Error: e}); err != nil {
		p.cfg.log.Debug("không gửi được phản hồi lỗi", "code", e.Code, "error", err)
	}
}

func (p *Peer) send(ctx context.Context, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.conn.Write(ctx, b)
}

// deliver trao phản hồi cho Call đang chờ. Không có ai chờ (Call đã hết hạn)
// thì bỏ phản hồi muộn.
func (p *Peer) deliver(m message) {
	id, err := strconv.ParseUint(string(bytes.TrimSpace(m.ID)), 10, 64)
	if err != nil {
		p.cfg.log.Warn("phản hồi có id không phải số do Peer này cấp", "id", string(m.ID))
		return
	}
	p.mu.Lock()
	ch, ok := p.pending[id]
	p.mu.Unlock()
	if !ok {
		p.cfg.log.Debug("bỏ phản hồi muộn", "id", id)
		return
	}
	select {
	case ch <- callResult{result: m.Result, err: m.Error}:
	default: // đã có phản hồi cho id này, phản hồi trùng bị bỏ
	}
}

// Call gửi yêu cầu và chờ phản hồi. Lỗi từ bên kia trả về dạng *Error (dùng
// errors.As hoặc CodeOf); kết nối đóng giữa chừng trả lỗi bọc ErrClosed; ctx hết
// hạn trả ctx.Err(). result có thể nil khi không cần kết quả.
//
// ctx cũng giới hạn lúc ghi yêu cầu. Nếu ctx hết hạn khi thông điệp đang ghi dở
// vì bên kia không đọc (ghi nghẽn), cả kết nối bị đóng (với NDJSON là
// ndjsonConn.Write) và mọi lời gọi khác trên Peer nhận lỗi bọc ErrClosed. Đây là
// chủ đích: ghi dở làm hỏng khung nên kết nối không dùng tiếp được, và trên luồng
// byte không có hạn chót ghi thì đóng là cách duy nhất gỡ lần ghi đang chặn. ctx
// hết hạn khi còn chờ tới lượt ghi thì chỉ lời gọi này lỗi, kết nối giữ nguyên.
func (p *Peer) Call(ctx context.Context, method string, params, result any) error {
	if method == "" {
		return errors.New("tên method rỗng")
	}
	rawParams, err := encodeParams(params)
	if err != nil {
		return fmt.Errorf("mã hoá tham số của %s: %w", method, err)
	}

	id := p.nextID.Add(1)
	ch := make(chan callResult, 1)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return p.closedError()
	}
	p.pending[id] = ch
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
	}()

	if err := p.send(ctx, outRequest{JSONRPC: version, ID: id, Method: method, Params: rawParams}); err != nil {
		return p.sendError("gửi "+method, err)
	}

	select {
	case res := <-ch:
		return finishCall(method, res, result)
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		// Phản hồi có thể đã về ngay trước lúc đóng; ưu tiên nó.
		select {
		case res := <-ch:
			return finishCall(method, res, result)
		default:
		}
		return p.closedError()
	}
}

func finishCall(method string, res callResult, result any) error {
	if res.err != nil {
		return res.err
	}
	if result == nil || len(res.result) == 0 {
		return nil
	}
	if err := json.Unmarshal(res.result, result); err != nil {
		return fmt.Errorf("giải mã kết quả của %s: %w", method, err)
	}
	return nil
}

// Notify gửi notification: không chờ và không có phản hồi.
//
// Như Call, ctx hết hạn khi thông điệp đang ghi dở vì ghi nghẽn thì cả kết nối
// bị đóng, có chủ đích: khung đã hỏng thì không gửi tiếp được gì.
func (p *Peer) Notify(ctx context.Context, method string, params any) error {
	if method == "" {
		return errors.New("tên method rỗng")
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return p.closedError()
	}
	rawParams, err := encodeParams(params)
	if err != nil {
		return fmt.Errorf("mã hoá tham số của %s: %w", method, err)
	}
	if err := p.send(ctx, outNotification{JSONRPC: version, Method: method, Params: rawParams}); err != nil {
		return p.sendError("gửi notification "+method, err)
	}
	return nil
}

// sendError báo lỗi ghi; nếu Peer đã dừng thì trả lỗi đóng để người gọi chỉ cần
// kiểm một dấu hiệu (ErrClosed) cho cả hai đường: ghi hỏng và chờ phản hồi.
func (p *Peer) sendError(what string, err error) error {
	select {
	case <-p.done:
		return p.closedError()
	default:
		return fmt.Errorf("%s: %w", what, err)
	}
}

func encodeParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	return json.Marshal(params)
}

// shutdown đánh dấu Peer đã dừng và đánh thức mọi Call đang chờ.
func (p *Peer) shutdown(cause error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.closeErr = cause
	close(p.done)
}

func (p *Peer) closedError() error {
	p.mu.Lock()
	cause := p.closeErr
	p.mu.Unlock()
	if cause == nil {
		return ErrClosed
	}
	return fmt.Errorf("%w: %w", ErrClosed, cause)
}
