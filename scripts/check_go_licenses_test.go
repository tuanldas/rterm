package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const testPolicy = `
# chính sách thử
allow MIT
allow BSD-3-Clause
deny GPL-*
deny AGPL-*
exception go example.com/devonly MPL-2.0 dev Chỉ dùng trong test.
exception go example.com/anyscope LGPL-3.0 runtime Đã hỏi người dùng.
exception go example.com/oddtext MIT runtime Văn bản MIT sửa lời, đã đọc tay.
exception go example.com/gpl GPL-3.0 runtime Thử ghi đè lệnh cấm.
exception pub example.com/otheco MPL-2.0 runtime Ngoại lệ của hệ khác không áp cho Go.
`

func mustPolicy(t *testing.T, text string) policy {
	t.Helper()
	p, err := parsePolicy(text)
	if err != nil {
		t.Fatalf("chính sách thử không đọc được: %v", err)
	}
	return p
}

func TestCongGiayPhepGoChanGiayPhepCamVaNgoaiDanhSach(t *testing.T) {
	p := mustPolicy(t, testPolicy)
	cases := []struct {
		ten      string
		licenses []string
		want     verdict
	}{
		{"giấy phép dễ dãi", []string{"MIT"}, verdictAllow},
		{"GPL bị cấm", []string{"GPL-3.0"}, verdictDeny},
		{"AGPL bị cấm", []string{"AGPL-3.0"}, verdictDeny},
		{"LGPL phải hỏi", []string{"LGPL-2.1"}, verdictAsk},
		{"go-licenses không nhận ra", []string{"Unknown"}, verdictUnknown},
		{"không có giấy phép nào", nil, verdictUnknown},
		{"AND: một cái bị cấm là cả thư viện bị cấm", []string{"MIT", "GPL-2.0"}, verdictDeny},
		{"AND: mọi cái đều đạt", []string{"MIT", "BSD-3-Clause"}, verdictAllow},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got, notes := classify(p, "example.com/a", c.licenses, scopeRuntime); got != c.want {
				t.Errorf("kết luận %v, cần %v (%v)", got, c.want, notes)
			}
		})
	}
}

func TestNgoaiLeChiApDungDungThuVienMaVaPhamVi(t *testing.T) {
	p := mustPolicy(t, testPolicy)
	cases := []struct {
		ten      string
		lib      string
		licenses []string
		s        scope
		want     verdict
	}{
		{"ngoại lệ dev cho thư viện chỉ dùng trong test", "example.com/devonly", []string{"MPL-2.0"}, scopeDev, verdictException},
		{"ngoại lệ dev không che thư viện vào tệp chạy", "example.com/devonly", []string{"MPL-2.0"}, scopeRuntime, verdictAsk},
		{"ngoại lệ runtime che cả dev", "example.com/anyscope", []string{"LGPL-3.0"}, scopeDev, verdictException},
		{"ngoại lệ theo module che gói con", "example.com/anyscope/sub", []string{"LGPL-3.0"}, scopeRuntime, verdictException},
		{"tên chỉ trùng tiền tố chuỗi thì không che", "example.com/anyscopex", []string{"LGPL-3.0"}, scopeRuntime, verdictAsk},
		{"đổi sang giấy phép khác thì ngoại lệ cũ không che", "example.com/anyscope", []string{"MPL-2.0"}, scopeRuntime, verdictAsk},
		{"không nhận ra văn bản: ngoại lệ ghi tay là kết luận", "example.com/oddtext", []string{"Unknown"}, scopeRuntime, verdictException},
		{"ngoại lệ của hệ khác không áp", "example.com/otheco", []string{"MPL-2.0"}, scopeRuntime, verdictAsk},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got, notes := classify(p, c.lib, c.licenses, c.s); got != c.want {
				t.Errorf("kết luận %v, cần %v (%v)", got, c.want, notes)
			}
		})
	}
	if _, notes := classify(p, "example.com/devonly", []string{"MPL-2.0"}, scopeRuntime); !strings.Contains(strings.Join(notes, ";"), "phạm vi dev") {
		t.Errorf("thư viện vào tệp chạy mà chỉ có ngoại lệ dev phải được nêu rõ, có: %v", notes)
	}
}

func TestNgoaiLeKhongGoDuocLenhCam(t *testing.T) {
	p := mustPolicy(t, testPolicy)
	if got, _ := classify(p, "example.com/gpl", []string{"GPL-3.0"}, scopeRuntime); got != verdictDeny {
		t.Errorf("ngoại lệ không được gỡ giấy phép bị cấm, kết luận %v", got)
	}
}

func TestLenhCamKhongPhanBietHoaThuong(t *testing.T) {
	p := mustPolicy(t, testPolicy+
		"deny sspl-1.0\n"+
		"exception go example.com/lower gpl-3.0-only runtime Mã cấm viết thường.\n")
	cases := []struct {
		ten      string
		lib      string
		licenses []string
	}{
		{"mã viết thường khớp mục cấm theo tiền tố", "example.com/a", []string{"gpl-3.0"}},
		{"mục cấm viết thường khớp mã viết hoa", "example.com/a", []string{"SSPL-1.0"}},
		{"ngoại lệ ghi mã cấm viết thường không gỡ được giấy phép không rõ", "example.com/lower", []string{unknownLicense}},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got, notes := classify(p, c.lib, c.licenses, scopeRuntime); got != verdictDeny {
				t.Errorf("kết luận %v, cần %v (%v)", got, verdictDeny, notes)
			}
		})
	}
}

func TestNgoaiLeGhiMaCamThangKhongTuyThuTuDong(t *testing.T) {
	const allowed = "exception go example.com/two MIT runtime Đã đọc văn bản.\n"
	const banned = "exception go example.com/two GPL-3.0 runtime Thử ghi đè lệnh cấm.\n"
	cases := []struct{ ten, extra string }{
		{"ngoại lệ hợp lệ đứng trước", allowed + banned},
		{"ngoại lệ cấm đứng trước", banned + allowed},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			p := mustPolicy(t, testPolicy+c.extra)
			for _, s := range []scope{scopeRuntime, scopeDev} {
				if got, notes := classify(p, "example.com/two", []string{unknownLicense}, s); got != verdictDeny {
					t.Errorf("phạm vi %s: kết luận %v, cần %v (%v)", s, got, verdictDeny, notes)
				}
			}
		})
	}
}

func TestGiayPhepCamNhanRaDuocThangNgoaiLeChoPhanKhongRo(t *testing.T) {
	p := mustPolicy(t, testPolicy+"exception go example.com/dual MIT runtime Phần không rõ đã đọc tay.\n")
	for _, s := range []scope{scopeRuntime, scopeDev} {
		if got, notes := classify(p, "example.com/dual", []string{"GPL-3.0", unknownLicense}, s); got != verdictDeny {
			t.Errorf("phạm vi %s: kết luận %v, cần %v (%v)", s, got, verdictDeny, notes)
		}
	}
	// Đối chứng: cùng ngoại lệ đó gỡ được phần không rõ khi phần nhận ra được là dễ dãi.
	if got, notes := classify(p, "example.com/dual", []string{"MIT", unknownLicense}, scopeRuntime); got != verdictException {
		t.Errorf("kết luận %v, cần %v (%v)", got, verdictException, notes)
	}
}

func TestChinhSachThatDocDuocDongSaiBiBaoLoi(t *testing.T) {
	raw, err := os.ReadFile("license_policy.txt")
	if err != nil {
		t.Fatalf("đọc chính sách thật: %v", err)
	}
	p := mustPolicy(t, string(raw))
	for license, want := range map[string]verdict{
		"MIT": verdictAllow, "Apache-2.0": verdictAllow, "BSD-3-Clause": verdictAllow,
		"GPL-3.0": verdictDeny, "AGPL-3.0": verdictDeny, "LGPL-3.0": verdictAsk, "MPL-2.0": verdictAsk,
	} {
		if got := p.verdictOf(license); got != want {
			t.Errorf("chính sách thật: %s là %v, cần %v", license, got, want)
		}
	}

	for _, bad := range []string{
		"allow",
		"permit MIT",
		"deny GPL-* AGPL-*",
		"exception go example.com/a MIT runtime",
		"exception npm example.com/a MIT runtime lý do",
		"exception go example.com/a MIT test lý do",
	} {
		if _, err := parsePolicy("allow MIT\n" + bad); err == nil {
			t.Errorf("dòng sai cú pháp %q phải bị báo lỗi", bad)
		}
	}
	if _, err := parsePolicy("# chỉ có chú thích\n"); err == nil {
		t.Error("chính sách không có allow nào phải bị báo lỗi")
	}
}

func TestBaoCaoGoLicensesDocDungCSV(t *testing.T) {
	report := "example.com/a,https://example.com/a/LICENSE,MIT\n" +
		"example.com/b,\"https://example.com/b,c/LICENSE\",Apache-2.0\n" +
		"example.com/b,https://example.com/b/LICENSE,BSD-3-Clause\n" +
		"example.com/c,Unknown,Unknown\n"
	got, err := parseReport(strings.NewReader(report))
	if err != nil {
		t.Fatalf("đọc báo cáo: %v", err)
	}
	if !slices.Equal(got["example.com/b"], []string{"Apache-2.0", "BSD-3-Clause"}) {
		t.Errorf("thư viện nhiều giấy phép phải giữ đủ, có %v", got["example.com/b"])
	}
	if !slices.Equal(got["example.com/c"], []string{"Unknown"}) {
		t.Errorf("giấy phép không rõ phải được giữ để cổng chặn, có %v", got["example.com/c"])
	}
	if _, err := parseReport(strings.NewReader("thiếu,cột\n")); err == nil {
		t.Error("dòng thiếu cột phải bị báo lỗi thay vì bỏ qua")
	}
}

func TestThuVienChiDungTrongTestDuocXepPhamViDev(t *testing.T) {
	libs := map[string]*library{}
	// Mỗi hệ điều hành chạy hai lượt: không kèm test (runtime) rồi kèm test.
	merge(libs, map[string][]string{"example.com/a": {"MIT"}}, true)
	merge(libs, map[string][]string{"example.com/a": {"MIT"}, "example.com/testonly": {"MIT"}}, false)
	// Phụ thuộc chỉ có trên Windows vẫn là runtime dù lượt Linux không thấy.
	merge(libs, map[string][]string{"example.com/winonly": {"BSD-3-Clause"}}, true)

	if !libs["example.com/a"].Runtime || !libs["example.com/winonly"].Runtime {
		t.Error("thư viện có mặt ở lượt không kèm test của bất kỳ hệ điều hành nào phải là runtime")
	}
	if libs["example.com/testonly"].Runtime {
		t.Error("thư viện chỉ có ở lượt kèm test phải là dev")
	}
}
