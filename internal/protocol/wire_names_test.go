package protocol_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tuanldas/rterm/internal/jsonrpc"
	"github.com/tuanldas/rterm/internal/product"
	"github.com/tuanldas/rterm/internal/protocol"
)

// registry là toàn bộ định danh trên dây. testdata/wire_names.golden.json là bản
// đã đóng băng: định danh chỉ được thêm, không được xoá hay đổi giá trị.
type registry struct {
	Protos                map[string]int `json:"protos"`
	APIPrefix             string         `json:"api_prefix"`
	AgentSyncPath         string         `json:"agent_sync_path"`
	SessionEnvVars        []string       `json:"session_env_vars"`
	SessionEnvNew         string         `json:"session_env_new"`
	ExitSessionNotFound   int            `json:"exit_session_not_found"`
	StderrSessionNotFound string         `json:"stderr_session_not_found"`
	SubsystemAgentRPC     string         `json:"subsystem_agent_rpc"`
	RPCMethods            []string       `json:"rpc_methods"`
	RPCNotifications      []string       `json:"rpc_notifications"`
	EventTopics           []string       `json:"event_topics"`
	SyncMethods           []string       `json:"sync_methods"`
	Capabilities          []string       `json:"capabilities"`
	SigningContexts       []string       `json:"signing_contexts"`
	ErrorCodes            map[string]int `json:"jsonrpc_error_codes"`
}

func currentRegistry() registry {
	caps := []protocol.Capability{
		protocol.CapSessionV1, protocol.CapEventsV1, protocol.CapPolicyV1, protocol.CapStatusV1,
		protocol.CapAuditV1, protocol.CapEmergencyV1, protocol.CapDevicesV1, protocol.CapGroupsV1,
		protocol.CapSharingV1, protocol.CapSSHKeysV1,
	}
	contexts := []protocol.SigningContext{
		protocol.ContextPolicySnapshot, protocol.ContextPolicyKeyRollover, protocol.ContextSyncAuth,
		protocol.ContextEnrollPoP, protocol.ContextDeviceKeyPoP, protocol.ContextReleaseKeyset,
		protocol.ContextReleaseManifest,
	}
	r := registry{
		Protos:                map[string]int{"rpc": protocol.RPCProto, "sync": protocol.SyncProto, "api": protocol.APIProto},
		APIPrefix:             protocol.APIPrefix,
		AgentSyncPath:         protocol.AgentSyncPath,
		SessionEnvVars:        slices.Clone(protocol.SessionEnvVars),
		SessionEnvNew:         protocol.SessionEnvNew,
		ExitSessionNotFound:   protocol.ExitSessionNotFound,
		StderrSessionNotFound: protocol.StderrSessionNotFound,
		SubsystemAgentRPC:     protocol.SubsystemAgentRPC,
		RPCMethods: []string{
			protocol.RPCMethodHello, protocol.RPCMethodSessionList, protocol.RPCMethodSessionCreate,
			protocol.RPCMethodSessionClose, protocol.RPCMethodSessionSize, protocol.RPCMethodEventsSubscribe,
		},
		RPCNotifications: []string{protocol.RPCNotifyEvent},
		EventTopics: []string{
			protocol.TopicSessionCreated, protocol.TopicSessionClosed, protocol.TopicSessionAttached,
			protocol.TopicSessionDetached, protocol.TopicSessionResized,
		},
		SyncMethods: []string{
			protocol.SyncMethodHello, protocol.SyncMethodAuth, protocol.SyncMethodPolicyApply,
			protocol.SyncMethodPolicyKeys, protocol.SyncMethodMachineStatus, protocol.SyncMethodAuditReport,
			protocol.SyncMethodEmergencyVerify,
		},
		ErrorCodes: map[string]int{
			// -32000 do thư viện jsonrpc trả nhưng đi trên dây nên cũng đóng băng. Đọc
			// nó từ gói test protocol_test để gói protocol không phải phụ thuộc jsonrpc.
			"server_busy":    jsonrpc.CodeServerBusy,
			"unauthorized":   protocol.CodeUnauthorized,
			"not_found":      protocol.CodeNotFound,
			"limit_exceeded": protocol.CodeLimitExceeded,
			"conflict":       protocol.CodeConflict,
		},
	}
	for _, c := range caps {
		r.Capabilities = append(r.Capabilities, string(c))
	}
	for _, c := range contexts {
		r.SigningContexts = append(r.SigningContexts, string(c))
	}
	return r
}

func loadGolden(t *testing.T) registry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "wire_names.golden.json"))
	if err != nil {
		t.Fatalf("đọc bản đóng băng định danh: %v", err)
	}
	// Khoá cấp cao mà registry chưa có phải làm test đỏ. Nếu không, ai thêm định danh vào
	// bản đóng băng mà quên phía Go thì Go vẫn xanh, trong khi Python và Dart đỏ.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var g registry
	if err := dec.Decode(&g); err != nil {
		t.Fatalf("bản đóng băng không phải JSON hợp lệ hoặc có khoá registry chưa biết: %v", err)
	}
	// Sau đối tượng chỉ được còn khoảng trắng. dec.More() không đủ, vì nó coi `}` hay `]`
	// thừa là hết dữ liệu.
	if tok, err := dec.Token(); err != io.EOF {
		t.Fatalf("bản đóng băng có dữ liệu thừa sau đối tượng JSON: %v %v", tok, err)
	}
	return g
}

// diffSet so hai tập chuỗi: mất đi hoặc đổi tên là vi phạm hợp đồng, thêm mới
// phải được ghi vào bản đóng băng một cách có chủ đích.
func diffSet(t *testing.T, name string, golden, current []string) {
	t.Helper()
	for _, g := range golden {
		if !slices.Contains(current, g) {
			t.Errorf("%s: định danh %q đã đóng băng nhưng mất hoặc bị đổi trong mã", name, g)
		}
	}
	for _, c := range current {
		if !slices.Contains(golden, c) {
			t.Errorf("%s: định danh mới %q chưa có trong testdata/wire_names.golden.json (thêm vào đó và vào docs/protocol/README.md nếu là chủ ý)", name, c)
		}
	}
}

func TestDinhDanhTrenDayDuocDongBang(t *testing.T) {
	g, c := loadGolden(t), currentRegistry()

	if !reflect.DeepEqual(g.SessionEnvVars, c.SessionEnvVars) {
		t.Errorf("SessionEnvVars = %v, đã đóng băng %v: chỉ được thêm alias vào cuối, không xoá hay đổi thứ tự", c.SessionEnvVars, g.SessionEnvVars)
	}
	if !reflect.DeepEqual(g.Protos, c.Protos) {
		t.Errorf("proto = %v, đã đóng băng %v: đổi proto là thay đổi phá vỡ", c.Protos, g.Protos)
	}
	scalars := []struct {
		name         string
		golden, curr any
	}{
		{"APIPrefix", g.APIPrefix, c.APIPrefix},
		{"AgentSyncPath", g.AgentSyncPath, c.AgentSyncPath},
		{"SessionEnvNew", g.SessionEnvNew, c.SessionEnvNew},
		{"ExitSessionNotFound", g.ExitSessionNotFound, c.ExitSessionNotFound},
		{"StderrSessionNotFound", g.StderrSessionNotFound, c.StderrSessionNotFound},
		{"SubsystemAgentRPC", g.SubsystemAgentRPC, c.SubsystemAgentRPC},
	}
	for _, s := range scalars {
		if s.golden != s.curr {
			t.Errorf("%s = %v, đã đóng băng %v", s.name, s.curr, s.golden)
		}
	}
	if !reflect.DeepEqual(g.ErrorCodes, c.ErrorCodes) {
		t.Errorf("mã lỗi JSON-RPC = %v, đã đóng băng %v", c.ErrorCodes, g.ErrorCodes)
	}
	diffSet(t, "rpc_methods", g.RPCMethods, c.RPCMethods)
	diffSet(t, "rpc_notifications", g.RPCNotifications, c.RPCNotifications)
	diffSet(t, "event_topics", g.EventTopics, c.EventTopics)
	diffSet(t, "sync_methods", g.SyncMethods, c.SyncMethods)
	diffSet(t, "capabilities", g.Capabilities, c.Capabilities)
	diffSet(t, "signing_contexts", g.SigningContexts, c.SigningContexts)
}

var envVarName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func TestTenBienPhienHopLeVaKhongTrung(t *testing.T) {
	if len(protocol.SessionEnvVars) == 0 {
		t.Fatal("phải có ít nhất tên biến chính")
	}
	seen := map[string]bool{}
	for _, v := range protocol.SessionEnvVars {
		if !envVarName.MatchString(v) {
			t.Errorf("%q không phải tên biến môi trường hợp lệ (chữ hoa, số, gạch dưới)", v)
		}
		if seen[v] {
			t.Errorf("alias %q bị khai báo hai lần", v)
		}
		seen[v] = true
	}
	if protocol.SessionEnvNew == "" || protocol.SessionEnvNew == protocol.SessionEnvVars[0] {
		t.Errorf("SessionEnvNew = %q không hợp lệ", protocol.SessionEnvNew)
	}
}

// packageSourceFiles liệt kê tệp nguồn (không phải test) của gói protocol.
func packageSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("đọc thư mục gói: %v", err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			out = append(out, e.Name())
		}
	}
	if len(out) == 0 {
		t.Fatal("không thấy tệp nguồn nào của gói")
	}
	return out
}

func TestDinhDanhTrenDayKhongPhuThuocTenSanPham(t *testing.T) {
	productPkg := regexp.MustCompile(`/internal/product$`)

	t.Run("gói protocol không import product", func(t *testing.T) {
		fset := token.NewFileSet()
		for _, name := range packageSourceFiles(t) {
			f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("phân tích %s: %v", name, err)
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if productPkg.MatchString(path) {
					t.Errorf("%s import %s: định danh trên dây không được suy ra từ tên sản phẩm", name, path)
				}
			}
		}
	})

	t.Run("tên biến phiên và mã thoát là literal, không suy ra từ biểu thức", func(t *testing.T) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "wire_names.go", nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("phân tích wire_names.go: %v", err)
		}
		foundEnv, foundExit := false, false
		ast.Inspect(f, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, id := range spec.Names {
				if i >= len(spec.Values) {
					continue
				}
				switch id.Name {
				case "SessionEnvVars":
					foundEnv = true
					lit, ok := spec.Values[i].(*ast.CompositeLit)
					if !ok {
						t.Errorf("SessionEnvVars phải là danh sách literal, nhận được %T", spec.Values[i])
						continue
					}
					for _, elt := range lit.Elts {
						if b, ok := elt.(*ast.BasicLit); !ok || b.Kind != token.STRING {
							t.Errorf("phần tử %T của SessionEnvVars phải là chuỗi literal, không phải biểu thức: tên biến phiên không được suy ra từ nơi khác", elt)
						}
					}
				case "ExitSessionNotFound":
					foundExit = true
					if b, ok := spec.Values[i].(*ast.BasicLit); !ok || b.Kind != token.INT {
						t.Errorf("ExitSessionNotFound phải là số nguyên literal, nhận được %T", spec.Values[i])
					}
				}
			}
			return true
		})
		if !foundEnv || !foundExit {
			t.Fatalf("không tìm thấy khai báo SessionEnvVars (%v) hoặc ExitSessionNotFound (%v) trong wire_names.go", foundEnv, foundExit)
		}
	})

	t.Run("định danh không chứa tên sản phẩm, trừ bí danh biến môi trường", func(t *testing.T) {
		names := append([]string{product.Name}, product.LegacyNames()...)
		g := loadGolden(t)
		ids := slices.Concat(
			[]string{g.APIPrefix, g.AgentSyncPath, g.SessionEnvNew, g.StderrSessionNotFound, g.SubsystemAgentRPC},
			g.RPCMethods, g.RPCNotifications, g.EventTopics, g.SyncMethods, g.Capabilities, g.SigningContexts,
		)
		for k := range g.ErrorCodes {
			ids = append(ids, k)
		}
		for _, id := range ids {
			for _, n := range names {
				if strings.Contains(strings.ToLower(id), strings.ToLower(n)) {
					t.Errorf("định danh trên dây %q chứa tên sản phẩm %q: đổi tên sản phẩm sẽ làm vỡ client cũ", id, n)
				}
			}
		}
	})
}

func TestTaiLieuNeuDayDuDinhDanhTrenDay(t *testing.T) {
	raw, err := os.ReadFile(protocolPath("README.md"))
	if err != nil {
		t.Fatalf("đọc README của hợp đồng: %v", err)
	}
	readme := string(raw)
	g := loadGolden(t)
	ids := slices.Concat(
		[]string{g.APIPrefix, g.AgentSyncPath, g.SessionEnvNew, g.StderrSessionNotFound, g.SubsystemAgentRPC, strconv.Itoa(g.ExitSessionNotFound)},
		g.SessionEnvVars, g.RPCMethods, g.RPCNotifications, g.EventTopics, g.SyncMethods, g.Capabilities, g.SigningContexts,
	)
	for _, code := range g.ErrorCodes {
		ids = append(ids, strconv.Itoa(code))
	}
	for _, id := range ids {
		if !strings.Contains(readme, fmt.Sprintf("`%s`", id)) {
			t.Errorf("docs/protocol/README.md không nêu `%s` trong registry", id)
		}
	}
}
