package product

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestHangSanPhamSuyRaTuTen(t *testing.T) {
	if Name == "" {
		t.Fatal("Name không được rỗng")
	}
	if want := strings.ToUpper(Name) + "_"; EnvPrefix != want {
		t.Errorf("EnvPrefix = %q, cần %q (suy ra từ Name)", EnvPrefix, want)
	}
	if want := Name + "-server"; ServerImage != want {
		t.Errorf("ServerImage = %q, cần %q", ServerImage, want)
	}
	if !strings.HasSuffix(AppID, "."+Name) {
		t.Errorf("AppID = %q phải kết thúc bằng .%s", AppID, Name)
	}
	if StateDirName() != Name {
		t.Errorf("StateDirName() = %q, cần %q", StateDirName(), Name)
	}
	if !strings.HasSuffix(TokenPrefix, "_") || TokenPrefix != strings.ToLower(TokenPrefix) {
		t.Errorf("TokenPrefix = %q phải viết thường và kết thúc bằng _", TokenPrefix)
	}
}

// productJSONKey là một khoá của product.json ở gốc repo kèm giá trị Go tương ứng.
type productJSONKey struct {
	key string
	got any
}

// productJSONKeys liệt kê mọi khoá của product.json mà phía Go phải khớp. Slice
// rỗng viết là []string{} chứ không phải nil để so được với [] trong JSON.
func productJSONKeys() []productJSONKey {
	return []productJSONKey{
		{"name", Name},
		{"agent_binary", AgentBinary},
		{"server_image", ServerImage},
		{"app_id", AppID},
		{"default_agent_port", DefaultAgentPort},
		{"default_server_port", DefaultServerPort},
		{"env_prefix", EnvPrefix},
		{"token_prefix", TokenPrefix},
		{"url_scheme", URLScheme()},
		{"state_dir_name", StateDirName()},
		{"legacy_names", append([]string{}, LegacyNames()...)},
	}
}

func TestHangSanPhamKhopProductJSON(t *testing.T) {
	path := filepath.Join(repoRoot(t), "product.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc %s: %v", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("product.json không phải đối tượng JSON: %v", err)
	}

	known := map[string]bool{"$comment": true}
	for _, k := range productJSONKeys() {
		known[k.key] = true
		val, ok := doc[k.key]
		if !ok {
			t.Errorf("product.json thiếu khoá %q mà internal/product khai báo %#v", k.key, k.got)
			continue
		}
		// Giải mã theo đúng kiểu của hằng Go để lệch kiểu (cổng viết thành chuỗi)
		// cũng lộ ra, không chỉ lệch giá trị.
		want := reflect.New(reflect.TypeOf(k.got))
		if err := json.Unmarshal(val, want.Interface()); err != nil {
			t.Errorf("product.json %q = %s không cùng kiểu với hằng Go %#v: %v", k.key, val, k.got, err)
			continue
		}
		if !reflect.DeepEqual(want.Elem().Interface(), k.got) {
			t.Errorf("product.json %q = %s nhưng internal/product khai báo %#v; sửa cho khớp ở chỗ đang sai", k.key, val, k.got)
		}
	}

	// Khoá mới trong product.json phải được quyết định rõ phía Go có cần hằng
	// tương ứng hay không, thay vì lặng lẽ bị bỏ qua.
	var extra []string
	for key := range doc {
		if !known[key] {
			extra = append(extra, key)
		}
	}
	slices.Sort(extra)
	for _, key := range extra {
		t.Errorf("product.json có khoá %q mà internal/product chưa đối chiếu", key)
	}
}

func TestURLSchemeHopLeTheoRFC3986(t *testing.T) {
	scheme := regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)
	if !scheme.MatchString(URLScheme()) {
		t.Errorf("URLScheme() = %q không phải scheme hợp lệ (chữ thường, bắt đầu bằng chữ cái)", URLScheme())
	}
}

func TestTenThuMucTrangThaiKhongChuaDauPhanCach(t *testing.T) {
	if strings.ContainsAny(StateDirName(), `/\:`) || StateDirName() == "." || StateDirName() == ".." {
		t.Errorf("StateDirName() = %q không dùng được làm tên thư mục", StateDirName())
	}
}

func TestCongMacDinhKhacNhauVaNamTrongDaiCongNguoiDung(t *testing.T) {
	if DefaultAgentPort == DefaultServerPort {
		t.Fatalf("agent và server cùng cổng %d sẽ không chạy chung một máy được", DefaultAgentPort)
	}
	for name, p := range map[string]int{"agent": DefaultAgentPort, "server": DefaultServerPort} {
		if p <= 1024 || p > 65535 {
			t.Errorf("cổng mặc định của %s = %d nằm ngoài dải cổng người dùng", name, p)
		}
	}
}

func TestPhienBanMacDinhLaDev(t *testing.T) {
	if Version != "dev" {
		t.Errorf("Version = %q, bản build thường phải là %q để bản phát hành gán đè bằng ldflags", Version, "dev")
	}
}

func TestLegacyNamesRongKhiChuaTungDoiTen(t *testing.T) {
	if got := LegacyNames(); len(got) != 0 {
		t.Errorf("LegacyNames() = %v, cần rỗng khi chưa đổi tên", got)
	}
}

func TestLegacyNamesTraBanSaoNenSuaKetQuaKhongAnhHuongLanSau(t *testing.T) {
	cu := legacyNames
	t.Cleanup(func() { legacyNames = cu })
	legacyNames = []string{"ten-cu"}

	got := LegacyNames()
	got[0] = "bi-sua"

	if again := LegacyNames(); !slices.Equal(again, []string{"ten-cu"}) {
		t.Errorf("sửa kết quả của LegacyNames() làm đổi dữ liệu gốc: %v", again)
	}
}

// violation là một string literal chứa tên sản phẩm ở chỗ không được phép.
type violation struct {
	Pos   string
	Line  int
	Value string
}

// findProductNameLiterals tìm string literal chứa một trong các tên sản phẩm
// (không phân biệt hoa thường). Import path không tính vì chúng là tên module
// chứ không phải chuỗi do chương trình tự khai báo.
func findProductNameLiterals(fset *token.FileSet, f *ast.File, names []string) []violation {
	importPath := make(map[*ast.BasicLit]bool, len(f.Imports))
	for _, imp := range f.Imports {
		importPath[imp.Path] = true
	}

	var out []violation
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || importPath[lit] {
			return true
		}
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			// Không giải mã được thì quét nguyên văn còn hơn bỏ sót.
			val = lit.Value
		}
		lower := strings.ToLower(val)
		for _, ten := range names {
			if ten != "" && strings.Contains(lower, strings.ToLower(ten)) {
				out = append(out, violation{Pos: fset.Position(lit.Pos()).String(), Line: fset.Position(lit.Pos()).Line, Value: val})
				break
			}
		}
		return true
	})
	return out
}

func scanSource(t *testing.T, src string, names []string) []violation {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "mau.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("mã mẫu không phân tích được: %v", err)
	}
	return findProductNameLiterals(fset, f, names)
}

func TestBoQuetLiteralPhatHienDungViPham(t *testing.T) {
	ten := []string{"rterm"}
	cases := []struct {
		ten  string
		src  string
		want int
	}{
		{"literal thường", "package x\nvar a = \"rterm\"\n", 1},
		{"viết hoa vẫn bị bắt", "package x\nvar a = \"RTERM_SESSION\"\n", 1},
		{"chuỗi thô", "package x\nvar a = `thư mục rterm`\n", 1},
		{"struct tag", "package x\ntype T struct{ A int `json:\"rterm\"` }\n", 1},
		{"import path được miễn", "package x\nimport _ \"github.com/tuanldas/rterm/internal/product\"\n", 0},
		{"chú thích không phải literal", "package x\n// rterm\nvar a = 1\n", 0},
		{"literal không chứa tên", "package x\nvar a = \"khac\"\n", 0},
		{"ghép chuỗi vẫn lộ nửa tên", "package x\nvar a = \"rterm\" + \"-x\"\n", 1},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got := scanSource(t, c.src, ten); len(got) != c.want {
				t.Errorf("tìm thấy %d vi phạm, cần %d: %+v", len(got), c.want, got)
			}
		})
	}
}

func TestBoQuetLiteralBatCaTenCu(t *testing.T) {
	got := scanSource(t, "package x\nvar a = \"ten-cu\"\n", []string{"rterm", "ten-cu"})
	if len(got) != 1 {
		t.Errorf("tên cũ trong LegacyNames cũng phải bị chặn ngoài gói, tìm thấy %d vi phạm", len(got))
	}
}

// isExempt cho biết tệp được phép chứa literal tên sản phẩm. Đường dẫn tính
// từ gốc repo, dùng dấu /.
//   - internal/product: nơi khai báo duy nhất.
//   - internal/protocol/wire_names.go: định danh trên dây phải đóng băng qua
//     lần đổi tên nên không được suy ra từ product; bản thân chúng có thể chứa
//     tên cũ (biến môi trường mang mã phiên).
func isExempt(rel string) bool {
	return strings.HasPrefix(rel, "internal/product/") || rel == "internal/protocol/wire_names.go"
}

// repoRoot tìm gốc repo từ thư mục chạy test (internal/product).
func repoRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("không thấy go.mod ở gốc repo %q: %v", root, err)
	}
	return root
}

// skipDirs liệt kê thư mục không thuộc mã Go của module này.
var skipDirs = map[string]bool{
	".git": true, "testdata": true, "vendor": true, "node_modules": true,
	".dart_tool": true, "build": true, ".idea": true, ".vscode": true,
}

func TestTenSanPhamChiKhaiBaoMotCho(t *testing.T) {
	root := repoRoot(t)
	names := append([]string{Name}, LegacyNames()...)

	var violations []violation
	fileCount := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				// Module lồng nhau (thí nghiệm spike) nằm ngoài build chính.
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isExempt(rel) {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		fileCount++
		for _, v := range findProductNameLiterals(fset, f, names) {
			v.Pos = rel + ":" + strconv.Itoa(v.Line)
			violations = append(violations, v)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("quét repo lỗi: %v", err)
	}
	if fileCount == 0 {
		t.Fatal("không quét được tệp Go nào, đường dẫn gốc repo sai")
	}
	for _, v := range violations {
		t.Errorf("%s: literal %q chứa tên sản phẩm; dùng gói product (hoặc hằng giao thức nếu là định danh trên dây)", v.Pos, v.Value)
	}
}
