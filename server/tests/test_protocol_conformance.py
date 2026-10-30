"""Conformance của server với hợp đồng giao thức docs/protocol, miền sync và http-api.

Bảo đảm:
- Mọi ví dụ hợp lệ qua schema; mọi ví dụ *.invalid.json bị từ chối.
- payload của SignedEnvelope phải giải ra một PolicySnapshot hợp lệ. jsonschema coi
  contentSchema chỉ là chú thích, nên test tự giải base64 rồi kiểm; thiếu bước này thì ví dụ
  SignedEnvelope.payload-khong-phai-snapshot.invalid lọt qua.
- Model Pydantic đọc được mọi ví dụ; đọc rồi ghi lại giữ nguyên trường đã biết và vẫn hợp lệ.
- Trường lạ ở mọi tầng bị bỏ qua, không làm hỏng việc đọc hay đổi phần đã biết.
- Model khớp schema: đúng tập trường, đúng trường bắt buộc; mọi model đều được ví dụ phủ.

Schema dùng $ref chéo giữa ba tệp nên cả ba được nạp vào một referencing.Registry; có registry
thì thư viện không tự tải tham chiếu từ xa.
"""

import base64
import json
import types
import typing
from collections.abc import Iterator
from dataclasses import dataclass
from functools import cache
from pathlib import Path
from typing import Any

import pytest
from jsonschema import Draft202012Validator
from pydantic import ValidationError
from referencing import Registry, Resource

from rterm_server.protocol import http_api, sync
from rterm_server.protocol.wire_message import WireMessage

# Tính lúc import vì tham số hoá cần danh sách ví dụ ngay khi thu thập test.
PROTOCOL_DIR = Path(__file__).resolve().parents[2] / "docs" / "protocol"

# Phải nạp cả ba tệp schema vì $ref đi chéo giữa chúng (sync dùng rpc#/$defs/ID, …).
SCHEMA_DOMAINS = ("rpc", "sync", "http-api")

# Miền server cài đặt và module chứa model của miền đó (tên lớp = tên $defs).
MODELS_BY_DOMAIN: dict[str, types.ModuleType] = {"sync": sync, "http-api": http_api}

# Thuộc tính mà schema chú thích bằng contentSchema, kèm def mà nội dung đã giải phải khớp.
# jsonschema không kiểm contentSchema nên test tự giải base64 và kiểm (xem contract_errors).
SELF_CHECKED_CONTENT = {("sync", "SignedEnvelope", "payload"): ("sync", "PolicySnapshot")}

UNKNOWN_FIELD = "x_truong_la"


@dataclass(frozen=True)
class Example:
    """Một tệp examples/<miền>/<Def>.<biến thể>.json."""

    domain: str
    definition: str
    variant: str
    path: Path

    @property
    def invalid(self) -> bool:
        return self.variant.rsplit(".", 1)[-1] == "invalid"

    def load(self) -> Any:
        return json.loads(self.path.read_bytes())


def _load_examples() -> list[Example]:
    examples = []
    for domain in MODELS_BY_DOMAIN:
        for path in sorted((PROTOCOL_DIR / "examples" / domain).glob("*.json")):
            definition, _, variant = path.name.removesuffix(".json").partition(".")
            examples.append(Example(domain, definition, variant, path))
    return examples


EXAMPLES = _load_examples()
VALID = [e for e in EXAMPLES if not e.invalid]
INVALID = [e for e in EXAMPLES if e.invalid]
ROUND_TRIP = [e for e in VALID if e.variant != "unknown-fields"]


def _example_id(example: Example) -> str:
    return f"{example.domain}/{example.path.name}"


@cache
def _schema_doc(domain: str) -> Any:
    doc = json.loads((PROTOCOL_DIR / "schema" / f"{domain}.schema.json").read_bytes())
    # Schema sai cú pháp JSON Schema thì mọi kết luận phía sau vô nghĩa: dừng ngay ở đây.
    Draft202012Validator.check_schema(doc)
    return doc


@cache
def _registry() -> Registry[Any]:
    return Registry().with_resources(
        (f"{domain}.schema.json", Resource.from_contents(_schema_doc(domain)))
        for domain in SCHEMA_DOMAINS
    )


@cache
def _validator(domain: str, definition: str) -> Draft202012Validator:
    if definition not in _schema_doc(domain)["$defs"]:
        pytest.fail(f"schema {domain} không có $defs/{definition} (gõ sai tên ví dụ?)")
    ref = {"$ref": f"{domain}.schema.json#/$defs/{definition}"}
    return Draft202012Validator(ref, registry=_registry())


def schema_errors(domain: str, definition: str, instance: Any) -> list[str]:
    """Lỗi của instance theo đúng schema, như một trình kiểm JSON Schema bình thường thấy."""
    return [
        f"{'/'.join(map(str, error.absolute_path)) or '(gốc)'}: {error.message}"
        for error in _validator(domain, definition).iter_errors(instance)
    ]


def contract_errors(domain: str, definition: str, instance: Any) -> list[str]:
    """Lỗi theo hợp đồng: schema cộng phần contentSchema mà jsonschema bỏ qua."""
    errors = schema_errors(domain, definition, instance)
    if errors or not isinstance(instance, dict):
        return errors
    for (d, owner, prop), (target_domain, target) in SELF_CHECKED_CONTENT.items():
        if (d, owner) != (domain, definition) or prop not in instance:
            continue
        try:
            decoded = json.loads(base64.b64decode(instance[prop], validate=True))
        except ValueError as exc:  # gồm base64 hỏng, byte không phải UTF-8, JSON hỏng
            errors.append(f"{prop}: không giải được ra JSON: {exc}")
            continue
        errors += [f"{prop} (đã giải) → {e}" for e in schema_errors(target_domain, target, decoded)]
    return errors


def model_for(domain: str, definition: str) -> type[WireMessage]:
    module = MODELS_BY_DOMAIN[domain]
    model = getattr(module, definition, None)
    # So module để không nhận nhầm lớp import từ miền khác (http_api dùng PolicyKey của sync).
    if not (
        isinstance(model, type)
        and issubclass(model, WireMessage)
        and model.__module__ == module.__name__
    ):
        pytest.fail(f"{domain}/{definition}: chưa có model {definition} trong {module.__name__}")
    return model


def _domain_models(domain: str) -> list[type[WireMessage]]:
    module = MODELS_BY_DOMAIN[domain]
    return [
        value
        for value in vars(module).values()
        if isinstance(value, type)
        and issubclass(value, WireMessage)
        and value.__module__ == module.__name__
    ]


def _nested_models(annotation: Any) -> Iterator[type[WireMessage]]:
    if isinstance(annotation, type) and issubclass(annotation, WireMessage):
        yield annotation
        return
    for arg in typing.get_args(annotation):
        yield from _nested_models(arg)


def _inject_unknown(value: Any, annotation: Any) -> None:
    """Chèn một trường lạ vào mọi đối tượng JSON ứng với một model.

    Đi theo kiểu của model nên không chèn vào map tự do (meta, details): khoá của map là dữ
    liệu, không phải trường.
    """
    if isinstance(annotation, type) and issubclass(annotation, WireMessage):
        if isinstance(value, dict):
            value[UNKNOWN_FIELD] = {"long": [1.5, "hai", None], "x_sau": {"co": True}}
            for name, field in annotation.model_fields.items():
                if name in value:
                    _inject_unknown(value[name], field.annotation)
        return
    origin = typing.get_origin(annotation)
    if origin is list and isinstance(value, list):
        (item,) = typing.get_args(annotation)
        for element in value:
            _inject_unknown(element, item)
    elif origin in (typing.Union, types.UnionType):
        for arg in typing.get_args(annotation):
            _inject_unknown(value, arg)


def _strip_unknown(value: Any) -> Any:
    """Bỏ mọi khoá tiền tố x_ (quy ước đặt tên trường lạ trong ví dụ)."""
    if isinstance(value, dict):
        return {k: _strip_unknown(v) for k, v in value.items() if not k.startswith("x_")}
    if isinstance(value, list):
        return [_strip_unknown(v) for v in value]
    return value


def _resolve_ref(domain: str, ref: str) -> tuple[str, str]:
    """Đổi $ref dạng "[<miền>.schema.json]#/$defs/<Def>" thành (miền, Def)."""
    file, _, fragment = ref.partition("#")
    target_domain = file.removesuffix(".schema.json") if file else domain
    return target_domain, fragment.removeprefix("/$defs/")


def _shape(domain: str, definition: str) -> tuple[set[str], set[str]]:
    """Thuộc tính được phép và thuộc tính bắt buộc của một def, gộp các nhánh allOf.

    Thuộc tính có schema false (như password của EmergencyVerifyParams) là thuộc tính bị cấm,
    không tính là trường của thông điệp. Ràng buộc có điều kiện (if/then) không tính.
    """
    body = _schema_doc(domain)["$defs"][definition]
    allowed: set[str] = set()
    required: set[str] = set()
    for part in [body, *body.get("allOf", [])]:
        if "$ref" in part:
            more_allowed, more_required = _shape(*_resolve_ref(domain, part["$ref"]))
            allowed |= more_allowed
            required |= more_required
            continue
        allowed |= {k for k, v in part.get("properties", {}).items() if v is not False}
        required |= set(part.get("required", []))
    return allowed, required


def _content_schemas(
    node: Any, path: tuple[str, ...] = ()
) -> Iterator[tuple[tuple[str, ...], Any]]:
    if isinstance(node, dict):
        if "contentSchema" in node:
            yield path, node["contentSchema"]
        for key, child in node.items():
            yield from _content_schemas(child, (*path, key))
    elif isinstance(node, list):
        for index, child in enumerate(node):
            yield from _content_schemas(child, (*path, str(index)))


def test_moi_mien_co_ca_vi_du_hop_le_lan_khong_hop_le():
    for domain in MODELS_BY_DOMAIN:
        assert any(e.domain == domain for e in VALID), f"miền {domain} không có ví dụ hợp lệ nào"
        assert any(e.domain == domain for e in INVALID), (
            f"miền {domain} không có ví dụ .invalid.json nào để chứng minh schema từ chối được"
        )


@pytest.mark.parametrize("example", VALID, ids=_example_id)
def test_vi_du_giao_thuc_khop_schema(example: Example):
    errors = contract_errors(example.domain, example.definition, example.load())
    assert errors == [], "ví dụ hợp lệ bị hợp đồng từ chối"


@pytest.mark.parametrize("example", INVALID, ids=_example_id)
def test_vi_du_khong_hop_le_bi_schema_tu_choi(example: Example):
    assert contract_errors(example.domain, example.definition, example.load()), (
        "ví dụ .invalid.json mà hợp đồng vẫn chấp nhận: schema quá lỏng, ví dụ sai, hoặc thiếu"
        " bước tự kiểm contentSchema"
    )


def test_payload_envelope_giai_ra_policy_snapshot():
    examples = PROTOCOL_DIR / "examples" / "sync"
    envelope = sync.SignedEnvelope.model_validate_json(
        (examples / "SignedEnvelope.full.json").read_bytes()
    )
    raw = envelope.payload_bytes()
    assert sync.encode_payload(raw) == envelope.payload, (
        "encode_payload và payload_bytes phải là cặp nghịch đảo chính xác"
    )
    assert schema_errors("sync", "PolicySnapshot", json.loads(raw)) == []
    expected = sync.PolicySnapshot.model_validate_json(
        (examples / "PolicySnapshot.full.json").read_bytes()
    )
    assert sync.PolicySnapshot.model_validate_json(raw) == expected, (
        "payload của ví dụ ký phải là đúng snapshot trong PolicySnapshot.full"
    )

    bad = json.loads(
        (examples / "SignedEnvelope.payload-khong-phai-snapshot.invalid.json").read_bytes()
    )
    assert contract_errors("sync", "SignedEnvelope", bad), (
        "payload không giải ra PolicySnapshot mà vẫn lọt: phải tự giải base64 rồi kiểm"
    )
    with pytest.raises(ValidationError):
        sync.PolicySnapshot.model_validate_json(
            sync.SignedEnvelope.model_validate(bad).payload_bytes()
        )

    with pytest.raises(ValueError, match="base64"):
        sync.SignedEnvelope(payload="%%%", sig=envelope.sig, key_id=envelope.key_id).payload_bytes()


@pytest.mark.parametrize("example", ROUND_TRIP, ids=_example_id)
def test_doc_roi_ghi_lai_giu_nguyen_truong_da_biet(example: Example):
    model = model_for(example.domain, example.definition)
    written = model.model_validate_json(example.path.read_bytes()).to_wire()
    assert written == example.load(), "đọc rồi ghi lại làm đổi nội dung"
    assert contract_errors(example.domain, example.definition, written) == [], (
        "thông điệp server ghi ra không hợp lệ theo hợp đồng"
    )


@pytest.mark.parametrize("example", VALID, ids=_example_id)
def test_truong_la_bi_bo_qua_khi_parse(example: Example):
    model = model_for(example.domain, example.definition)
    known = model.model_validate_json(example.path.read_bytes()).to_wire()

    noisy = example.load()
    _inject_unknown(noisy, model)
    assert UNKNOWN_FIELD in noisy, "không chèn được trường lạ: test không kiểm gì"
    parsed = model.model_validate_json(json.dumps(noisy))
    assert parsed.to_wire() == known, "trường lạ làm đổi nội dung đã biết"

    if example.variant == "unknown-fields":
        assert known == _strip_unknown(example.load()), (
            "ví dụ có trường lạ viết tay không giữ đúng phần đã biết"
        )


@pytest.mark.parametrize("domain", list(MODELS_BY_DOMAIN))
def test_model_khop_truong_va_bat_buoc_cua_schema(domain: str):
    models = _domain_models(domain)
    assert models, f"module của miền {domain} không có model nào"
    for model in models:
        name = model.__name__
        assert name in _schema_doc(domain)["$defs"], (
            f"{name} không có $defs/{name} trong schema {domain}"
        )
        allowed, required = _shape(domain, name)
        fields = set(model.model_fields)
        model_required = {n for n, f in model.model_fields.items() if f.is_required()}
        assert fields == allowed, (
            f"{name}: trường của model lệch schema; thiếu {sorted(allowed - fields)},"
            f" thừa {sorted(fields - allowed)}"
        )
        assert model_required == required, (
            f"{name}: trường bắt buộc lệch schema; model {sorted(model_required)},"
            f" schema {sorted(required)}"
        )


def test_moi_model_duoc_vi_du_phu():
    reached: set[type[WireMessage]] = set()
    pending = [model_for(e.domain, e.definition) for e in VALID]
    while pending:
        model = pending.pop()
        if model in reached:
            continue
        reached.add(model)
        for field in model.model_fields.values():
            pending.extend(_nested_models(field.annotation))
    for domain in MODELS_BY_DOMAIN:
        for model in _domain_models(domain):
            assert model in reached, (
                f"{domain}/{model.__name__} không nằm trong ví dụ hợp lệ nào, kể cả gián tiếp:"
                " thêm ví dụ hoặc bỏ model thừa"
            )


def test_moi_content_schema_deu_duoc_tu_kiem():
    found = {}
    for domain in MODELS_BY_DOMAIN:
        for path, content in _content_schemas(_schema_doc(domain)):
            if len(path) != 4 or path[0] != "$defs" or path[2] != "properties":
                pytest.fail(
                    f"contentSchema ở vị trí test chưa biết tự kiểm: {domain} {'/'.join(path)}"
                )
            found[(domain, path[1], path[3])] = _resolve_ref(domain, content.get("$ref", ""))
    assert found == SELF_CHECKED_CONTENT, (
        "jsonschema không kiểm contentSchema: mỗi chỗ dùng phải được khai vào SELF_CHECKED_CONTENT"
    )
