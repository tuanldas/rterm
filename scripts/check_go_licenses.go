// Command check_go_licenses là cổng giấy phép cho phụ thuộc Go của module,
// trực tiếp lẫn gián tiếp, kể cả phụ thuộc chỉ dùng trong test.
//
// Nó chạy google/go-licenses (ghim bản ở goLicensesModule) để lấy giấy phép
// của từng thư viện, rồi xếp loại theo scripts/license_policy.txt, tệp dùng
// chung với cổng pub và PyPI. go-licenses chỉ thấy phụ thuộc của một hệ điều
// hành mỗi lần chạy (phụ thuộc chỉ có trên Windows không lộ khi chạy trên
// Linux), nên cổng chạy nó cho từng hệ điều hành mà agent hỗ trợ và gộp lại.
// go-licenses tự nó thoát 0 cả khi không nhận ra giấy phép, nên mọi kết luận
// nằm ở đây chứ không dựa vào mã thoát của nó.
//
// Chạy ở gốc repo:
//
//	go run ./scripts [-policy scripts/license_policy.txt] [-go-licenses <tệp chạy có sẵn>]
//
// Mã thoát: 0 mọi thư viện đạt; 1 có thư viện bị cấm, cần hỏi hoặc không rõ
// giấy phép; 2 dùng sai, chính sách hỏng hoặc công cụ lỗi.
package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// goLicensesModule là bản go-licenses được dùng. Đổi bản là đổi kết quả phân
// loại giấy phép, nên ghim cứng và chỉ nâng có chủ đích.
const goLicensesModule = "github.com/google/go-licenses/v2@v2.0.1"

// targetOSes là các hệ điều hành mà agent được build cho.
var targetOSes = []string{"linux", "darwin", "windows"}

const (
	// ecosystem là hệ gói của cổng này trong dòng exception của chính sách.
	ecosystem = "go"

	// unknownLicense là chữ go-licenses ghi khi không nhận ra giấy phép.
	unknownLicense = "Unknown"
)

// scope cho biết thư viện vào tệp chạy (runtime) hay chỉ dùng trong test (dev).
type scope string

const (
	scopeRuntime scope = "runtime"
	scopeDev     scope = "dev"
)

// verdict là kết luận cho một giấy phép hoặc cả thư viện, xếp từ tốt tới tệ.
type verdict int

const (
	verdictAllow verdict = iota
	verdictException
	verdictAsk
	verdictUnknown
	verdictDeny
)

func (v verdict) passes() bool { return v == verdictAllow || v == verdictException }

func (v verdict) String() string {
	switch v {
	case verdictAllow:
		return "đạt"
	case verdictException:
		return "đạt nhờ ngoại lệ"
	case verdictAsk:
		return "cần hỏi người dùng"
	case verdictUnknown:
		return "không rõ giấy phép"
	default:
		return "bị cấm"
	}
}

// policyException là một dòng "exception <hệ> <gói> <mã-SPDX> <phạm-vi> <lý do>".
type policyException struct {
	Ecosystem, Package, License string
	Scope                       scope
	Reason                      string
}

// covers cho biết ngoại lệ áp cho thư viện lib ở phạm vi s. Tên thư viện của
// go-licenses có thể là đường dẫn gói con của module (golang.org/x/sys/windows)
// nên ngoại lệ ghi theo module cũng che các gói con. Ngoại lệ runtime bao cả dev.
func (e policyException) covers(lib string, s scope) bool {
	return e.Ecosystem == ecosystem &&
		(lib == e.Package || strings.HasPrefix(lib, e.Package+"/")) &&
		(e.Scope == scopeRuntime || s == scopeDev)
}

type policy struct {
	Allow      map[string]bool
	Deny       []string // mã SPDX hoặc tiền tố kết thúc bằng *
	Exceptions []policyException
}

// denied cho biết license nằm trong nhóm cấm. Mã SPDX không phân biệt hoa
// thường nên so cũng vậy: so phân biệt thì một mã cấm viết thường, như
// "gpl-3.0-only" ghi trong ngoại lệ, sẽ lọt qua.
func (p policy) denied(license string) bool {
	id := strings.ToLower(license)
	for _, d := range p.Deny {
		entry := strings.ToLower(d)
		if prefix, ok := strings.CutSuffix(entry, "*"); (ok && strings.HasPrefix(id, prefix)) || id == entry {
			return true
		}
	}
	return false
}

// verdictOf kết luận theo nhóm allow/deny; cấm thắng nếu một mã nằm ở cả hai.
func (p policy) verdictOf(license string) verdict {
	switch {
	case license == unknownLicense:
		return verdictUnknown
	case p.denied(license):
		return verdictDeny
	case p.Allow[license]:
		return verdictAllow
	default:
		return verdictAsk
	}
}

// parsePolicy đọc chính sách. Dòng sai cú pháp là lỗi chứ không bị bỏ qua: một
// dòng deny gõ sai mà lặng lẽ mất thì cổng xanh sai.
func parsePolicy(text string) (policy, error) {
	p := policy{Allow: map[string]bool{}}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		bad := func(why string) error {
			return fmt.Errorf("chính sách dòng %d: %s: %q", i+1, why, line)
		}
		switch f[0] {
		case "allow":
			if len(f) != 2 {
				return policy{}, bad(`cần "allow <mã-SPDX>"`)
			}
			p.Allow[f[1]] = true
		case "deny":
			if len(f) != 2 {
				return policy{}, bad(`cần "deny <mã-SPDX | tiền-tố*>"`)
			}
			p.Deny = append(p.Deny, f[1])
		case "exception":
			if len(f) < 6 {
				return policy{}, bad(`cần "exception <hệ> <gói> <mã-SPDX> <phạm-vi> <lý do>"`)
			}
			if !slices.Contains([]string{"go", "pub", "pypi"}, f[1]) {
				return policy{}, bad(fmt.Sprintf("hệ %q không thuộc go, pub, pypi", f[1]))
			}
			s := scope(f[4])
			if s != scopeRuntime && s != scopeDev {
				return policy{}, bad(fmt.Sprintf("phạm vi %q không thuộc runtime, dev", f[4]))
			}
			p.Exceptions = append(p.Exceptions, policyException{
				Ecosystem: f[1], Package: f[2], License: f[3], Scope: s,
				Reason: strings.Join(f[5:], " "),
			})
		default:
			return policy{}, bad(fmt.Sprintf("chỉ thị lạ %q", f[0]))
		}
	}
	if len(p.Allow) == 0 {
		return policy{}, errors.New("chính sách không có dòng allow nào")
	}
	return p, nil
}

// classify kết luận cho thư viện lib có các giấy phép licenses. Nhiều giấy phép
// được coi như AND: mọi giấy phép đều phải đạt, vì go-licenses không phân biệt
// "chọn một trong hai" với "mỗi phần một giấy phép".
//
// Ngoại lệ chỉ gỡ được nhóm "cần hỏi" và "không rõ"; giấy phép bị cấm vẫn bị
// cấm, muốn đổi phải sửa chính dòng deny cho thấy rõ là đổi chính sách.
func classify(p policy, lib string, licenses []string, s scope) (verdict, []string) {
	if len(licenses) == 0 {
		licenses = []string{unknownLicense}
	}
	worst := verdictAllow
	var notes []string
	for _, license := range licenses {
		v := p.verdictOf(license)
		switch v {
		case verdictAsk, verdictUnknown:
			// Không nhận ra giấy phép thì ngoại lệ ghi tay là kết luận của người
			// đã đọc văn bản; nhận ra thì ngoại lệ phải đúng mã đó, để thư viện
			// đổi sang giấy phép khác vẫn bị chặn lại.
			var matching, narrower []policyException
			for _, e := range p.Exceptions {
				if e.License != license && license != unknownLicense {
					continue
				}
				if e.covers(lib, s) {
					matching = append(matching, e)
				} else if e.covers(lib, scopeDev) {
					narrower = append(narrower, e)
				}
			}
			// Chỉ cần một ngoại lệ khớp ghi mã bị cấm là cấm, để kết quả không
			// tuỳ thứ tự dòng trong tệp chính sách.
			banned := slices.IndexFunc(matching, func(e policyException) bool { return p.denied(e.License) })
			switch {
			case banned >= 0:
				v = verdictDeny
				notes = append(notes, fmt.Sprintf("%s: ngoại lệ ghi %s, giấy phép bị cấm", license, matching[banned].License))
			case len(matching) > 0:
				v = verdictException
				notes = append(notes, fmt.Sprintf("%s: ngoại lệ (%s)", license, matching[0].Reason))
			case len(narrower) > 0:
				notes = append(notes, fmt.Sprintf("%s: %s; ngoại lệ chỉ cho phạm vi dev nhưng thư viện vào tệp chạy", license, v))
			default:
				notes = append(notes, fmt.Sprintf("%s: %s", license, v))
			}
		case verdictDeny:
			notes = append(notes, fmt.Sprintf("%s: %s", license, v))
		}
		worst = max(worst, v)
	}
	return worst, notes
}

// library là một thư viện mà go-licenses báo, gộp qua mọi lần chạy.
type library struct {
	Licenses []string
	Runtime  bool // có mặt khi không tính phụ thuộc chỉ của test
}

// parseReport đọc CSV mặc định của go-licenses report: thư viện, URL giấy
// phép, tên giấy phép; một thư viện nhiều giấy phép thì nhiều dòng.
func parseReport(r io.Reader) (map[string][]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3
	out := map[string][]string{}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("đọc báo cáo của go-licenses: %w", err)
		}
		if !slices.Contains(out[rec[0]], rec[2]) {
			out[rec[0]] = append(out[rec[0]], rec[2])
		}
	}
}

// merge gộp một báo cáo vào libs; isRuntime đánh dấu báo cáo chạy không kèm test.
func merge(libs map[string]*library, report map[string][]string, isRuntime bool) {
	for name, licenses := range report {
		lib := libs[name]
		if lib == nil {
			lib = &library{}
			libs[name] = lib
		}
		for _, l := range licenses {
			if !slices.Contains(lib.Licenses, l) {
				lib.Licenses = append(lib.Licenses, l)
			}
		}
		lib.Runtime = lib.Runtime || isRuntime
	}
}

func main() {
	policyPath := flag.String("policy", filepath.Join("scripts", "license_policy.txt"), "tệp chính sách giấy phép")
	toolPath := flag.String("go-licenses", "", "tệp chạy go-licenses có sẵn; bỏ trống thì cài "+goLicensesModule+" vào thư mục tạm")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "đối số thừa: %v\n", flag.Args())
		os.Exit(2)
	}
	report, code, err := run(*policyPath, *toolPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cổng giấy phép Go: %v\n", err)
		os.Exit(2)
	}
	if _, err := os.Stdout.WriteString(report); err != nil {
		fmt.Fprintf(os.Stderr, "cổng giấy phép Go: in báo cáo: %v\n", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// run chạy cổng; trả báo cáo để in và mã thoát 0 hoặc 1.
func run(policyPath, toolPath string) (string, int, error) {
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		return "", 0, fmt.Errorf("đọc chính sách: %w", err)
	}
	p, err := parsePolicy(string(raw))
	if err != nil {
		return "", 0, err
	}
	module, err := output(exec.Command("go", "list", "-m"))
	if err != nil {
		return "", 0, fmt.Errorf("tìm module chính (chạy ở gốc repo): %w", err)
	}
	module = strings.TrimSpace(module)

	if toolPath == "" {
		dir, err := os.MkdirTemp("", "go-licenses-")
		if err != nil {
			return "", 0, fmt.Errorf("tạo thư mục tạm cho go-licenses: %w", err)
		}
		// Thư mục tạm chỉ chứa tệp chạy go-licenses; xoá hỏng cũng vô hại.
		defer func() { _ = os.RemoveAll(dir) }()
		install := exec.Command("go", "install", goLicensesModule)
		// Cài cho máy đang chạy dù GOOS/GOARCH của môi trường trỏ nơi khác.
		install.Env = append(os.Environ(), "GOBIN="+dir, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
		if _, err := output(install); err != nil {
			return "", 0, fmt.Errorf("cài %s: %w", goLicensesModule, err)
		}
		toolPath = filepath.Join(dir, "go-licenses")
		if runtime.GOOS == "windows" {
			toolPath += ".exe"
		}
	}

	libs := map[string]*library{}
	for _, goos := range targetOSes {
		for _, withTests := range []bool{false, true} {
			args := []string{"report", "./...", "--ignore", module}
			if withTests {
				args = append(args, "--include_tests")
			}
			cmd := exec.Command(toolPath, args...)
			// Bản phát hành build với CGO_ENABLED=0; quét đúng tập tệp đó.
			cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
			out, err := output(cmd)
			if err != nil {
				return "", 0, fmt.Errorf("go-licenses report (GOOS=%s): %w", goos, err)
			}
			report, err := parseReport(strings.NewReader(out))
			if err != nil {
				return "", 0, err
			}
			merge(libs, report, !withTests)
		}
	}

	var w strings.Builder
	names := make([]string, 0, len(libs))
	for name := range libs {
		names = append(names, name)
	}
	slices.Sort(names)
	failed := 0
	for _, name := range names {
		lib := libs[name]
		s := scopeDev
		if lib.Runtime {
			s = scopeRuntime
		}
		slices.Sort(lib.Licenses)
		v, notes := classify(p, name, lib.Licenses, s)
		mark := "OK "
		if !v.passes() {
			mark = "ĐỎ "
			failed++
		}
		line := fmt.Sprintf("%s %s [%s] %s", mark, name, s, strings.Join(lib.Licenses, " AND "))
		if len(notes) > 0 {
			line += " — " + strings.Join(notes, "; ")
		}
		fmt.Fprintln(&w, line)
	}
	for _, e := range p.Exceptions {
		if e.Ecosystem != ecosystem {
			continue
		}
		used := slices.ContainsFunc(names, func(n string) bool { return e.covers(n, scopeDev) })
		if !used {
			fmt.Fprintf(&w, "Cảnh báo: ngoại lệ cho %s không còn dùng, nên xoá khỏi chính sách.\n", e.Package)
		}
	}
	fmt.Fprintf(&w, "Đã kiểm %d thư viện Go trên %s (gồm phụ thuộc chỉ dùng trong test).\n", len(names), strings.Join(targetOSes, ", "))
	if failed > 0 {
		fmt.Fprintf(&w, "ĐỎ: %d thư viện không đạt chính sách giấy phép. Gỡ phụ thuộc, hoặc hỏi người sở hữu dự án rồi ghi ngoại lệ có lý do vào %s.\n", failed, policyPath)
		return w.String(), 1, nil
	}
	fmt.Fprintln(&w, "XANH: mọi thư viện Go đạt chính sách giấy phép.")
	return w.String(), 0, nil
}

// output chạy cmd và trả stdout; lỗi kèm stderr để người đọc log CI thấy nguyên nhân.
func output(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
