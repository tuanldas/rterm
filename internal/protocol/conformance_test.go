package protocol_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tuanldas/rterm/internal/protocol"
)

// Hợp đồng nằm ở docs/protocol: examples/<miền>/<Def>.<biến thể>.json được kiểm
// với schema/<miền>.schema.json#/$defs/<Def>. Bộ test này giữ ba thứ khớp nhau:
// tài liệu (schema + ví dụ), kiểu Go, và quy tắc tương thích tiến.

const protocolDir = "../../docs/protocol"

func protocolPath(parts ...string) string {
	return filepath.Join(append([]string{protocolDir}, parts...)...)
}

// newValue ánh xạ <miền>/<Def> sang kiểu Go tương ứng. Thêm một thông điệp mới
// vào hợp đồng nghĩa là thêm dòng ở đây.
var newValue = map[string]func() any{
	"rpc/HelloParams":           func() any { return new(protocol.HelloParams) },
	"rpc/HelloResult":           func() any { return new(protocol.HelloResult) },
	"rpc/SessionInfo":           func() any { return new(protocol.SessionInfo) },
	"rpc/SizeInfo":              func() any { return new(protocol.SizeInfo) },
	"rpc/SessionListResult":     func() any { return new(protocol.SessionListResult) },
	"rpc/SessionCreateParams":   func() any { return new(protocol.SessionCreateParams) },
	"rpc/SessionCloseParams":    func() any { return new(protocol.SessionCloseParams) },
	"rpc/SessionSizeParams":     func() any { return new(protocol.SessionSizeParams) },
	"rpc/EventsSubscribeParams": func() any { return new(protocol.EventsSubscribeParams) },
	"rpc/EventsSubscribeResult": func() any { return new(protocol.EventsSubscribeResult) },
	"rpc/Event":                 func() any { return new(protocol.Event) },
	"rpc/SessionResizedData":    func() any { return new(protocol.SessionResizedData) },
	"rpc/ErrorData":             func() any { return new(protocol.ErrorData) },

	"sync/Applied":               func() any { return new(protocol.Applied) },
	"sync/SyncHello":             func() any { return new(protocol.SyncHello) },
	"sync/SyncHelloResult":       func() any { return new(protocol.SyncHelloResult) },
	"sync/SyncAuth":              func() any { return new(protocol.SyncAuth) },
	"sync/PolicySnapshot":        func() any { return new(protocol.PolicySnapshot) },
	"sync/SignedEnvelope":        func() any { return new(protocol.SignedEnvelope) },
	"sync/KeyEndorsement":        func() any { return new(protocol.KeyEndorsement) },
	"sync/MachineStatus":         func() any { return new(protocol.MachineStatus) },
	"sync/AuditReport":           func() any { return new(protocol.AuditReport) },
	"sync/AuditAck":              func() any { return new(protocol.AuditAck) },
	"sync/EmergencyVerifyParams": func() any { return new(protocol.EmergencyVerifyParams) },
	"sync/EmergencyVerifyResult": func() any { return new(protocol.EmergencyVerifyResult) },

	"http-api/Meta":                  func() any { return new(protocol.Meta) },
	"http-api/ErrorResponse":         func() any { return new(protocol.ErrorResponse) },
	"http-api/SetupRequest":          func() any { return new(protocol.SetupRequest) },
	"http-api/SetupResult":           func() any { return new(protocol.SetupResult) },
	"http-api/LoginRequest":          func() any { return new(protocol.LoginRequest) },
	"http-api/TokenPair":             func() any { return new(protocol.TokenPair) },
	"http-api/RefreshRequest":        func() any { return new(protocol.RefreshRequest) },
	"http-api/StepUpRequest":         func() any { return new(protocol.StepUpRequest) },
	"http-api/TOTPEnrollment":        func() any { return new(protocol.TOTPEnrollment) },
	"http-api/TOTPConfirmRequest":    func() any { return new(protocol.TOTPConfirmRequest) },
	"http-api/TOTPConfirmResult":     func() any { return new(protocol.TOTPConfirmResult) },
	"http-api/Me":                    func() any { return new(protocol.Me) },
	"http-api/ChangePasswordRequest": func() any { return new(protocol.ChangePasswordRequest) },
	"http-api/DeviceList":            func() any { return new(protocol.DeviceList) },
	"http-api/AlertList":             func() any { return new(protocol.AlertList) },
	"http-api/KeyChallenge":          func() any { return new(protocol.KeyChallenge) },
	"http-api/RegisterKeyRequest":    func() any { return new(protocol.RegisterKeyRequest) },
	"http-api/RegisterKeyResult":     func() any { return new(protocol.RegisterKeyResult) },
	"http-api/KeyList":               func() any { return new(protocol.KeyList) },
	"http-api/AffectedMachines":      func() any { return new(protocol.AffectedMachines) },
	"http-api/EnrollStartRequest":    func() any { return new(protocol.EnrollStartRequest) },
	"http-api/EnrollStartResult":     func() any { return new(protocol.EnrollStartResult) },
	"http-api/EnrollPollRequest":     func() any { return new(protocol.EnrollPollRequest) },
	"http-api/EnrollPollResult":      func() any { return new(protocol.EnrollPollResult) },
	"http-api/EnrollInfo":            func() any { return new(protocol.EnrollInfo) },
	"http-api/EnrollApproveRequest":  func() any { return new(protocol.EnrollApproveRequest) },
	"http-api/Machine":               func() any { return new(protocol.Machine) },
	"http-api/MachineList":           func() any { return new(protocol.MachineList) },
	"http-api/MachinePatch":          func() any { return new(protocol.MachinePatch) },
}

// example là một tệp trong examples/<miền>/.
type example struct {
	Domain  string
	Def     string
	Variant string
	Invalid bool
	Path    string
	Raw     []byte
}

func (e example) key() string  { return e.Domain + "/" + e.Def }
func (e example) name() string { return filepath.Base(e.Path) }

// loadExamples đọc mọi ví dụ và kiểm quy ước đặt tên <Def>.<biến thể>[.invalid].json.
func loadExamples(t *testing.T) []example {
	t.Helper()
	root := protocolPath("examples")
	domains, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("không đọc được %s: %v", root, err)
	}
	var out []example
	for _, d := range domains {
		if !d.IsDir() {
			t.Errorf("examples/ chỉ chứa thư mục theo miền, thấy tệp %s", d.Name())
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, d.Name()))
		if err != nil {
			t.Fatalf("không đọc được miền %s: %v", d.Name(), err)
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				t.Errorf("examples/%s/%s: chỉ chấp nhận tệp .json", d.Name(), f.Name())
				continue
			}
			parts := strings.Split(strings.TrimSuffix(f.Name(), ".json"), ".")
			if len(parts) < 2 {
				t.Errorf("examples/%s/%s: tên phải có dạng <Def>.<biến thể>.json", d.Name(), f.Name())
				continue
			}
			path := filepath.Join(root, d.Name(), f.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("đọc %s: %v", path, err)
			}
			out = append(out, example{
				Domain:  d.Name(),
				Def:     parts[0],
				Variant: strings.Join(parts[1:], "."),
				Invalid: parts[len(parts)-1] == "invalid",
				Path:    path,
				Raw:     raw,
			})
		}
	}
	if len(out) == 0 {
		t.Fatal("không có ví dụ nào")
	}
	return out
}

// fileURL đổi đường dẫn tuyệt đối sang URL file:// dùng được trên cả Windows.
func fileURL(abs string) string {
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

type schemaSet struct {
	compiler *jsonschema.Compiler
	cache    map[string]*jsonschema.Schema
}

func newSchemaSet() *schemaSet {
	c := jsonschema.NewCompiler()
	// Bật kiểm contentEncoding/contentSchema: chữ ký phủ lên payload nên schema
	// phải bắt được payload không giải ra snapshot.
	c.AssertContent()
	return &schemaSet{compiler: c, cache: map[string]*jsonschema.Schema{}}
}

func (s *schemaSet) def(t *testing.T, domain, def string) *jsonschema.Schema {
	t.Helper()
	key := domain + "/" + def
	if sch, ok := s.cache[key]; ok {
		return sch
	}
	abs, err := filepath.Abs(protocolPath("schema", domain+".schema.json"))
	if err != nil {
		t.Fatalf("đường dẫn schema %s: %v", domain, err)
	}
	sch, err := s.compiler.Compile(fileURL(abs) + "#/$defs/" + def)
	if err != nil {
		t.Fatalf("biên dịch schema %s: %v", key, err)
	}
	s.cache[key] = sch
	return sch
}

func validate(t *testing.T, sch *jsonschema.Schema, raw []byte) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("JSON không phân tích được: %v", err)
	}
	return sch.Validate(inst)
}

func TestViDuGiaoThucKhopSchema(t *testing.T) {
	schemas := newSchemaSet()
	count := 0
	for _, ex := range loadExamples(t) {
		if ex.Invalid {
			continue
		}
		count++
		t.Run(ex.Domain+"/"+ex.name(), func(t *testing.T) {
			if err := validate(t, schemas.def(t, ex.Domain, ex.Def), ex.Raw); err != nil {
				t.Errorf("ví dụ hợp lệ bị schema từ chối: %v", err)
			}
		})
	}
	if count == 0 {
		t.Fatal("không có ví dụ hợp lệ nào")
	}
}

func TestViDuKhongHopLeBiSchemaTuChoi(t *testing.T) {
	schemas := newSchemaSet()
	count := 0
	for _, ex := range loadExamples(t) {
		if !ex.Invalid {
			continue
		}
		count++
		t.Run(ex.Domain+"/"+ex.name(), func(t *testing.T) {
			if err := validate(t, schemas.def(t, ex.Domain, ex.Def), ex.Raw); err == nil {
				t.Error("ví dụ *.invalid.json mà schema vẫn chấp nhận: schema quá lỏng hoặc ví dụ sai")
			}
		})
	}
	if count == 0 {
		t.Fatal("hợp đồng phải có ví dụ không hợp lệ để chứng minh schema từ chối được")
	}
}

// loadDefs trả tên các $defs của từng miền, kèm thân JSON của chúng.
func loadDefs(t *testing.T) map[string]map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]map[string]any{}
	for _, domain := range []string{"rpc", "sync", "http-api"} {
		raw, err := os.ReadFile(protocolPath("schema", domain+".schema.json"))
		if err != nil {
			t.Fatalf("đọc schema %s: %v", domain, err)
		}
		var doc struct {
			Defs map[string]map[string]any `json:"$defs"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("schema %s không phải JSON: %v", domain, err)
		}
		out[domain] = doc.Defs
	}
	return out
}

var refPattern = regexp.MustCompile(`"\$ref"\s*:\s*"([^"]+)"`)

// referencedDefs liệt kê <miền>/<Def> được $ref ở đâu đó trong các schema.
func referencedDefs(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, domain := range []string{"rpc", "sync", "http-api"} {
		raw, err := os.ReadFile(protocolPath("schema", domain+".schema.json"))
		if err != nil {
			t.Fatalf("đọc schema %s: %v", domain, err)
		}
		for _, m := range refPattern.FindAllStringSubmatch(string(raw), -1) {
			file, frag, _ := strings.Cut(m[1], "#")
			refDomain := domain
			if file != "" {
				refDomain = strings.TrimSuffix(file, ".schema.json")
			}
			out[refDomain+"/"+strings.TrimPrefix(frag, "/$defs/")] = true
		}
	}
	return out
}

func TestMoiDefCuaSchemaBienDichDuoc(t *testing.T) {
	// Biên dịch cả def chưa có ví dụ: $ref gõ sai hay từ khoá không hợp lệ phải lộ ở đây.
	schemas := newSchemaSet()
	for domain, defs := range loadDefs(t) {
		for name := range defs {
			schemas.def(t, domain, name)
		}
	}
}

func TestMoiViDuTroVaoDefVaKieuGoCoThat(t *testing.T) {
	defs := loadDefs(t)
	for _, ex := range loadExamples(t) {
		if _, ok := defs[ex.Domain]; !ok {
			t.Errorf("%s: thư mục miền %q không có schema tương ứng", ex.name(), ex.Domain)
			continue
		}
		if _, ok := defs[ex.Domain][ex.Def]; !ok {
			t.Errorf("%s/%s: schema %s không có $defs/%s (gõ sai tên?)", ex.Domain, ex.name(), ex.Domain, ex.Def)
			continue
		}
		if !ex.Invalid {
			if _, ok := newValue[ex.key()]; !ok {
				t.Errorf("%s/%s: chưa có kiểu Go cho %s trong bảng newValue", ex.Domain, ex.name(), ex.key())
			}
		}
	}
}

func TestMoiThongDiepCoViDuHopLe(t *testing.T) {
	defs := loadDefs(t)
	referenced := referencedDefs(t)
	scalar := map[string]bool{"string": true, "integer": true, "number": true, "boolean": true}

	haveValid := map[string]bool{}
	for _, ex := range loadExamples(t) {
		if !ex.Invalid {
			haveValid[ex.key()] = true
		}
	}

	for domain, ds := range defs {
		for name, body := range ds {
			key := domain + "/" + name
			if typ, _ := body["type"].(string); scalar[typ] {
				continue // kiểu vô hướng được kiểm qua thông điệp chứa nó
			}
			// Def được def khác tham chiếu thì được kiểm gián tiếp qua ví dụ của def chứa nó.
			if !referenced[key] && !haveValid[key] {
				t.Errorf("%s là thông điệp gốc nhưng không có ví dụ hợp lệ nào", key)
			}
		}
	}
	for key := range newValue {
		if !haveValid[key] {
			t.Errorf("kiểu Go %s đã đăng ký nhưng không có ví dụ hợp lệ nào", key)
		}
	}
}

// normalize đưa JSON về dạng generic để so sánh không phụ thuộc thứ tự khoá hay khoảng trắng.
func normalize(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("JSON không phân tích được: %v\n%s", err, raw)
	}
	return v
}

func TestVongLapGoGiuNguyenTruongDaBiet(t *testing.T) {
	schemas := newSchemaSet()
	for _, ex := range loadExamples(t) {
		if ex.Invalid || ex.Variant == "unknown-fields" {
			continue
		}
		t.Run(ex.Domain+"/"+ex.name(), func(t *testing.T) {
			v := newValue[ex.key()]()

			// Giải mã nghiêm ngặt (chỉ trong test): kiểu Go phải khai báo đủ mọi
			// trường mà ví dụ dùng, nếu không là kiểu Go lệch khỏi tài liệu.
			dec := json.NewDecoder(bytes.NewReader(ex.Raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(v); err != nil {
				t.Fatalf("kiểu Go không nhận được ví dụ: %v", err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("mã hoá lại: %v", err)
			}
			if got, want := normalize(t, out), normalize(t, ex.Raw); !reflect.DeepEqual(got, want) {
				t.Errorf("round-trip làm đổi nội dung\n có:  %s\n cần: %s", out, ex.Raw)
			}
			if err := validate(t, schemas.def(t, ex.Domain, ex.Def), out); err != nil {
				t.Errorf("kết quả mã hoá từ Go không hợp lệ theo schema: %v", err)
			}
		})
	}
}

// injectUnknown thêm một trường lạ vào mọi đối tượng JSON tương ứng với struct Go.
// Khoá của map Go (như meta) không phải trường nên không được thêm vào đó.
func injectUnknown(v any, t reflect.Type) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch x := v.(type) {
	case map[string]any:
		switch t.Kind() {
		case reflect.Struct:
			x["x_truong_la"] = map[string]any{"long": []any{1.0, "hai"}}
			for i := range t.NumField() {
				f := t.Field(i)
				tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
				if tag == "-" {
					continue
				}
				if f.Anonymous && tag == "" {
					injectUnknown(x, f.Type) // struct nhúng được làm phẳng vào cùng đối tượng
					continue
				}
				if child, ok := x[tag]; ok {
					injectUnknown(child, f.Type)
				}
			}
		case reflect.Map:
			for _, e := range x {
				injectUnknown(e, t.Elem())
			}
		}
	case []any:
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			for _, e := range x {
				injectUnknown(e, t.Elem())
			}
		}
	}
}

// stripUnknown bỏ mọi khoá có tiền tố x_ (quy ước đặt tên trường lạ trong ví dụ).
func stripUnknown(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if !strings.HasPrefix(k, "x_") {
				out[k] = stripUnknown(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = stripUnknown(e)
		}
		return out
	default:
		return v
	}
}

func TestTruongLaBiBoQuaKhiParse(t *testing.T) {
	for _, ex := range loadExamples(t) {
		if ex.Invalid {
			continue
		}
		t.Run(ex.Domain+"/"+ex.name(), func(t *testing.T) {
			base := newValue[ex.key()]()
			if err := json.Unmarshal(ex.Raw, base); err != nil {
				t.Fatalf("giải mã ví dụ: %v", err)
			}
			want, err := json.Marshal(base)
			if err != nil {
				t.Fatalf("mã hoá: %v", err)
			}

			// Chèn trường lạ ở mọi tầng rồi giải mã lại bằng đường mặc định: kết
			// quả phải y hệt, không lỗi và không mất trường đã biết.
			generic := normalize(t, ex.Raw)
			injectUnknown(generic, reflect.TypeOf(base))
			withUnknown, err := json.Marshal(generic)
			if err != nil {
				t.Fatalf("mã hoá bản có trường lạ: %v", err)
			}
			got := newValue[ex.key()]()
			if err := json.Unmarshal(withUnknown, got); err != nil {
				t.Fatalf("trường lạ làm hỏng việc giải mã: %v\n%s", err, withUnknown)
			}
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("mã hoá: %v", err)
			}
			if !reflect.DeepEqual(normalize(t, gotJSON), normalize(t, want)) {
				t.Errorf("trường lạ làm đổi nội dung đã biết\n có:  %s\n cần: %s", gotJSON, want)
			}

			// Ví dụ viết tay có trường lạ: kết quả phải bằng ví dụ sau khi bỏ các trường x_.
			if ex.Variant == "unknown-fields" {
				if !reflect.DeepEqual(normalize(t, want), stripUnknown(normalize(t, ex.Raw))) {
					t.Errorf("giải mã ví dụ có trường lạ không giữ đúng phần đã biết\n có:  %s\n gốc: %s", want, ex.Raw)
				}
			}
		})
	}
}

func TestSnapshotRongKhongBaoGioLaNull(t *testing.T) {
	out, err := json.Marshal(protocol.PolicySnapshot{Proto: 1, MachineID: "m", OSUser: "u", Full: true})
	if err != nil {
		t.Fatalf("mã hoá: %v", err)
	}
	if !strings.Contains(string(out), `"grants":[]`) {
		t.Errorf("snapshot không có grant phải ra grants:[] (rỗng hợp lệ), không phải null: %s", out)
	}
}

func TestPayloadCuaVanBanKyLaByteGoc(t *testing.T) {
	envelope := loadOne[protocol.SignedEnvelope](t, "sync", "SignedEnvelope.full")
	snapshot := loadOne[protocol.PolicySnapshot](t, "sync", "PolicySnapshot.full")

	raw, err := envelope.PayloadBytes()
	if err != nil {
		t.Fatalf("PayloadBytes: %v", err)
	}
	if protocol.EncodePayload(raw) != envelope.Payload {
		t.Error("EncodePayload và PayloadBytes phải là cặp nghịch đảo chính xác")
	}
	var fromPayload protocol.PolicySnapshot
	if err := json.Unmarshal(raw, &fromPayload); err != nil {
		t.Fatalf("payload không giải ra snapshot: %v", err)
	}
	if !reflect.DeepEqual(fromPayload, snapshot) {
		t.Errorf("payload của ví dụ ký phải mã hoá đúng snapshot trong ví dụ PolicySnapshot.full")
	}
	if _, err := (protocol.SignedEnvelope{Payload: "%%%"}).PayloadBytes(); err == nil {
		t.Error("payload không phải base64 phải báo lỗi")
	}
}

func loadOne[T any](t *testing.T, domain, name string) T {
	t.Helper()
	raw, err := os.ReadFile(protocolPath("examples", domain, name+".json"))
	if err != nil {
		t.Fatalf("đọc ví dụ %s/%s: %v", domain, name, err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("giải mã ví dụ %s/%s: %v", domain, name, err)
	}
	return v
}

// ---- Kiểm chữ ký thật theo đúng đặc tả mã hoá trong docs/protocol/sync.md ----

// wireString là kiểu string của SSH (RFC 4251 §5): độ dài uint32 big-endian rồi byte.
func wireString(b []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(b)))
	return append(out, b...)
}

// Các thông điệp được ký, dựng đúng như tài liệu mô tả. Nhiều trường được ghép
// bằng kiểu string của SSH (có tiền tố độ dài) chứ không nối thô, để không có
// nhập nhằng ranh giới giữa các trường.
func syncAuthMessage(nonce []byte, origin, machineID string, proto uint32) []byte {
	m := slices.Concat(wireString(nonce), wireString([]byte(origin)), wireString([]byte(machineID)))
	return binary.BigEndian.AppendUint32(m, proto)
}

func keyRolloverMessage(keyID, publicKey string, notBefore uint64) []byte {
	m := slices.Concat(wireString([]byte(keyID)), wireString([]byte(publicKey)))
	return binary.BigEndian.AppendUint64(m, notBefore)
}

func enrollPoPMessage(deviceCode, origin string) []byte {
	return slices.Concat(wireString([]byte(deviceCode)), wireString([]byte(origin)))
}

func deviceKeyPoPMessage(nonce []byte, origin string) []byte {
	return slices.Concat(wireString(nonce), wireString([]byte(origin)))
}

// wireReader đọc lại các trường string của SSH.
type wireReader struct{ b []byte }

func (r *wireReader) next() ([]byte, error) {
	if len(r.b) < 4 {
		return nil, errors.New("thiếu 4 byte độ dài")
	}
	n := binary.BigEndian.Uint32(r.b)
	if uint64(len(r.b)-4) < uint64(n) {
		return nil, errors.New("độ dài vượt dữ liệu còn lại")
	}
	out := r.b[4 : 4+n]
	r.b = r.b[4+n:]
	return out, nil
}

// verifySSH kiểm chữ ký dạng SSH trên (ngữ cảnh, byte 0x00, msg) bằng khoá dạng
// dòng authorized_keys. Chỉ hỗ trợ hai kiểu khoá mà hợp đồng dùng cho ví dụ.
func verifySSH(pubLine string, ctx protocol.SigningContext, msg []byte, sigB64 string) error {
	typ, b64, ok := strings.Cut(pubLine, " ")
	if !ok {
		return errors.New("khoá không phải dạng '<kiểu> <base64>'")
	}
	blob, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("blob khoá: %w", err)
	}
	kr := &wireReader{b: blob}
	if name, err := kr.next(); err != nil || string(name) != typ {
		return fmt.Errorf("kiểu khoá trong blob không khớp %q", typ)
	}

	sigWire, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("chữ ký không phải base64: %w", err)
	}
	sr := &wireReader{b: sigWire}
	format, err := sr.next()
	if err != nil || string(format) != typ {
		return fmt.Errorf("định dạng chữ ký %q không khớp kiểu khoá %q", format, typ)
	}
	sigBlob, err := sr.next()
	if err != nil {
		return fmt.Errorf("blob chữ ký: %w", err)
	}

	data := slices.Concat([]byte(ctx), []byte{0}, msg)
	switch typ {
	case "ssh-ed25519":
		pub, err := kr.next()
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return errors.New("khoá ed25519 sai độ dài")
		}
		if !ed25519.Verify(pub, data, sigBlob) {
			return errors.New("chữ ký ed25519 không khớp")
		}
	case "ecdsa-sha2-nistp256":
		if _, err := kr.next(); err != nil { // tên đường cong
			return err
		}
		point, err := kr.next()
		if err != nil {
			return err
		}
		pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
		if err != nil {
			return fmt.Errorf("điểm đường cong: %w", err)
		}
		rs := &wireReader{b: sigBlob}
		rb, err := rs.next()
		if err != nil {
			return err
		}
		sb, err := rs.next()
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		if !ecdsa.Verify(pub, digest[:], new(big.Int).SetBytes(rb), new(big.Int).SetBytes(sb)) {
			return errors.New("chữ ký ecdsa không khớp")
		}
	default:
		return fmt.Errorf("kiểu khoá %q chưa hỗ trợ trong test", typ)
	}
	return nil
}

// exampleOrigin là origin của server mà các ví dụ có chữ ký đã ký vào.
const exampleOrigin = "https://hub.example.test:7443"

func TestChuKyViDuXacThucTheoDacTa(t *testing.T) {
	policyKeys := loadOne[protocol.EnrollPollResult](t, "http-api", "EnrollPollResult.full").PolicyKeys
	keyByID := func(id string) string {
		for _, k := range policyKeys {
			if k.KeyID == id {
				return k.PublicKey
			}
		}
		t.Fatalf("EnrollPollResult.full không có khoá %q", id)
		return ""
	}
	hostKey := loadOne[protocol.EnrollStartRequest](t, "http-api", "EnrollStartRequest.full").HostKey

	t.Run("snapshot ký trên byte payload", func(t *testing.T) {
		env := loadOne[protocol.SignedEnvelope](t, "sync", "SignedEnvelope.full")
		raw, err := env.PayloadBytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := verifySSH(keyByID(env.KeyID), protocol.ContextPolicySnapshot, raw, env.Sig); err != nil {
			t.Errorf("chữ ký snapshot: %v", err)
		}
		// Ký trên byte chứ không trên JSON đã phân tích: đổi một byte thì hỏng.
		raw[len(raw)/2] ^= 1
		if err := verifySSH(keyByID(env.KeyID), protocol.ContextPolicySnapshot, raw, env.Sig); err == nil {
			t.Error("sửa một byte của payload mà chữ ký vẫn hợp lệ")
		}
	})

	t.Run("thử thách của kênh đồng bộ", func(t *testing.T) {
		hello := loadOne[protocol.SyncHello](t, "sync", "SyncHello.full")
		result := loadOne[protocol.SyncHelloResult](t, "sync", "SyncHelloResult.full")
		auth := loadOne[protocol.SyncAuth](t, "sync", "SyncAuth.full")
		nonce, err := base64.StdEncoding.DecodeString(result.Nonce)
		if err != nil || len(nonce) != 16 {
			t.Fatalf("nonce phải là 16 byte base64: %v", err)
		}
		msg := syncAuthMessage(nonce, exampleOrigin, hello.MachineID, protocol.SyncProto)
		if err := verifySSH(hostKey, protocol.ContextSyncAuth, msg, auth.Sig); err != nil {
			t.Errorf("chữ ký sync-auth: %v", err)
		}
		// Chữ ký gắn với origin: server khác không dùng lại được.
		other := syncAuthMessage(nonce, "https://khac.example.test:7443", hello.MachineID, protocol.SyncProto)
		if err := verifySSH(hostKey, protocol.ContextSyncAuth, other, auth.Sig); err == nil {
			t.Error("chữ ký sync-auth vẫn hợp lệ với origin khác")
		}
		// Tách miền: cùng khoá, cùng dữ liệu nhưng ngữ cảnh khác thì không hợp lệ.
		if err := verifySSH(hostKey, protocol.ContextEnrollPoP, msg, auth.Sig); err == nil {
			t.Error("chữ ký sync-auth dùng lại được ở ngữ cảnh enroll-pop")
		}
	})

	t.Run("chứng thực xoay khoá ký chính sách", func(t *testing.T) {
		e := loadOne[protocol.KeyEndorsement](t, "sync", "KeyEndorsement.full")
		msg := keyRolloverMessage(e.KeyID, e.PublicKey, uint64(e.NotBefore))
		if err := verifySSH(keyByID(e.SignerKeyID), protocol.ContextPolicyKeyRollover, msg, e.Sig); err != nil {
			t.Errorf("chữ ký chứng thực: %v", err)
		}
	})

	t.Run("chứng minh sở hữu host key khi thêm máy", func(t *testing.T) {
		start := loadOne[protocol.EnrollStartResult](t, "http-api", "EnrollStartResult.full")
		poll := loadOne[protocol.EnrollPollRequest](t, "http-api", "EnrollPollRequest.full")
		if poll.DeviceCode != start.DeviceCode {
			t.Fatal("ví dụ poll phải dùng device_code của ví dụ start")
		}
		if err := verifySSH(hostKey, protocol.ContextEnrollPoP, enrollPoPMessage(poll.DeviceCode, exampleOrigin), poll.Sig); err != nil {
			t.Errorf("chữ ký enroll-pop: %v", err)
		}
	})

	t.Run("chứng minh sở hữu khoá thiết bị", func(t *testing.T) {
		challenge := loadOne[protocol.KeyChallenge](t, "http-api", "KeyChallenge.full")
		req := loadOne[protocol.RegisterKeyRequest](t, "http-api", "RegisterKeyRequest.device")
		if req.Nonce != challenge.Nonce {
			t.Fatal("ví dụ đăng ký khoá phải ký đúng nonce của ví dụ thử thách")
		}
		nonce, err := base64.StdEncoding.DecodeString(req.Nonce)
		if err != nil || len(nonce) != 16 {
			t.Fatalf("nonce phải là 16 byte base64: %v", err)
		}
		msg := deviceKeyPoPMessage(nonce, exampleOrigin)
		if err := verifySSH(req.PublicKey, protocol.ContextDeviceKeyPoP, msg, req.Sig); err != nil {
			t.Errorf("chữ ký device-key-pop: %v", err)
		}
		// Chữ ký gắn với origin: lấy được qua server khác thì không dùng lại được ở đây.
		other := deviceKeyPoPMessage(nonce, "https://khac.example.test:7443")
		if err := verifySSH(req.PublicKey, protocol.ContextDeviceKeyPoP, other, req.Sig); err == nil {
			t.Error("chữ ký device-key-pop vẫn hợp lệ với origin khác")
		}
	})
}

func TestHasTimDungCapability(t *testing.T) {
	caps := []protocol.Capability{protocol.CapSessionV1, protocol.CapEventsV1}
	if !protocol.Has(caps, protocol.CapEventsV1) {
		t.Error("phải thấy capability có trong danh sách")
	}
	if protocol.Has(caps, protocol.CapPolicyV1) {
		t.Error("không được thấy capability vắng mặt")
	}
	if protocol.Has(nil, protocol.CapSessionV1) {
		t.Error("danh sách rỗng không chứa gì")
	}
}

func TestAPIErrorLaErrorVaGiuMaLoi(t *testing.T) {
	var err error = protocol.APIError{Code: protocol.APIErrInvalidCredentials, Message: "sai"}
	var apiErr protocol.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "auth.invalid_credentials" {
		t.Fatalf("errors.As phải lấy lại được APIError: %v", err)
	}
	if !strings.Contains(err.Error(), "auth.invalid_credentials") {
		t.Errorf("thông báo lỗi phải nêu mã: %q", err.Error())
	}
}

func TestMaThoatKhongTimThayPhienKhongTrungMaDaDung(t *testing.T) {
	code := protocol.ExitSessionNotFound
	if code <= 0 || code > 255 {
		t.Fatalf("exit-status %d ngoài dải 1..255", code)
	}
	for _, used := range []int{0, 1, 2, 126, 127, 255} {
		if code == used {
			t.Errorf("exit-status %d trùng mã mà shell và công cụ đã dùng", code)
		}
	}
	if code > 128 {
		t.Errorf("exit-status %d nằm trong dải 128+n của tín hiệu", code)
	}
}

func TestMaLoiJSONRPCRiengNamTrongDaiDanhChoSanPham(t *testing.T) {
	seen := map[int]bool{}
	for _, c := range []int{protocol.CodeUnauthorized, protocol.CodeNotFound, protocol.CodeLimitExceeded, protocol.CodeConflict} {
		if c > -32001 || c < -32004 {
			t.Errorf("mã %d ngoài dải -32001..-32004 dành cho sản phẩm", c)
		}
		if seen[c] {
			t.Errorf("mã %d bị dùng hai lần", c)
		}
		seen[c] = true
	}
}
