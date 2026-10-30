#!/usr/bin/env python3
"""Cổng giấy phép cho phụ thuộc PyPI của server (chỉ dùng thư viện chuẩn).

Đọc metadata của mọi gói trong môi trường Python đang chạy script (importlib.metadata) và xác
định giấy phép theo thứ tự: License-Expression (PEP 639) → classifier "License :: …" → trường
License nếu nó là một biểu thức SPDX. Rồi xếp loại theo scripts/license_policy.txt:

- Biểu thức SPDX: AND lấy mức tệ nhất của các vế (phải tuân thủ mọi giấy phép), OR lấy mức tốt
  nhất (được chọn một); AND gắn chặt hơn OR; "X WITH ngoại_lệ" giữ mức của X vì ngoại lệ SPDX
  chỉ cho thêm quyền.
- Mức từ tốt đến tệ: ALLOW (danh sách allow), ASK (ngoài cả allow và deny: người dùng quyết),
  UNKNOWN (không xác định được giấy phép), DENY (danh sách deny). Mọi mức khác ALLOW là đỏ.
- Ngoại lệ (exception pypi <gói> <mã> <phạm vi> …) là kết luận của người đã đọc văn bản giấy
  phép của riêng gói đó, cùng nghĩa với cổng Go và Dart:
  - giấy phép đọc được thì ngoại lệ chỉ gỡ đúng <mã> có trong biểu thức, để gói đổi sang giấy
    phép khác vẫn bị chặn lại;
  - không đọc trọn được giấy phép (không có thông tin, trường License là văn bản tự do,
    classifier chưa ánh xạ, biểu thức hỏng) thì ngoại lệ của gói gỡ được phần không đọc được,
    dù ghi <mã> nào; mã vẫn nhận ra được (classifier đã ánh xạ, mã nêu trong biểu thức hỏng)
    coi là AND và xếp như giấy phép đọc được;
  - <mã> nằm trong deny thì không bao giờ được cho qua, dù nêu trong metadata hay ghi trong
    ngoại lệ: cấm luôn thắng ngoại lệ.
  Ngoại lệ phạm vi dev chỉ áp khi kiểm --scope dev; ngoại lệ runtime áp cả hai phạm vi.
- Chính dự án server (tên lấy từ server/pyproject.toml) không phải phụ thuộc nên bỏ qua.

Chạy trong đúng môi trường cần kiểm, từ thư mục server/:
    uv sync --frozen --no-default-groups --group app
    uv run --no-sync python ../scripts/check_python_licenses.py --scope runtime
    uv sync --frozen
    uv run --no-sync python ../scripts/check_python_licenses.py --scope dev

Mã thoát: 0 xanh, 1 đỏ, 2 dùng sai (tham số, tệp chính sách hỏng, môi trường rỗng).
Giới hạn: chỉ thấy giấy phép gói tự khai trong metadata; mã native đóng kèm wheel (OpenSSL,
thư viện C, crate Rust) dựa vào khai báo của gói chủ.
"""

import argparse
import enum
import importlib.metadata
import re
import sys
import tomllib
from collections.abc import Iterable
from dataclasses import dataclass
from pathlib import Path

_SCRIPT_DIR = Path(__file__).resolve().parent
_DEFAULT_POLICY = _SCRIPT_DIR / "license_policy.txt"
_SERVER_PYPROJECT = _SCRIPT_DIR.parent / "server" / "pyproject.toml"

SCOPES = ("runtime", "dev")
_ECOSYSTEMS = frozenset({"go", "pub", "pypi"})
_OPERATORS = frozenset({"AND", "OR", "WITH"})
# Mã SPDX: chữ, số, ".", "-", hậu tố "+"; LicenseRef-… và DocumentRef-…:LicenseRef-… có ":".
_LICENSE_ID = re.compile(r"[A-Za-z0-9.+:-]+")
_DENY_PREFIX = re.compile(r"[A-Za-z0-9.+:-]+\*")
# Từ của biểu thức SPDX: một dấu ngoặc, hoặc một dãy không có khoảng trắng và ngoặc.
_TOKEN = re.compile(r"[()]|[^\s()]+")

# Classifier giấy phép → mã SPDX. Classifier "BSD License" và "Apache Software License" không
# nêu biến thể: ánh xạ về biến thể phổ biến vì mọi biến thể của hai họ này đều dễ dãi, nên mức
# xếp loại không đổi. Classifier không có ở đây (kể cả "Other/Proprietary", "Public Domain")
# cho mức UNKNOWN để người dùng xem tay.
_OSI = "License :: OSI Approved :: "
_CLASSIFIER_SPDX = {
    _OSI + "Apache Software License": "Apache-2.0",
    _OSI + "BSD License": "BSD-3-Clause",
    _OSI + "Zero-Clause BSD (0BSD)": "0BSD",
    _OSI + "MIT License": "MIT",
    _OSI + "MIT No Attribution License (MIT-0)": "MIT-0",
    _OSI + "ISC License (ISCL)": "ISC",
    _OSI + "Python Software Foundation License": "PSF-2.0",
    _OSI + "zlib/libpng License": "Zlib",
    _OSI + "Boost Software License 1.0 (BSL-1.0)": "BSL-1.0",
    _OSI + "The Unlicense (Unlicense)": "Unlicense",
    "License :: CC0 1.0 Universal (CC0 1.0) Public Domain Dedication": "CC0-1.0",
    _OSI + "Historical Permission Notice and Disclaimer (HPND)": "HPND",
    _OSI + "CMU License (MIT-CMU)": "MIT-CMU",
    _OSI + "PostgreSQL License": "PostgreSQL",
    _OSI + "Universal Permissive License (UPL)": "UPL-1.0",
    _OSI + "Blue Oak Model License (BlueOak-1.0.0)": "BlueOak-1.0.0",
    _OSI + "University of Illinois/NCSA Open Source License": "NCSA",
    _OSI + "Mozilla Public License 1.0 (MPL)": "MPL-1.0",
    _OSI + "Mozilla Public License 1.1 (MPL 1.1)": "MPL-1.1",
    _OSI + "Mozilla Public License 2.0 (MPL 2.0)": "MPL-2.0",
    _OSI + "Eclipse Public License 1.0 (EPL-1.0)": "EPL-1.0",
    _OSI + "Eclipse Public License 2.0 (EPL-2.0)": "EPL-2.0",
    _OSI + "Common Development and Distribution License 1.0 (CDDL-1.0)": "CDDL-1.0",
    _OSI + "European Union Public Licence 1.0 (EUPL 1.0)": "EUPL-1.0",
    _OSI + "European Union Public Licence 1.1 (EUPL 1.1)": "EUPL-1.1",
    _OSI + "European Union Public Licence 1.2 (EUPL 1.2)": "EUPL-1.2",
    _OSI + "GNU Library or Lesser General Public License (LGPL)": "LGPL-2.0-or-later",
    _OSI + "GNU Lesser General Public License v2 (LGPLv2)": "LGPL-2.0-only",
    _OSI + "GNU Lesser General Public License v2 or later (LGPLv2+)": "LGPL-2.0-or-later",
    _OSI + "GNU Lesser General Public License v3 (LGPLv3)": "LGPL-3.0-only",
    _OSI + "GNU Lesser General Public License v3 or later (LGPLv3+)": "LGPL-3.0-or-later",
    _OSI + "GNU General Public License (GPL)": "GPL-1.0-or-later",
    _OSI + "GNU General Public License v2 (GPLv2)": "GPL-2.0-only",
    _OSI + "GNU General Public License v2 or later (GPLv2+)": "GPL-2.0-or-later",
    _OSI + "GNU General Public License v3 (GPLv3)": "GPL-3.0-only",
    _OSI + "GNU General Public License v3 or later (GPLv3+)": "GPL-3.0-or-later",
    _OSI + "GNU Affero General Public License v3": "AGPL-3.0-only",
    _OSI + "GNU Affero General Public License v3 or later (AGPLv3+)": "AGPL-3.0-or-later",
}
# Classifier không mang thông tin giấy phép cụ thể: bỏ qua thay vì coi là giấy phép lạ.
_GENERIC_CLASSIFIERS = frozenset({"License :: OSI Approved", "License :: DFSG approved"})
# Giá trị mà công cụ đóng gói cũ ghi vào trường License khi tác giả không khai.
_PLACEHOLDER_LICENSES = frozenset({"UNKNOWN", "NONE"})


class Verdict(enum.IntEnum):
    """Mức xếp loại, giá trị càng lớn càng tệ (AND lấy max, OR lấy min)."""

    ALLOW = 0
    ASK = 1
    UNKNOWN = 2
    DENY = 3


_VERDICT_TEXT = {
    Verdict.ALLOW: "cho phép",
    Verdict.ASK: "ngoài danh sách cho phép và cấm: hỏi người sở hữu dự án, đồng ý thì ghi ngoại lệ",
    Verdict.UNKNOWN: "không xác định được giấy phép",
    Verdict.DENY: "giấy phép bị cấm",
}


class PolicyError(ValueError):
    """Tệp chính sách sai cú pháp."""


class ExpressionError(ValueError):
    """Biểu thức SPDX sai cú pháp."""


@dataclass(frozen=True)
class Exemption:
    """Ngoại lệ: coi license_id là được phép cho riêng một gói, trong một phạm vi."""

    package: str
    license_id: str
    scope: str
    reason: str

    def applies_to(self, scope: str) -> bool:
        # Gói runtime cũng có mặt khi kiểm dev, nên ngoại lệ runtime áp cả hai phạm vi.
        return self.scope == "runtime" or scope == "dev"


@dataclass(frozen=True)
class Policy:
    """Chính sách đã đọc.

    allow, deny, deny_prefixes và extra_allow lưu chữ thường vì SPDX so mã không phân biệt hoa
    thường; ngoại lệ giữ nguyên cách viết để in ra đúng như trong tệp.
    """

    allow: frozenset[str]
    deny: frozenset[str]
    deny_prefixes: tuple[str, ...]
    exemptions: tuple[Exemption, ...]

    def verdict_of(self, license_id: str, extra_allow: frozenset[str] = frozenset()) -> Verdict:
        """Mức của một mã; extra_allow là mã mà ngoại lệ của riêng gói đang xét cho phép."""
        key = license_id.lower()
        # Cấm xét trước mọi thứ, kể cả ngoại lệ: ngoại lệ chỉ gỡ được mức ASK và UNKNOWN, không
        # bao giờ mở cho giấy phép bị cấm. Cổng Go và Dart cũng theo đúng quy tắc này.
        if key in self.deny or key.startswith(self.deny_prefixes):
            return Verdict.DENY
        if key in extra_allow or key in self.allow:
            return Verdict.ALLOW
        return Verdict.ASK


def normalize_name(name: str) -> str:
    """Tên gói chuẩn hoá theo PEP 503 để so khớp ("PyOTP", "pyotp" là một)."""
    return re.sub(r"[-_.]+", "-", name).lower()


def parse_policy(text: str) -> Policy:
    """Đọc license_policy.txt; dòng sai cú pháp là lỗi chứ không bị bỏ qua lặng lẽ."""
    allow: set[str] = set()
    deny: set[str] = set()
    prefixes: list[str] = []
    exemptions: list[Exemption] = []
    for number, raw in enumerate(text.splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        kind, *tail = line.split(maxsplit=1)
        rest = tail[0] if tail else ""
        args = rest.split()
        if kind in ("allow", "deny") and len(args) != 1:
            raise PolicyError(f"dòng {number}: '{kind}' cần đúng một mã giấy phép: {line!r}")
        if kind == "allow":
            if not _LICENSE_ID.fullmatch(args[0]):
                raise PolicyError(f"dòng {number}: mã giấy phép không hợp lệ {args[0]!r}")
            allow.add(args[0].lower())
        elif kind == "deny":
            if _DENY_PREFIX.fullmatch(args[0]):
                prefixes.append(args[0][:-1].lower())
            elif _LICENSE_ID.fullmatch(args[0]):
                deny.add(args[0].lower())
            else:
                raise PolicyError(f"dòng {number}: mục cấm không hợp lệ {args[0]!r}")
        elif kind == "exception":
            fields = rest.split(maxsplit=4)
            if len(fields) < 5:
                raise PolicyError(
                    f"dòng {number}: 'exception' cần <hệ> <gói> <mã-SPDX> <phạm-vi> <lý do>:"
                    f" {line!r}"
                )
            ecosystem, package, license_id, scope, reason = fields
            if ecosystem not in _ECOSYSTEMS:
                raise PolicyError(
                    f"dòng {number}: hệ {ecosystem!r} không thuộc {sorted(_ECOSYSTEMS)}"
                )
            if scope not in SCOPES:
                raise PolicyError(f"dòng {number}: phạm vi {scope!r} không thuộc {list(SCOPES)}")
            if not _LICENSE_ID.fullmatch(license_id):
                raise PolicyError(f"dòng {number}: mã giấy phép không hợp lệ {license_id!r}")
            if ecosystem == "pypi":
                exemptions.append(Exemption(normalize_name(package), license_id, scope, reason))
        else:
            raise PolicyError(f"dòng {number}: không hiểu chỉ thị {kind!r}")
    return Policy(frozenset(allow), frozenset(deny), tuple(prefixes), tuple(exemptions))


type Expression = LicenseRef | Combination


@dataclass(frozen=True)
class LicenseRef:
    """Lá của biểu thức: một mã giấy phép."""

    license_id: str


@dataclass(frozen=True)
class Combination:
    """Nhiều vế nối bằng cùng một toán tử AND hoặc OR."""

    operator: str
    parts: tuple[Expression, ...]


class _Parser:
    """Phân tích cú pháp biểu thức SPDX (phụ lục D của đặc tả SPDX) theo đệ quy xuống."""

    def __init__(self, text: str) -> None:
        self._tokens: list[str] = _TOKEN.findall(text)
        self._pos = 0

    def parse(self) -> Expression:
        if not self._tokens:
            raise ExpressionError("biểu thức rỗng")
        node = self._any_of()
        if self._pos != len(self._tokens):
            raise ExpressionError(f"thừa {self._tokens[self._pos]!r}")
        return node

    def _accept(self, token: str) -> bool:
        # Toán tử SPDX viết hoa theo đặc tả, nhưng nhận cả chữ thường như nhiều gói vẫn viết.
        if self._pos < len(self._tokens) and self._tokens[self._pos].upper() == token:
            self._pos += 1
            return True
        return False

    def _any_of(self) -> Expression:
        parts = [self._all_of()]
        while self._accept("OR"):
            parts.append(self._all_of())
        return parts[0] if len(parts) == 1 else Combination("OR", tuple(parts))

    def _all_of(self) -> Expression:
        parts = [self._with_exception()]
        while self._accept("AND"):
            parts.append(self._with_exception())
        return parts[0] if len(parts) == 1 else Combination("AND", tuple(parts))

    def _with_exception(self) -> Expression:
        node = self._primary()
        if self._accept("WITH"):
            self._identifier()  # ngoại lệ SPDX chỉ cho thêm quyền nên giữ mức của giấy phép gốc
        return node

    def _primary(self) -> Expression:
        if self._accept("("):
            node = self._any_of()
            if not self._accept(")"):
                raise ExpressionError("thiếu ')'")
            return node
        return LicenseRef(self._identifier())

    def _identifier(self) -> str:
        if self._pos >= len(self._tokens):
            raise ExpressionError("thiếu mã giấy phép ở cuối")
        token = self._tokens[self._pos]
        if token.upper() in _OPERATORS or not _LICENSE_ID.fullmatch(token):
            raise ExpressionError(f"cần mã giấy phép, gặp {token!r}")
        self._pos += 1
        return token


def parse_expression(text: str) -> Expression:
    return _Parser(text).parse()


def license_ids(node: Expression) -> frozenset[str]:
    """Mọi mã giấy phép trong biểu thức, chữ thường."""
    if isinstance(node, LicenseRef):
        return frozenset({node.license_id.lower()})
    return frozenset().union(*(license_ids(part) for part in node.parts))


def _named_ids(text: str) -> tuple[str, ...]:
    """Mọi từ có dạng mã giấy phép trong một biểu thức hỏng, trừ toán tử.

    Không biết các mã nối nhau ra sao nên người gọi coi chúng là AND, như classifier: mã bị cấm
    nêu trong biểu thức vẫn thắng ngoại lệ dù biểu thức không phân tích được.
    """
    tokens = _TOKEN.findall(text)
    return tuple(
        dict.fromkeys(t for t in tokens if t.upper() not in _OPERATORS and _LICENSE_ID.fullmatch(t))
    )


def evaluate(
    node: Expression, policy: Policy, extra_allow: frozenset[str] = frozenset()
) -> Verdict:
    if isinstance(node, LicenseRef):
        return policy.verdict_of(node.license_id, extra_allow)
    verdicts = [evaluate(part, policy, extra_allow) for part in node.parts]
    return max(verdicts) if node.operator == "AND" else min(verdicts)


def classify(expr: str, policy: Policy, extra_allow: frozenset[str] = frozenset()) -> Verdict:
    """Mức của một biểu thức SPDX theo chính sách; biểu thức hỏng là UNKNOWN."""
    try:
        return evaluate(parse_expression(expr), policy, extra_allow)
    except ExpressionError:
        return Verdict.UNKNOWN


@dataclass(frozen=True)
class Detected:
    """Giấy phép đọc được từ metadata.

    expression là None khi không đọc trọn được giấy phép; lúc đó source nêu lý do thay vì nguồn,
    còn recognized giữ các mã vẫn nhận ra được (classifier đã ánh xạ đi cùng classifier chưa ánh
    xạ), để mã bị cấm trong đó không bị ngoại lệ gỡ mất.
    """

    expression: str | None
    source: str
    recognized: tuple[str, ...] = ()


def _is_spdx_expression(text: str) -> bool:
    try:
        parse_expression(text)
    except ExpressionError:
        return False
    return True


def detect_license(metadata: importlib.metadata.PackageMetadata) -> Detected:
    expression = (metadata.get("License-Expression") or "").strip()
    if expression:
        return Detected(expression, "License-Expression")

    classifiers = [
        c.strip()
        for c in metadata.get_all("Classifier") or []
        if c.strip().startswith("License ::") and c.strip() not in _GENERIC_CLASSIFIERS
    ]
    if classifiers:
        mapped = [_CLASSIFIER_SPDX[c] for c in classifiers if c in _CLASSIFIER_SPDX]
        unmapped = [c for c in classifiers if c not in _CLASSIFIER_SPDX]
        # Classifier không nói quan hệ giữa các giấy phép; coi là AND để không lạc quan.
        ids = tuple(dict.fromkeys(mapped))
        if unmapped:
            return Detected(None, f"classifier chưa ánh xạ được sang SPDX: {unmapped}", ids)
        return Detected(" AND ".join(ids), "classifier")

    field = (metadata.get("License") or "").strip()
    if field and "\n" not in field and field.upper() not in _PLACEHOLDER_LICENSES:
        if _is_spdx_expression(field):
            return Detected(field, "License")
        return Detected(None, f"trường License không phải biểu thức SPDX: {field[:60]!r}")
    return Detected(None, "không có thông tin giấy phép")


@dataclass(frozen=True)
class Finding:
    """Kết quả kiểm một gói; note giải thích mức (rỗng khi được phép thẳng).

    used_exemptions là ngoại lệ đã áp cho gói vì gói cần ngoại lệ, kể cả khi gói vẫn đỏ (ngoại lệ
    ghi mã bị cấm, hoặc chưa đủ để gỡ mọi giấy phép). Ngoại lệ không có trong Finding nào là
    không dùng tới: gói vắng mặt, đổi giấy phép, hoặc đạt sẵn không cần ngoại lệ.
    """

    package: str
    version: str
    detected: Detected
    verdict: Verdict
    note: str = ""
    used_exemptions: tuple[Exemption, ...] = ()


def _reasons(exemptions: Iterable[Exemption]) -> str:
    return "; ".join(f"{e.license_id} ({e.scope}): {e.reason}" for e in exemptions)


def _evaluate_unreadable(
    reason: str, recognized: Iterable[str], own: tuple[Exemption, ...], policy: Policy
) -> tuple[Verdict, str, tuple[Exemption, ...]]:
    """Mức, ghi chú và ngoại lệ đã dùng cho gói không đọc trọn được giấy phép.

    reason nêu vì sao không đọc được; recognized là các mã vẫn nhận ra, coi là AND; own là ngoại
    lệ của gói ở phạm vi đang kiểm. Mã nhận ra được xếp như giấy phép đọc được: ngoại lệ phải ghi
    đúng mã đó. Phần không đọc được thì ngoại lệ là kết luận của người đã đọc văn bản giấy phép,
    ghi mã nào cũng gỡ được, trừ mã bị cấm. Chỉ cần một ngoại lệ của gói ghi mã bị cấm là gói
    thành DENY, để kết quả không tuỳ thứ tự dòng trong tệp chính sách.
    """
    denied = [e.license_id for e in own if policy.verdict_of(e.license_id) is Verdict.DENY]
    if not own:
        remainder = Verdict.UNKNOWN
    elif denied:
        remainder = Verdict.DENY
    else:
        remainder = Verdict.ALLOW
    exempted = frozenset(e.license_id.lower() for e in own)
    parts = {license_id: policy.verdict_of(license_id, exempted) for license_id in recognized}
    verdict = max([remainder, *parts.values()])

    notes = [reason]
    if denied:
        notes.append(f"ngoại lệ ghi {', '.join(denied)}, giấy phép bị cấm")
    notes += [f"{i}: {_VERDICT_TEXT[v]}" for i, v in parts.items() if v is not Verdict.ALLOW]
    if verdict is Verdict.ALLOW:
        notes.append(f"nhờ ngoại lệ {_reasons(own)}")
    # Mọi ngoại lệ của gói đều được xét cho phần không đọc được nên đều đã dùng tới, kể cả khi gói
    # vẫn đỏ: báo cáo không được khuyên xoá chúng như ngoại lệ thừa.
    return verdict, "; ".join(notes), own


def evaluate_package(
    metadata: importlib.metadata.PackageMetadata, policy: Policy, scope: str
) -> Finding:
    package = normalize_name(metadata.get("Name") or "?")
    version = metadata.get("Version") or "?"
    detected = detect_license(metadata)
    own = tuple(e for e in policy.exemptions if e.package == package and e.applies_to(scope))
    if detected.expression is None:
        verdict, note, used = _evaluate_unreadable(
            detected.source, detected.recognized, own, policy
        )
        return Finding(package, version, detected, verdict, note, used)
    try:
        node = parse_expression(detected.expression)
    except ExpressionError as exc:
        verdict, note, used = _evaluate_unreadable(
            f"biểu thức SPDX hỏng: {exc}", _named_ids(detected.expression), own, policy
        )
        return Finding(package, version, detected, verdict, note, used)

    strict = evaluate(node, policy)
    # Đọc được giấy phép thì ngoại lệ phải ghi đúng một mã có trong biểu thức, để gói đổi sang
    # giấy phép khác vẫn bị chặn lại.
    ids = license_ids(node)
    candidates = tuple(e for e in own if e.license_id.lower() in ids)
    relaxed = evaluate(node, policy, frozenset(e.license_id.lower() for e in candidates))
    if relaxed < strict:
        note = f"nhờ ngoại lệ {_reasons(candidates)}"
        return Finding(package, version, detected, relaxed, note, candidates)
    note = "" if strict is Verdict.ALLOW else _VERDICT_TEXT[strict]
    # Gói cần ngoại lệ mà vẫn đỏ thì ngoại lệ khớp vẫn đã dùng tới (ghi mã bị cấm, hoặc chưa đủ);
    # gói đạt sẵn thì ngoại lệ khớp là thừa.
    used = candidates if strict is not Verdict.ALLOW else ()
    return Finding(package, version, detected, strict, note, used)


def check(
    metadatas: Iterable[importlib.metadata.PackageMetadata],
    policy: Policy,
    scope: str,
    skip: frozenset[str] = frozenset(),
) -> list[Finding]:
    """Kiểm mọi gói (bỏ gói trong skip); gói trùng tên thì lấy bản đầu như lúc import."""
    findings: dict[str, Finding] = {}
    for metadata in metadatas:
        name = normalize_name(metadata.get("Name") or "?")
        if name in skip or name in findings:
            continue
        findings[name] = evaluate_package(metadata, policy, scope)
    return sorted(findings.values(), key=lambda f: f.package)


def _own_project_name() -> str:
    with _SERVER_PYPROJECT.open("rb") as file:
        return normalize_name(tomllib.load(file)["project"]["name"])


def _render(findings: list[Finding], policy: Policy, scope: str, skipped: str) -> str:
    lines = [f"Cổng giấy phép PyPI, phạm vi {scope}, bỏ qua chính dự án {skipped}:"]
    for f in findings:
        if f.detected.expression is None:
            license_text = "?"
        else:
            license_text = f"{f.detected.expression} [{f.detected.source}]"
        line = f"  {'XANH' if f.verdict is Verdict.ALLOW else 'ĐỎ  '} {f.package} {f.version}: "
        line += license_text
        if f.verdict is not Verdict.ALLOW:
            line += f" → {f.verdict.name}"
        if f.note:
            line += f"; {f.note}"
        lines.append(line)
    used = {e for f in findings for e in f.used_exemptions}
    for e in policy.exemptions:
        if not e.applies_to(scope):
            continue
        if e not in used:
            lines.append(
                f"  Ghi chú: ngoại lệ {e.package} {e.license_id} ({e.scope}) không dùng tới trong"
                " lượt này; gói đã vắng hoặc đổi giấy phép thì xoá khỏi license_policy.txt."
            )
        elif policy.verdict_of(e.license_id) is Verdict.DENY:
            # Ngoại lệ này đã khớp gói nhưng không bao giờ gỡ được: xoá nó gói vẫn đỏ, nên phải nói
            # rõ cách gỡ thật thay vì để người đọc tưởng chỉ cần dọn tệp chính sách.
            lines.append(
                f"  Ghi chú: ngoại lệ {e.package} {e.license_id} ({e.scope}) không bao giờ cho qua"
                f" vì {e.license_id} bị cấm; phải thay hoặc bỏ gói {e.package}, xoá riêng dòng này"
                " không làm gói xanh."
            )
    red = sum(f.verdict is not Verdict.ALLOW for f in findings)
    lines.append(f"Kết quả: {red}/{len(findings)} gói đỏ.")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Kiểm giấy phép mọi gói PyPI trong môi trường Python đang chạy."
    )
    parser.add_argument(
        "--scope",
        required=True,
        choices=SCOPES,
        help="runtime: môi trường chỉ có nhóm app (đúng thứ trong image); dev: môi trường đủ nhóm",
    )
    parser.add_argument(
        "--policy", type=Path, default=_DEFAULT_POLICY, help="tệp chính sách giấy phép"
    )
    parser.add_argument(
        "--path",
        action="append",
        dest="paths",
        metavar="THƯ_MỤC",
        help="chỉ đọc gói trong thư mục này thay cho sys.path (lặp được)",
    )
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:
        return exc.code if isinstance(exc.code, int) else 2

    try:
        policy = parse_policy(args.policy.read_text(encoding="utf-8"))
        own = _own_project_name()
    except (OSError, PolicyError, tomllib.TOMLDecodeError, KeyError) as exc:
        print(f"lỗi: {exc}", file=sys.stderr)
        return 2

    distributions = (
        importlib.metadata.distributions(path=args.paths)
        if args.paths
        else importlib.metadata.distributions()
    )
    findings = check((d.metadata for d in distributions), policy, args.scope, frozenset({own}))
    if not findings:
        print("lỗi: không thấy gói nào trong môi trường; chạy sai môi trường?", file=sys.stderr)
        return 2
    print(_render(findings, policy, args.scope, own))
    return 0 if all(f.verdict is Verdict.ALLOW for f in findings) else 1


if __name__ == "__main__":
    raise SystemExit(main())
