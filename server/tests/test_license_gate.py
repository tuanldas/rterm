"""Cổng giấy phép PyPI (scripts/check_python_licenses.py).

Bảo đảm:
- Biểu thức SPDX được xếp loại đúng: AND lấy mức tệ nhất, OR lấy mức tốt nhất, AND gắn chặt hơn
  OR; gói không xác định được giấy phép là đỏ. Đây là đúng các ca mà pip-licenses phân loại sai.
- Nguồn giấy phép đọc theo thứ tự License-Expression → classifier → trường License (nếu là SPDX).
- Ngoại lệ phạm vi dev không áp cho lượt runtime; ngoại lệ không mở được giấy phép bị cấm.
- Gói không đọc được giấy phép (văn bản tự do, không có thông tin, classifier chưa ánh xạ, biểu
  thức hỏng) đỏ khi chưa có ngoại lệ; ngoại lệ đúng gói, đúng phạm vi thì gỡ được, cùng nghĩa với
  cổng Go và Dart. Ngoại lệ ghi mã bị cấm, hay mã bị cấm vẫn nhận ra trong metadata, không bao giờ
  xanh; mã nhận ra được vẫn cần ngoại lệ ghi đúng mã đó.
- Lệnh trả 0 khi xanh, 1 khi đỏ, 2 khi dùng sai hoặc tệp chính sách hỏng; tệp chính sách thật
  đọc được và chặn GPL.
- Ghi chú cuối báo cáo: ngoại lệ ghi mã bị cấm đã khớp gói được báo là không bao giờ cho qua, phải
  thay hoặc bỏ gói; chỉ ngoại lệ không áp cho gói nào (gói vắng, đổi giấy phép, đạt sẵn) mới bị
  báo là không dùng tới.

Test dùng metadata giả (dist-info tự viết) nên không phụ thuộc gói nào đang cài.
"""

import email
import importlib.util
import sys
import tomllib
from pathlib import Path
from types import ModuleType

import pytest

TEST_POLICY = """\
# Chính sách thử: độc lập với tệp thật để test không đổi theo quyết định của người dùng.
allow MIT
allow Apache-2.0
allow BSD-3-Clause
deny GPL-*
deny AGPL-*
exception pypi certifi MPL-2.0 dev chỉ cần khi kiểm thử
exception go example.test/mod MPL-2.0 runtime thuộc hệ khác, cổng PyPI bỏ qua
"""


@pytest.fixture(scope="module")
def gate(repo_root: Path, monkeypatch_module: pytest.MonkeyPatch) -> ModuleType:
    path = repo_root / "scripts" / "check_python_licenses.py"
    spec = importlib.util.spec_from_file_location("check_python_licenses", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    # dataclass cần tìm thấy module trong sys.modules lúc tạo lớp.
    monkeypatch_module.setitem(sys.modules, spec.name, module)
    # Không ghi bytecode vào scripts/: test không được để lại tệp ngoài server/.
    monkeypatch_module.setattr(sys, "dont_write_bytecode", True)
    spec.loader.exec_module(module)
    return module


@pytest.fixture(scope="module")
def monkeypatch_module():
    with pytest.MonkeyPatch.context() as patch:
        yield patch


@pytest.fixture(scope="module")
def policy(gate: ModuleType):
    return gate.parse_policy(TEST_POLICY)


def _metadata_text(name: str, headers: list[tuple[str, str]]) -> str:
    lines = ["Metadata-Version: 2.4", f"Name: {name}", "Version: 1.0"]
    lines += [f"{key}: {value}" for key, value in headers]
    return "\n".join(lines) + "\n"


def _metadata(name: str, headers: list[tuple[str, str]]):
    # importlib.metadata cũng đọc METADATA bằng bộ phân tích email nên đây là cùng một dạng.
    return email.message_from_string(_metadata_text(name, headers))


@pytest.mark.parametrize(
    ("headers", "expected"),
    [
        pytest.param([("License-Expression", "MIT AND GPL-3.0-only")], "DENY", id="AND có GPL đỏ"),
        pytest.param([("License-Expression", "MIT OR GPL-3.0-only")], "ALLOW", id="OR có MIT xanh"),
        pytest.param([], "UNKNOWN", id="không có thông tin đỏ"),
        pytest.param([("License-Expression", "MPL-2.0")], "ASK", id="ngoài hai danh sách đỏ"),
        pytest.param(
            [("License-Expression", "MIT OR MPL-2.0 AND GPL-3.0-only")],
            "ALLOW",
            id="AND gắn chặt hơn OR",
        ),
        pytest.param(
            [("License-Expression", "(MIT OR GPL-3.0-only) AND AGPL-3.0-or-later")],
            "DENY",
            id="ngoặc đổi thứ tự",
        ),
        pytest.param(
            [("Classifier", "License :: OSI Approved :: MIT License")], "ALLOW", id="classifier"
        ),
        pytest.param(
            [
                ("Classifier", "License :: OSI Approved :: MIT License"),
                ("Classifier", "License :: OSI Approved :: GNU General Public License v3 (GPLv3)"),
            ],
            "DENY",
            id="nhiều classifier coi là AND",
        ),
        pytest.param([("License", "BSD-3-Clause")], "ALLOW", id="trường License là SPDX"),
        pytest.param(
            [("License", "Permission is hereby granted, free of charge")],
            "UNKNOWN",
            id="trường License là văn bản đỏ",
        ),
    ],
)
def test_cong_giay_phep_xu_ly_and_or(gate, policy, headers, expected):
    finding = gate.evaluate_package(_metadata("goi-gia", headers), policy, "runtime")
    assert finding.verdict.name == expected, finding


@pytest.mark.parametrize(
    ("headers", "expected"),
    [
        pytest.param(
            [
                ("License-Expression", "MIT"),
                ("Classifier", "License :: OSI Approved :: GNU General Public License v3 (GPLv3)"),
            ],
            "ALLOW",
            id="License-Expression thắng classifier",
        ),
        pytest.param(
            [
                ("Classifier", "License :: OSI Approved :: MIT License"),
                ("License", "GPL-3.0-only"),
            ],
            "ALLOW",
            id="classifier xét trước trường License",
        ),
        pytest.param([("License-Expression", "mit or gpl-3.0-only")], "ALLOW", id="chữ thường"),
        pytest.param(
            [("License-Expression", "Apache-2.0 WITH LLVM-exception")], "ALLOW", id="WITH"
        ),
        pytest.param([("License-Expression", "MIT AND")], "UNKNOWN", id="biểu thức cụt"),
        pytest.param([("License-Expression", "(MIT")], "UNKNOWN", id="thiếu ngoặc đóng"),
        pytest.param([("License-Expression", "LicenseRef-Rieng")], "ASK", id="LicenseRef"),
        pytest.param(
            [("Classifier", "License :: Other/Proprietary License")],
            "UNKNOWN",
            id="classifier chưa ánh xạ",
        ),
        pytest.param(
            [("Classifier", "License :: OSI Approved"), ("License", "MIT")],
            "ALLOW",
            id="classifier chung chung bị bỏ qua",
        ),
        pytest.param([("License", "UNKNOWN")], "UNKNOWN", id="giá trị giữ chỗ UNKNOWN"),
    ],
)
def test_nguon_giay_phep_va_bieu_thuc_dac_biet(gate, policy, headers, expected):
    finding = gate.evaluate_package(_metadata("goi-gia", headers), policy, "runtime")
    assert finding.verdict.name == expected, finding


def test_ham_classify_theo_dac_ta(gate, policy):
    assert gate.classify("MIT AND GPL-3.0-only", policy) is gate.Verdict.DENY
    assert gate.classify("MIT OR GPL-3.0-only", policy) is gate.Verdict.ALLOW
    assert gate.classify("", policy) is gate.Verdict.UNKNOWN


def test_ngoai_le_dev_khong_ap_cho_runtime(gate, policy):
    metadata = _metadata("Certifi", [("License", "MPL-2.0")])
    runtime = gate.evaluate_package(metadata, policy, "runtime")
    dev = gate.evaluate_package(metadata, policy, "dev")
    assert runtime.verdict.name == "ASK", "ngoại lệ dev không được làm xanh lượt runtime"
    assert dev.verdict.name == "ALLOW", "ngoại lệ dev phải áp cho lượt dev"
    assert "chỉ cần khi kiểm thử" in dev.note


def test_ngoai_le_khong_ap_khi_goi_doi_giay_phep(gate, policy):
    metadata = _metadata("certifi", [("License-Expression", "GPL-3.0-only")])
    assert gate.evaluate_package(metadata, policy, "dev").verdict.name == "DENY"


@pytest.mark.parametrize(
    "license_id",
    [
        pytest.param("GPL-3.0-only", id="GPL-3.0-only"),
        pytest.param("GPL-2.0-or-later", id="GPL-2.0-or-later"),
        pytest.param("AGPL-3.0-only", id="AGPL-3.0-only"),
        pytest.param("gpl-3.0-only", id="mã viết thường"),
    ],
)
def test_ngoai_le_khong_mo_duoc_giay_phep_bi_cam(gate, license_id):
    policy = gate.parse_policy(TEST_POLICY + f"exception pypi goi-gpl {license_id} dev thử mở\n")
    metadata = _metadata("goi-gpl", [("License-Expression", license_id)])
    for scope in ("runtime", "dev"):
        verdict = gate.evaluate_package(metadata, policy, scope).verdict.name
        assert verdict == "DENY", f"ngoại lệ không được mở {license_id} ở phạm vi {scope}"


# Metadata mà cổng không đọc ra được biểu thức giấy phép; hai dạng văn bản tự do phổ biến ở gói cũ.
UNREADABLE = [
    pytest.param([("License", "Apache 2.0")], id="License văn bản tự do Apache 2.0"),
    pytest.param([("License", "MIT License")], id="License văn bản tự do MIT License"),
    pytest.param(
        [("Classifier", "License :: Other/Proprietary License")], id="classifier chưa ánh xạ"
    ),
    pytest.param([], id="không có thông tin"),
    pytest.param([("License-Expression", "MIT AND")], id="biểu thức hỏng"),
]


def _policy_with(gate, *lines: str):
    return gate.parse_policy(TEST_POLICY + "".join(f"{line}\n" for line in lines))


@pytest.mark.parametrize("headers", UNREADABLE)
def test_giay_phep_khong_doc_duoc_khong_co_ngoai_le_thi_do(gate, policy, headers):
    metadata = _metadata("goi-cu", headers)
    for scope in ("runtime", "dev"):
        finding = gate.evaluate_package(metadata, policy, scope)
        assert finding.verdict.name == "UNKNOWN", finding


@pytest.mark.parametrize("headers", UNREADABLE)
def test_ngoai_le_dung_goi_dung_pham_vi_go_duoc_giay_phep_khong_doc_duoc(gate, headers):
    policy = _policy_with(gate, "exception pypi goi-cu Apache-2.0 runtime đã đọc LICENSE của gói")
    # Khác cách viết nhưng là cùng một gói theo PEP 503.
    metadata = _metadata("Goi_Cu", headers)
    for scope in ("runtime", "dev"):
        finding = gate.evaluate_package(metadata, policy, scope)
        assert finding.verdict.name == "ALLOW", finding
        assert "đã đọc LICENSE của gói" in finding.note
        # Ngoại lệ đã áp phải được tính là có dùng, kẻo báo cáo khuyên xoá nhầm.
        assert [e.package for e in finding.used_exemptions] == ["goi-cu"]


@pytest.mark.parametrize("headers", UNREADABLE)
@pytest.mark.parametrize(
    ("line", "runtime", "dev"),
    [
        pytest.param(
            "exception pypi goi-cu Apache-2.0 dev chỉ dùng khi kiểm thử",
            "UNKNOWN",
            "ALLOW",
            id="ngoại lệ dev",
        ),
        pytest.param(
            "exception pypi goi-khac Apache-2.0 runtime gói khác",
            "UNKNOWN",
            "UNKNOWN",
            id="gói khác",
        ),
        pytest.param(
            "exception pypi goi Apache-2.0 runtime chỉ trùng phần đầu tên",
            "UNKNOWN",
            "UNKNOWN",
            id="tên chỉ trùng phần đầu",
        ),
        pytest.param(
            "exception pub goi-cu Apache-2.0 runtime hệ khác",
            "UNKNOWN",
            "UNKNOWN",
            id="hệ khác",
        ),
    ],
)
def test_ngoai_le_khac_pham_vi_hoac_khac_goi_khong_ap(gate, headers, line, runtime, dev):
    policy = _policy_with(gate, line)
    metadata = _metadata("goi-cu", headers)
    assert gate.evaluate_package(metadata, policy, "runtime").verdict.name == runtime
    assert gate.evaluate_package(metadata, policy, "dev").verdict.name == dev


@pytest.mark.parametrize("headers", UNREADABLE)
@pytest.mark.parametrize(
    "lines",
    [
        pytest.param(["exception pypi goi-cu GPL-3.0-only runtime thử mở GPL"], id="GPL"),
        pytest.param(["exception pypi goi-cu AGPL-3.0-or-later runtime thử mở AGPL"], id="AGPL"),
        pytest.param(
            ["exception pypi goi-cu gpl-2.0-or-later runtime mã viết thường"], id="chữ thường"
        ),
        pytest.param(
            [
                "exception pypi goi-cu MIT runtime đã đọc LICENSE",
                "exception pypi goi-cu GPL-3.0-only runtime thử mở GPL",
            ],
            id="kèm ngoại lệ hợp lệ đứng trước",
        ),
        pytest.param(
            [
                "exception pypi goi-cu GPL-3.0-only runtime thử mở GPL",
                "exception pypi goi-cu MIT runtime đã đọc LICENSE",
            ],
            id="kèm ngoại lệ hợp lệ đứng sau",
        ),
    ],
)
def test_ngoai_le_ghi_ma_bi_cam_khong_bao_gio_xanh(gate, headers, lines):
    policy = _policy_with(gate, *lines)
    metadata = _metadata("goi-cu", headers)
    for scope in ("runtime", "dev"):
        verdict = gate.evaluate_package(metadata, policy, scope).verdict.name
        assert verdict == "DENY", f"ngoại lệ ghi mã bị cấm không được làm xanh ở phạm vi {scope}"


@pytest.mark.parametrize(
    "headers",
    [
        pytest.param(
            [
                ("Classifier", "License :: OSI Approved :: GNU General Public License v3 (GPLv3)"),
                ("Classifier", "License :: Other/Proprietary License"),
            ],
            id="classifier GPL kèm classifier chưa ánh xạ",
        ),
        pytest.param(
            [("License-Expression", "(MIT AND GPL-3.0-only")], id="biểu thức hỏng nêu GPL"
        ),
    ],
)
def test_ma_bi_cam_nhan_ra_trong_metadata_thang_ngoai_le(gate, headers):
    # Metadata không nói GPL là một lựa chọn hay bắt buộc nên coi là AND: ngoại lệ cho phần không
    # đọc được không gỡ được GPL.
    policy = _policy_with(
        gate, "exception pypi goi-cu LicenseRef-Thuong-Mai runtime đã mua bản riêng"
    )
    metadata = _metadata("goi-cu", headers)
    for scope in ("runtime", "dev"):
        finding = gate.evaluate_package(metadata, policy, scope)
        assert finding.verdict.name == "DENY", finding


def test_ma_nhan_ra_trong_metadata_van_can_ngoai_le_dung_ma(gate):
    metadata = _metadata(
        "goi-cu",
        [
            ("Classifier", "License :: OSI Approved :: Mozilla Public License 2.0 (MPL 2.0)"),
            ("Classifier", "License :: Other/Proprietary License"),
        ],
    )
    other = _policy_with(gate, "exception pypi goi-cu LicenseRef-Rieng runtime đã đọc phần riêng")
    assert gate.evaluate_package(metadata, other, "runtime").verdict.name == "ASK"
    exact = _policy_with(gate, "exception pypi goi-cu MPL-2.0 runtime đã đọc cả hai phần")
    assert gate.evaluate_package(metadata, exact, "runtime").verdict.name == "ALLOW"


def test_chinh_sach_chi_giu_ngoai_le_pypi(policy):
    assert [e.package for e in policy.exemptions] == ["certifi"]


def test_chinh_sach_nhan_moi_kieu_khoang_trang(gate):
    parsed = gate.parse_policy("allow\tMIT\ndeny   GPL-*\n")
    assert gate.classify("MIT", parsed) is gate.Verdict.ALLOW
    assert gate.classify("GPL-2.0-only", parsed) is gate.Verdict.DENY


@pytest.mark.parametrize(
    "line",
    [
        "alow MIT",
        "allow",
        "allow MIT Apache-2.0",
        "deny GPL-*-x",
        "exception pypi certifi MPL-2.0 dev",
        "exception npm certifi MPL-2.0 dev lý do",
        "exception pypi certifi MPL-2.0 prod lý do",
    ],
)
def test_chinh_sach_sai_cu_phap_bi_tu_choi(gate, line):
    with pytest.raises(gate.PolicyError, match="dòng 2"):
        gate.parse_policy(f"# chú thích\n{line}\n")


def test_chinh_sach_that_doc_duoc_va_chan_gpl(gate, repo_root):
    real = gate.parse_policy((repo_root / "scripts" / "license_policy.txt").read_text("utf-8"))
    assert gate.classify("GPL-3.0-or-later", real) is gate.Verdict.DENY
    assert gate.classify("AGPL-3.0-only", real) is gate.Verdict.DENY
    assert gate.classify("MIT", real) is gate.Verdict.ALLOW
    assert gate.classify("LGPL-3.0-only", real) is gate.Verdict.ASK


def _write_dist(site: Path, name: str, headers: list[tuple[str, str]]) -> None:
    dist_info = site / f"{name.replace('-', '_')}-1.0.dist-info"
    dist_info.mkdir(parents=True)
    (dist_info / "METADATA").write_text(_metadata_text(name, headers), encoding="utf-8")


def _run(gate, tmp_path: Path, scope: str, site: Path, extra: str = "") -> int:
    policy_file = tmp_path / "policy.txt"
    policy_file.write_text(TEST_POLICY + extra, encoding="utf-8")
    return gate.main(["--scope", scope, "--policy", str(policy_file), "--path", str(site)])


def test_lenh_tra_ma_thoat_theo_ket_qua(gate, tmp_path, repo_root, capsys):
    site = tmp_path / "site"
    _write_dist(site, "goi-mit", [("License-Expression", "MIT")])
    # Chính dự án server không có giấy phép khai báo nhưng phải được bỏ qua, không làm đỏ.
    pyproject = tomllib.loads((repo_root / "server" / "pyproject.toml").read_text("utf-8"))
    _write_dist(site, pyproject["project"]["name"], [])
    assert _run(gate, tmp_path, "runtime", site) == 0
    assert "goi-mit" in capsys.readouterr().out

    _write_dist(site, "goi-gpl", [("License-Expression", "GPL-3.0-only")])
    assert _run(gate, tmp_path, "runtime", site) == 1
    out = capsys.readouterr().out
    assert "goi-gpl" in out and "DENY" in out


_DENIED_NOTE = ("không bao giờ cho qua", "thay hoặc bỏ gói")
_UNUSED_NOTE = ("không dùng tới",)


@pytest.mark.parametrize(
    ("package", "headers", "license_id", "expected"),
    [
        pytest.param(
            "goi-gpl",
            [("License-Expression", "GPL-3.0-only")],
            "GPL-3.0-only",
            _DENIED_NOTE,
            id="ghi mã cấm, gói đọc được",
        ),
        pytest.param(
            "goi-cu",
            [("License", "GPL v3")],
            "GPL-3.0-only",
            _DENIED_NOTE,
            id="ghi mã cấm, gói văn bản tự do",
        ),
        pytest.param(
            "goi-hai",
            [("License-Expression", "MPL-2.0 AND EPL-2.0")],
            "MPL-2.0",
            (),
            id="khớp nhưng chưa đủ",
        ),
        pytest.param(
            "goi-mpl", [("License-Expression", "MPL-2.0")], "MPL-2.0", (), id="làm gói xanh"
        ),
        pytest.param(
            "goi-hoac",
            [("License-Expression", "MIT OR MPL-2.0")],
            "MPL-2.0",
            _UNUSED_NOTE,
            id="thừa vì gói đạt sẵn",
        ),
        pytest.param(None, [], "MIT", _UNUSED_NOTE, id="gói vắng mặt"),
        pytest.param(None, [], "GPL-3.0-only", _UNUSED_NOTE, id="ghi mã cấm, gói vắng mặt"),
    ],
)
def test_ghi_chu_ngoai_le_phan_biet_ma_cam_voi_khong_dung_toi(
    gate, tmp_path, capsys, package, headers, license_id, expected
):
    site = tmp_path / "site"
    # Luôn có một gói đạt để môi trường không rỗng.
    _write_dist(site, "goi-mit", [("License-Expression", "MIT")])
    if package is not None:
        _write_dist(site, package, headers)
    name = package or "goi-vang"
    _run(gate, tmp_path, "runtime", site, f"exception pypi {name} {license_id} runtime thử\n")
    notes = [
        line
        for line in capsys.readouterr().out.splitlines()
        if line.startswith("  Ghi chú:") and f" {name} " in line
    ]
    if not expected:
        assert notes == [], "ngoại lệ đang có tác dụng không được bị khuyên xoá"
    else:
        assert len(notes) == 1, notes
        assert all(phrase in notes[0] for phrase in expected), notes


def test_lenh_dung_sai_tra_ma_2(gate, tmp_path, capsys):
    empty = tmp_path / "rong"
    empty.mkdir()
    assert _run(gate, tmp_path, "runtime", empty) == 2, "môi trường rỗng không được coi là xanh"
    assert gate.main(["--scope", "prod"]) == 2
    assert gate.main([]) == 2

    broken = tmp_path / "hong.txt"
    broken.write_text("allow MIT\nkhong-hieu X\n", encoding="utf-8")
    assert gate.main(["--scope", "dev", "--policy", str(broken)]) == 2
    assert "dòng 2" in capsys.readouterr().err
