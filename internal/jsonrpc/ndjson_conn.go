package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// DefaultMaxMessage là giới hạn kích thước một thông điệp khi người tạo kết nối
// không chỉ định. Giới hạn thật do nơi dùng truyền vào từ gói limits.
const DefaultMaxMessage = 1 << 20

// ndjsonConn đóng khung mỗi thông điệp JSON thành một dòng kết thúc bằng '\n'
// trên một luồng byte (kênh SSH, socket, pipe).
type ndjsonConn struct {
	rwc    io.ReadWriteCloser
	br     *bufio.Reader
	maxMsg int

	// rerr dính sau lần đọc lỗi đầu tiên: dòng quá cỡ làm lệch khung nên
	// không thể đọc tiếp an toàn. Chỉ goroutine đọc chạm tới.
	rerr error

	// wsem là khoá một chỗ, nhận được cả ctx huỷ (mutex thường thì không).
	// werr dính sau lần ghi lỗi đầu tiên vì ghi dở làm hỏng khung; chỉ đọc/ghi
	// khi đang giữ wsem.
	wsem chan struct{}
	werr error

	closeOnce sync.Once
	closeErr  error
}

// NewNDJSONConn bọc rwc thành Conn đóng khung NDJSON. Mỗi thông điệp dài tối
// đa maxMsg byte (không tính '\n'); maxMsg không dương thì dùng
// DefaultMaxMessage. Không bao giờ để đọc không giới hạn: kẻ gửi dòng không có
// '\n' sẽ làm cạn bộ nhớ nếu không có trần.
//
// Read chỉ được gọi từ một goroutine; Write gọi đồng thời được.
func NewNDJSONConn(rwc io.ReadWriteCloser, maxMsg int) Conn {
	if maxMsg <= 0 {
		maxMsg = DefaultMaxMessage
	}
	return &ndjsonConn{
		rwc:    rwc,
		br:     bufio.NewReaderSize(rwc, min(maxMsg+1, 64<<10)),
		maxMsg: maxMsg,
		wsem:   make(chan struct{}, 1),
	}
}

// Read trả thông điệp kế tiếp (không kèm '\n'), bỏ qua dòng trống. ctx huỷ thì
// đóng kết nối để đánh thức lần đọc đang chờ: luồng byte tổng quát không có
// hạn chót đọc, đóng là cách duy nhất không rò goroutine.
func (c *ndjsonConn) Read(ctx context.Context) ([]byte, error) {
	if c.rerr != nil {
		return nil, c.rerr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	line, err := c.readLine()
	stop()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		c.rerr = err
		return nil, err
	}
	return line, nil
}

func (c *ndjsonConn) readLine() ([]byte, error) {
	var line []byte
	for {
		chunk, err := c.br.ReadSlice('\n')
		hasNewline := err == nil
		n := len(chunk)
		if hasNewline {
			n--
		}
		// Kiểm trước khi nối: không bao giờ giữ nhiều hơn maxMsg byte.
		if len(line)+n > c.maxMsg {
			return nil, fmt.Errorf("%w: dòng dài hơn %d byte", ErrMessageTooLarge, c.maxMsg)
		}
		line = append(line, chunk[:n]...)

		switch {
		case hasNewline:
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if len(bytes.TrimSpace(line)) == 0 {
				line = nil
				continue
			}
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(bytes.TrimSpace(line)) > 0:
			// Dòng cuối không có '\n' vẫn là một thông điệp; lần đọc sau trả EOF.
			return line, nil
		default:
			return nil, err
		}
	}
}

// Write gửi một thông điệp thành một dòng. ctx huỷ giữa lúc ghi dở thì đóng
// kết nối vì luồng không còn khung sạch; ctx huỷ trước khi bắt đầu ghi thì
// không đụng tới kết nối.
func (c *ndjsonConn) Write(ctx context.Context, msg []byte) error {
	if len(msg) > c.maxMsg {
		return fmt.Errorf("%w: %d byte, tối đa %d", ErrMessageTooLarge, len(msg), c.maxMsg)
	}
	if bytes.IndexByte(msg, '\n') >= 0 {
		return fmt.Errorf("%w: có ký tự xuống dòng làm vỡ khung NDJSON", ErrInvalidMessage)
	}

	select {
	case c.wsem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.wsem }()

	if c.werr != nil {
		return c.werr
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()

	// Một lần Write cho cả dòng: tách '\n' ra ghi riêng sẽ cho phép ghi dở.
	buf := make([]byte, 0, len(msg)+1)
	buf = append(buf, msg...)
	buf = append(buf, '\n')
	if _, err := c.rwc.Write(buf); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		c.werr = err
		return err
	}
	return nil
}

// Close đóng luồng bên dưới, gọi nhiều lần an toàn.
func (c *ndjsonConn) Close() error {
	c.closeOnce.Do(func() { c.closeErr = c.rwc.Close() })
	return c.closeErr
}
