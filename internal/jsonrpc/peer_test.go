package jsonrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tuanldas/rterm/internal/jsonrpc"
)

const (
	testTimeout = 5 * time.Second
	bufSize     = 64 << 10
)

func ctxWithTimeout(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}

// side mô tả một đầu của cặp Peer trong test.
type side struct {
	router *jsonrpc.Router
	opts   []jsonrpc.PeerOption
}

type peerPair struct {
	a, b *jsonrpc.Peer
}

// newPair nối hai Peer qua net.Pipe (không bộ đệm, nên lộ mọi lỗi khoá chết giữa
// hai vòng đọc), chạy Run ở hai goroutine và dọn khi test xong.
func newPair(t *testing.T, a, b side) peerPair {
	t.Helper()
	ca, cb := net.Pipe()
	pa := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, bufSize), a.router, a.opts...)
	pb := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(cb, bufSize), b.router, b.opts...)
	runPeer(t, pa)
	runPeer(t, pb)
	return peerPair{a: pa, b: pb}
}

// runPeer chạy Run và, khi test kết thúc, huỷ rồi kiểm Run thật sự trả về.
func runPeer(t *testing.T, p *jsonrpc.Peer) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(testTimeout):
			t.Error("Peer.Run không trả về sau khi huỷ ctx")
		}
	})
}

// newRaw dựng một Peer và trả đầu kia dưới dạng Conn thô để test đọc/ghi từng dòng JSON.
func newRaw(t *testing.T, r *jsonrpc.Router, opts ...jsonrpc.PeerOption) jsonrpc.Conn {
	t.Helper()
	ca, cb := net.Pipe()
	runPeer(t, jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, bufSize), r, opts...))
	raw := jsonrpc.NewNDJSONConn(cb, bufSize)
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

func writeRaw(t *testing.T, c jsonrpc.Conn, s string) {
	t.Helper()
	if err := c.Write(ctxWithTimeout(t), []byte(s)); err != nil {
		t.Fatalf("ghi %q: %v", s, err)
	}
}

func readRaw(t *testing.T, c jsonrpc.Conn) map[string]json.RawMessage {
	t.Helper()
	b, err := c.Read(ctxWithTimeout(t))
	if err != nil {
		t.Fatalf("đọc: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("phản hồi không phải JSON object: %q", b)
	}
	return m
}

func errCode(t *testing.T, m map[string]json.RawMessage) int {
	t.Helper()
	var e jsonrpc.Error
	if err := json.Unmarshal(m["error"], &e); err != nil {
		t.Fatalf("phản hồi không có error: %v", m)
	}
	return e.Code
}

func echoHandler(_ context.Context, params json.RawMessage) (any, error) {
	var v any
	if err := jsonrpc.DecodeParams(params, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func TestPhuongThucChuaBietTraVeMethodNotFound(t *testing.T) {
	server := jsonrpc.NewRouter()
	server.Handle("ping", func(context.Context, json.RawMessage) (any, error) { return "pong", nil })
	p := newPair(t, side{}, side{router: server})
	ctx := ctxWithTimeout(t)

	err := p.a.Call(ctx, "khong.ton.tai", nil, nil)

	var re *jsonrpc.Error
	if !errors.As(err, &re) || re.Code != jsonrpc.CodeMethodNotFound {
		t.Fatalf("method lạ phải trả lỗi %d, nhận được %v", jsonrpc.CodeMethodNotFound, err)
	}
	if !jsonrpc.IsMethodNotFound(err) {
		t.Error("IsMethodNotFound phải nhận ra lỗi này")
	}

	// Method lạ chỉ là "không hỗ trợ": phần còn lại của kết nối vẫn dùng được.
	var out string
	if err := p.a.Call(ctx, "ping", nil, &out); err != nil || out != "pong" {
		t.Fatalf("kết nối phải còn dùng được sau lỗi method lạ: out=%q err=%v", out, err)
	}
}

func TestNotificationMethodLaBiBoImLang(t *testing.T) {
	raw := newRaw(t, jsonrpc.NewRouter())

	writeRaw(t, raw, `{"jsonrpc":"2.0","method":"khong.ton.tai"}`)
	writeRaw(t, raw, `{"jsonrpc":"2.0","id":7,"method":"cung.khong.ton.tai"}`)

	// Phản hồi đầu tiên phải là của yêu cầu id 7: notification không được có phản hồi.
	m := readRaw(t, raw)
	if string(m["id"]) != "7" || errCode(t, m) != jsonrpc.CodeMethodNotFound {
		t.Fatalf("phản hồi đầu tiên phải là lỗi method-not-found của id 7, nhận được %v", m)
	}
}

func TestGoiHaiChieuGhepDungIDVoiKetQua(t *testing.T) {
	double := func(_ context.Context, params json.RawMessage) (any, error) {
		var n int
		if err := jsonrpc.DecodeParams(params, &n); err != nil {
			return nil, err
		}
		// Ngủ ngược theo n để phản hồi về sai thứ tự gửi.
		time.Sleep(time.Duration(100-n%100) * 20 * time.Microsecond)
		return n * 2, nil
	}
	ra, rb := jsonrpc.NewRouter(), jsonrpc.NewRouter()
	ra.Handle("nhan_doi", double)
	rb.Handle("nhan_doi", double)
	// Trần đồng thời phải lớn hơn số lời gọi mỗi chiều; vượt trần là hành vi khác, có test riêng.
	many := []jsonrpc.PeerOption{jsonrpc.WithMaxInflight(512)}
	p := newPair(t, side{router: ra, opts: many}, side{router: rb, opts: many})
	ctx := ctxWithTimeout(t)

	errs := make(chan error, 400)
	var wg sync.WaitGroup
	for i := range 100 {
		for ten, peer := range map[string]*jsonrpc.Peer{"a": p.a, "b": p.b} {
			wg.Go(func() {
				var got int
				if err := peer.Call(ctx, "nhan_doi", i, &got); err != nil {
					errs <- fmt.Errorf("peer %s gọi %d: %w", ten, i, err)
				} else if got != 2*i {
					errs <- fmt.Errorf("peer %s gọi %d nhận %d, cần %d: kết quả ghép nhầm id", ten, i, got, 2*i)
				}
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestCallTraLoiKhiKetNoiDong(t *testing.T) {
	received := make(chan struct{})
	r := jsonrpc.NewRouter()
	r.Handle("treo", func(ctx context.Context, _ json.RawMessage) (any, error) {
		close(received)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ca, cb := net.Pipe()
	client := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, bufSize), nil)
	server := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(cb, bufSize), r)
	runPeer(t, client)
	runPeer(t, server)

	// Bên server cắt kết nối khi yêu cầu đã tới tay handler.
	go func() {
		<-received
		_ = cb.Close()
	}()

	start := time.Now()
	err := client.Call(ctxWithTimeout(t), "treo", nil, nil)

	if !errors.Is(err, jsonrpc.ErrClosed) {
		t.Fatalf("Call đang chờ phải trả ErrClosed khi kết nối đóng, nhận được: %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Call mất %v mới biết kết nối đã đóng, phải được đánh thức ngay", d)
	}
	if err := client.Call(ctxWithTimeout(t), "bat.ky", nil, nil); !errors.Is(err, jsonrpc.ErrClosed) {
		t.Errorf("Call sau khi đóng phải trả ErrClosed ngay, nhận được: %v", err)
	}
	if err := client.Notify(ctxWithTimeout(t), "bat.ky", nil); !errors.Is(err, jsonrpc.ErrClosed) {
		t.Errorf("Notify sau khi đóng phải trả ErrClosed, nhận được: %v", err)
	}
}

func TestThongDiepQuaLonBiTuChoi(t *testing.T) {
	const limit = 64

	t.Run("đọc dòng quá cỡ làm Run dừng chứ không phình bộ nhớ", func(t *testing.T) {
		ca, cb := net.Pipe()
		peer := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, limit), jsonrpc.NewRouter())
		done := make(chan error, 1)
		go func() { done <- peer.Run(t.Context()) }()
		go func() {
			_, _ = cb.Write(append(bytes.Repeat([]byte("x"), 100_000), '\n'))
			_ = cb.Close()
		}()

		select {
		case err := <-done:
			if !errors.Is(err, jsonrpc.ErrMessageTooLarge) {
				t.Fatalf("Run phải dừng với ErrMessageTooLarge, nhận được: %v", err)
			}
		case <-time.After(testTimeout):
			t.Fatal("Run không từ chối dòng quá cỡ")
		}
	})

	t.Run("ghi thông điệp quá cỡ bị từ chối trước khi chạm kết nối", func(t *testing.T) {
		ca, cb := net.Pipe()
		t.Cleanup(func() { _ = ca.Close(); _ = cb.Close() })
		c := jsonrpc.NewNDJSONConn(ca, limit)

		err := c.Write(ctxWithTimeout(t), bytes.Repeat([]byte("x"), limit+1))

		if !errors.Is(err, jsonrpc.ErrMessageTooLarge) {
			t.Fatalf("cần ErrMessageTooLarge, nhận được: %v", err)
		}
	})

	t.Run("thông điệp đúng bằng giới hạn vẫn đi qua", func(t *testing.T) {
		ca, cb := net.Pipe()
		t.Cleanup(func() { _ = ca.Close(); _ = cb.Close() })
		w, r := jsonrpc.NewNDJSONConn(ca, limit), jsonrpc.NewNDJSONConn(cb, limit)
		msg := bytes.Repeat([]byte("y"), limit)

		go func() { _ = w.Write(t.Context(), msg) }()
		got, err := r.Read(ctxWithTimeout(t))

		if err != nil || !bytes.Equal(got, msg) {
			t.Fatalf("thông điệp %d byte phải qua nguyên vẹn: len=%d err=%v", limit, len(got), err)
		}
	})
}

func TestKetQuaQuaLonTraLoiLoiThayViTreo(t *testing.T) {
	r := jsonrpc.NewRouter()
	r.Handle("lon", func(context.Context, json.RawMessage) (any, error) { return strings.Repeat("z", 5000), nil })
	ca, cb := net.Pipe()
	client := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, bufSize), nil)
	server := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(cb, 1000), r) // trần của server nhỏ hơn kết quả
	runPeer(t, client)
	runPeer(t, server)

	err := client.Call(ctxWithTimeout(t), "lon", nil, nil)

	if code, ok := jsonrpc.CodeOf(err); !ok || code != jsonrpc.CodeInternalError {
		t.Fatalf("kết quả vượt trần phải về lỗi %d chứ không để bên gọi treo, nhận được: %v", jsonrpc.CodeInternalError, err)
	}
}

func TestHuyContextKhongRoGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()

	ca, cb := net.Pipe()
	r := jsonrpc.NewRouter()
	r.Handle("doi", func(ctx context.Context, _ json.RawMessage) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	pa := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, bufSize), r)
	pb := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(cb, bufSize), r)
	doneA, doneB := make(chan error, 1), make(chan error, 1)
	go func() { doneA <- pa.Run(ctx) }()
	go func() { doneB <- pb.Run(ctx) }()

	// Có yêu cầu đang xử lý và một Call đang chờ lúc huỷ.
	callDone := make(chan error, 1)
	go func() { callDone <- pa.Call(context.Background(), "doi", nil, nil) }()
	time.Sleep(50 * time.Millisecond)

	cancel()
	for _, c := range []chan error{doneA, doneB} {
		select {
		case err := <-c:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Run phải trả context.Canceled, nhận được: %v", err)
			}
		case <-time.After(testTimeout):
			t.Fatal("Run không trả về sau khi huỷ ctx")
		}
	}
	select {
	case err := <-callDone:
		if !errors.Is(err, jsonrpc.ErrClosed) {
			t.Errorf("Call đang chờ phải được đánh thức bằng ErrClosed, nhận được: %v", err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Call đang chờ không được đánh thức khi huỷ ctx")
	}

	until := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		buf := make([]byte, 1<<16)
		t.Fatalf("rò goroutine: trước %d, sau %d\n%s", before, after, buf[:runtime.Stack(buf, true)])
	}
}

func TestRunChiChayMotLan(t *testing.T) {
	ca, cb := net.Pipe()
	t.Cleanup(func() { _ = cb.Close() })
	p := jsonrpc.NewPeer(jsonrpc.NewNDJSONConn(ca, bufSize), nil)
	runPeer(t, p)
	time.Sleep(20 * time.Millisecond)

	if err := p.Run(t.Context()); !errors.Is(err, jsonrpc.ErrAlreadyRunning) {
		t.Fatalf("Run lần hai phải trả ErrAlreadyRunning, nhận được: %v", err)
	}
}

func TestNotificationDenHandlerVaGiuNguyenThuTu(t *testing.T) {
	const so = 100
	var mu sync.Mutex
	var order []int
	done := make(chan struct{})
	r := jsonrpc.NewRouter()
	r.Handle("ghi", func(_ context.Context, params json.RawMessage) (any, error) {
		var n int
		if err := jsonrpc.DecodeParams(params, &n); err != nil {
			return nil, err
		}
		mu.Lock()
		defer mu.Unlock()
		order = append(order, n)
		if len(order) == so {
			close(done)
		}
		return nil, nil
	})
	p := newPair(t, side{}, side{router: r, opts: []jsonrpc.PeerOption{jsonrpc.WithMaxInflight(4 * so)}})

	for i := range so {
		if err := p.a.Notify(ctxWithTimeout(t), "ghi", i); err != nil {
			t.Fatalf("Notify %d: %v", i, err)
		}
	}

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatalf("chỉ nhận được %d/%d notification", len(order), so)
	}
	mu.Lock()
	defer mu.Unlock()
	for i, n := range order {
		if n != i {
			t.Fatalf("notification đến sai thứ tự ở vị trí %d: %v", i, order)
		}
	}
}

func TestNotificationQuaTaiBiBoVaPeerSongTiep(t *testing.T) {
	const limit = 8
	var mu sync.Mutex
	got := 0
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	r := jsonrpc.NewRouter()
	r.Handle("cham", func(ctx context.Context, _ json.RawMessage) (any, error) {
		mu.Lock()
		got++
		mu.Unlock()
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, nil
	})
	r.Handle("ping", func(context.Context, json.RawMessage) (any, error) { return "pong", nil })
	p := newPair(t, side{}, side{router: r, opts: []jsonrpc.PeerOption{jsonrpc.WithMaxInflight(limit)}})

	for range 5 * limit {
		if err := p.a.Notify(ctxWithTimeout(t), "cham", nil); err != nil {
			t.Fatalf("Notify: %v", err)
		}
	}
	<-entered

	// Vòng đọc không được kẹt vì hàng đợi notification đầy: yêu cầu sau đó vẫn có phản hồi.
	var out string
	if err := p.a.Call(ctxWithTimeout(t), "ping", nil, &out); err != nil || out != "pong" {
		t.Fatalf("Call sau khi tràn notification phải còn chạy: out=%q err=%v", out, err)
	}
	close(release)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if got > limit+1 || got < 1 {
		t.Errorf("handler nhận %d notification, hàng đợi %d chỉ giữ tối đa %d", got, limit, limit+1)
	}
}

func TestLoiCuaHandlerDuocChuyenChoNguoiGoi(t *testing.T) {
	r := jsonrpc.NewRouter()
	r.Handle("xung.dot", func(context.Context, json.RawMessage) (any, error) {
		return nil, jsonrpc.NewErrorData(-32004, "xung đột", map[string]string{"reason": "stale"})
	})
	r.Handle("loi.noi.bo", func(context.Context, json.RawMessage) (any, error) {
		return nil, errors.New("mật khẩu db là hunter2")
	})
	p := newPair(t, side{}, side{router: r})
	ctx := ctxWithTimeout(t)

	err := p.a.Call(ctx, "xung.dot", nil, nil)
	var re *jsonrpc.Error
	if !errors.As(err, &re) || re.Code != -32004 || re.Message != "xung đột" {
		t.Fatalf("lỗi có mã phải đi nguyên vẹn, nhận được: %v", err)
	}
	var data struct{ Reason string }
	if err := json.Unmarshal(re.Data, &data); err != nil || data.Reason != "stale" {
		t.Errorf("data của lỗi bị mất: %s (%v)", re.Data, err)
	}

	err = p.a.Call(ctx, "loi.noi.bo", nil, nil)
	if code, ok := jsonrpc.CodeOf(err); !ok || code != jsonrpc.CodeInternalError {
		t.Fatalf("lỗi thường phải thành %d, nhận được: %v", jsonrpc.CodeInternalError, err)
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("chi tiết nội bộ bị lộ cho bên gọi: %v", err)
	}
}

func TestHandlerPanicKhongLamChetPeer(t *testing.T) {
	r := jsonrpc.NewRouter()
	r.Handle("no", func(context.Context, json.RawMessage) (any, error) { panic("lỗi lập trình") })
	r.Handle("ping", func(context.Context, json.RawMessage) (any, error) { return "pong", nil })
	p := newPair(t, side{}, side{router: r})
	ctx := ctxWithTimeout(t)

	err := p.a.Call(ctx, "no", nil, nil)
	if code, ok := jsonrpc.CodeOf(err); !ok || code != jsonrpc.CodeInternalError {
		t.Fatalf("panic phải thành lỗi nội bộ, nhận được: %v", err)
	}
	var out string
	if err := p.a.Call(ctx, "ping", nil, &out); err != nil || out != "pong" {
		t.Fatalf("Peer phải sống sau panic của handler: out=%q err=%v", out, err)
	}
}

func TestVuotGioiHanDongThoiTraLoiBan(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	r := jsonrpc.NewRouter()
	r.Handle("giu", func(ctx context.Context, _ json.RawMessage) (any, error) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return "xong", nil
	})
	p := newPair(t, side{}, side{router: r, opts: []jsonrpc.PeerOption{jsonrpc.WithMaxInflight(2)}})
	ctx := ctxWithTimeout(t)

	errs := make(chan error, 2)
	for range 2 {
		go func() { errs <- p.a.Call(ctx, "giu", nil, nil) }()
	}
	<-entered
	<-entered // cả hai chỗ đã bị chiếm

	err := p.a.Call(ctx, "giu", nil, nil)

	if code, ok := jsonrpc.CodeOf(err); !ok || code != jsonrpc.CodeServerBusy {
		t.Fatalf("yêu cầu vượt trần đồng thời phải nhận %d, nhận được: %v", jsonrpc.CodeServerBusy, err)
	}
	close(release)
	for range 2 {
		if err := <-errs; err != nil {
			t.Errorf("yêu cầu trong trần phải xong bình thường: %v", err)
		}
	}
}

func TestDongJSONHongTraLoiParseErrorVaGiuKetNoi(t *testing.T) {
	r := jsonrpc.NewRouter()
	r.Handle("echo", echoHandler)
	raw := newRaw(t, r)

	writeRaw(t, raw, `{đây không phải json`)
	m := readRaw(t, raw)
	if errCode(t, m) != jsonrpc.CodeParseError || string(m["id"]) != "null" {
		t.Fatalf("dòng hỏng phải nhận parse error với id null, nhận được %v", m)
	}

	writeRaw(t, raw, `{"jsonrpc":"2.0","id":1,"method":"echo","params":"con sống"}`)
	m = readRaw(t, raw)
	if string(m["result"]) != `"con sống"` {
		t.Fatalf("kết nối phải còn dùng được sau dòng hỏng, nhận được %v", m)
	}
}

func TestKhungJSONRPCCoTruongLaVanXuLyDuoc(t *testing.T) {
	r := jsonrpc.NewRouter()
	r.Handle("echo", echoHandler)
	raw := newRaw(t, r)

	writeRaw(t, raw, `{"jsonrpc":"2.0","id":3,"method":"echo","params":{"a":1},"x_tuong_lai":{"b":[1,2]}}`)

	m := readRaw(t, raw)
	if string(m["id"]) != "3" || string(m["result"]) != `{"a":1}` {
		t.Fatalf("trường lạ trong khung không được làm hỏng yêu cầu, nhận được %v", m)
	}
}

func TestKhungSaiPhienBanVaBatchBiTuChoiRoRang(t *testing.T) {
	raw := newRaw(t, jsonrpc.NewRouter())

	writeRaw(t, raw, `{"id":1,"method":"echo"}`)
	if m := readRaw(t, raw); errCode(t, m) != jsonrpc.CodeInvalidRequest {
		t.Errorf("thiếu jsonrpc phải nhận invalid request, nhận được %v", m)
	}

	writeRaw(t, raw, `[{"jsonrpc":"2.0","id":1,"method":"echo"}]`)
	if m := readRaw(t, raw); errCode(t, m) != jsonrpc.CodeInvalidRequest {
		t.Errorf("batch chưa hỗ trợ phải nhận invalid request, nhận được %v", m)
	}
}

func TestPhanHoiHongKhongBiDapLaiBangLoiMangIDCuaBenKia(t *testing.T) {
	r := jsonrpc.NewRouter()
	r.Handle("echo", echoHandler)
	raw := newRaw(t, r)

	// Phản hồi thiếu "jsonrpc" và một thông điệp vô nghĩa mang id: không được đáp lại
	// bằng lỗi mang id đó, vì id ấy không phải của yêu cầu nào bên kia gửi cho ta.
	writeRaw(t, raw, `{"id":5,"result":1}`)
	writeRaw(t, raw, `{"jsonrpc":"2.0","id":6}`)
	writeRaw(t, raw, `{"jsonrpc":"2.0","id":7,"method":"echo","params":"ok"}`)

	// Lỗi do vòng đọc sinh ra và phản hồi của handler đi từ hai goroutine khác nhau
	// nên thứ tự đến không cố định: chỉ kiểm tập phản hồi.
	got := map[string]map[string]json.RawMessage{}
	for range 2 {
		m := readRaw(t, raw)
		got[string(m["id"])] = m
	}
	if m, ok := got["null"]; !ok || errCode(t, m) != jsonrpc.CodeInvalidRequest {
		t.Errorf("thông điệp vô nghĩa phải nhận lỗi invalid request với id null, nhận được %v", got)
	}
	if m, ok := got["7"]; !ok || string(m["result"]) != `"ok"` {
		t.Errorf("yêu cầu id 7 phải có phản hồi, nhận được %v", got)
	}

	// Không còn phản hồi nào nữa (đặc biệt không có lỗi mang id 5 hay 6).
	short, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	if b, err := raw.Read(short); err == nil {
		t.Errorf("còn phản hồi thừa: %s", b)
	}
}

func TestThamSoCoTruongLaVanDuocNhan(t *testing.T) {
	type params struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	var got params
	err := jsonrpc.DecodeParams(json.RawMessage(`{"a":1,"b":"x","moi_o_ban_sau":{"c":[1]}}`), &got)
	if err != nil || got != (params{A: 1, B: "x"}) {
		t.Fatalf("trường lạ phải bị bỏ qua chứ không gây lỗi: got=%+v err=%v", got, err)
	}

	err = jsonrpc.DecodeParams(json.RawMessage(`{"a":"không phải số"}`), &got)
	if code, ok := jsonrpc.CodeOf(err); !ok || code != jsonrpc.CodeInvalidParams {
		t.Errorf("sai kiểu phải trả lỗi %d, nhận được: %v", jsonrpc.CodeInvalidParams, err)
	}
	err = jsonrpc.DecodeParams(json.RawMessage(`{hỏng`), &got)
	if code, ok := jsonrpc.CodeOf(err); !ok || code != jsonrpc.CodeInvalidParams {
		t.Errorf("JSON hỏng phải trả lỗi %d, nhận được: %v", jsonrpc.CodeInvalidParams, err)
	}

	got = params{A: 9}
	for _, empty := range []json.RawMessage{nil, json.RawMessage(`null`)} {
		if err := jsonrpc.DecodeParams(empty, &got); err != nil || got.A != 9 {
			t.Errorf("params vắng hoặc null phải giữ nguyên giá trị, got=%+v err=%v", got, err)
		}
	}
}

func TestRouterTuChoiDangKySai(t *testing.T) {
	ok := func(context.Context, json.RawMessage) (any, error) { return nil, nil }
	cases := map[string]func(r *jsonrpc.Router){
		"tên rỗng":          func(r *jsonrpc.Router) { r.Handle("", ok) },
		"handler nil":       func(r *jsonrpc.Router) { r.Handle("a", nil) },
		"tiền tố rpc.":      func(r *jsonrpc.Router) { r.Handle("rpc.x", ok) },
		"đăng ký trùng tên": func(r *jsonrpc.Router) { r.Handle("a", ok); r.Handle("a", ok) },
	}
	for ten, f := range cases {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("đăng ký sai phải panic để lộ ngay lúc khởi động")
				}
			}()
			f(jsonrpc.NewRouter())
		})
	}
}

func TestNDJSONBoQuaDongTrongVaChapNhanCRLF(t *testing.T) {
	ca, cb := net.Pipe()
	t.Cleanup(func() { _ = ca.Close(); _ = cb.Close() })
	r := jsonrpc.NewNDJSONConn(cb, bufSize)

	go func() { _, _ = io.WriteString(ca, "\n\r\n  \n{\"a\":1}\r\n{\"b\":2}\n") }()

	for _, want := range []string{`{"a":1}`, `{"b":2}`} {
		got, err := r.Read(ctxWithTimeout(t))
		if err != nil || string(got) != want {
			t.Fatalf("cần %q, nhận được %q (%v)", want, got, err)
		}
	}
}

func TestNDJSONDongCuoiKhongXuongDongVanDocDuocRoiGapEOF(t *testing.T) {
	ca, cb := net.Pipe()
	t.Cleanup(func() { _ = cb.Close() })
	r := jsonrpc.NewNDJSONConn(cb, bufSize)

	go func() { _, _ = io.WriteString(ca, `{"cuoi":true}`); _ = ca.Close() }()

	got, err := r.Read(ctxWithTimeout(t))
	if err != nil || string(got) != `{"cuoi":true}` {
		t.Fatalf("dòng cuối không có xuống dòng vẫn là một thông điệp, nhận được %q (%v)", got, err)
	}
	if _, err := r.Read(ctxWithTimeout(t)); !errors.Is(err, io.EOF) {
		t.Errorf("lần đọc sau phải là io.EOF, nhận được: %v", err)
	}
}

func TestNDJSONTuChoiGhiThongDiepChuaXuongDong(t *testing.T) {
	ca, cb := net.Pipe()
	t.Cleanup(func() { _ = ca.Close(); _ = cb.Close() })
	c := jsonrpc.NewNDJSONConn(ca, bufSize)

	err := c.Write(ctxWithTimeout(t), []byte("{\"a\":\n1}"))

	if !errors.Is(err, jsonrpc.ErrInvalidMessage) {
		t.Fatalf("thông điệp có xuống dòng làm vỡ khung, phải trả ErrInvalidMessage, nhận được: %v", err)
	}
}

func TestNDJSONGhiDongThoiKhongTronThongDiep(t *testing.T) {
	const writes = 200
	ca, cb := net.Pipe()
	t.Cleanup(func() { _ = ca.Close(); _ = cb.Close() })
	w, r := jsonrpc.NewNDJSONConn(ca, bufSize), jsonrpc.NewNDJSONConn(cb, bufSize)

	for i := range writes {
		go func() {
			msg := fmt.Sprintf(`{"i":%d,"dem":"%s"}`, i, strings.Repeat("d", 300))
			_ = w.Write(t.Context(), []byte(msg))
		}()
	}

	seen := make(map[int]bool)
	for range writes {
		b, err := r.Read(ctxWithTimeout(t))
		if err != nil {
			t.Fatalf("đọc: %v", err)
		}
		var m struct {
			I   int    `json:"i"`
			Dem string `json:"dem"`
		}
		if err := json.Unmarshal(b, &m); err != nil || len(m.Dem) != 300 {
			t.Fatalf("thông điệp bị trộn byte với thông điệp khác: %q (%v)", b, err)
		}
		seen[m.I] = true
	}
	if len(seen) != writes {
		t.Errorf("chỉ thấy %d/%d thông điệp khác nhau", len(seen), writes)
	}
}

func TestNDJSONHuyContextDanhThucLanDocDangCho(t *testing.T) {
	ca, cb := net.Pipe()
	t.Cleanup(func() { _ = ca.Close() })
	r := jsonrpc.NewNDJSONConn(cb, bufSize)
	ctx, cancel := context.WithCancel(t.Context())

	res := make(chan error, 1)
	go func() { _, err := r.Read(ctx); res <- err }()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-res:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Read bị huỷ phải trả context.Canceled, nhận được: %v", err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Read không được đánh thức khi ctx bị huỷ")
	}
}
