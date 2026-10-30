"""Lệnh version của server.

Bảo đảm: `version` in đúng _version.VERSION và trả 0; dùng sai trả 2; lệnh console đã cài và
`python -m` cùng chạy được (gói được dựng và khai báo điểm vào đúng).
"""

import shutil
import subprocess
import sys
import sysconfig

import pytest

import rterm_server
from rterm_server import product
from rterm_server._version import VERSION
from rterm_server.cli import main


def test_lenh_version_in_phien_ban(capsys: pytest.CaptureFixture[str]):
    assert main(["version"]) == 0
    assert capsys.readouterr().out == f"{VERSION}\n"


def test_phien_ban_mac_dinh_trong_repo_la_dev():
    assert VERSION == "dev", "bản trong repo phải là dev để image ghi đè bằng tag lúc build"


@pytest.mark.parametrize("argv", [[], ["khong-co-lenh-nay"], ["version", "thua"]])
def test_dung_sai_tra_ma_2(argv: list[str], capsys: pytest.CaptureFixture[str]):
    assert main(argv) == 2
    assert product.SERVER_IMAGE in capsys.readouterr().err, "hướng dẫn dùng phải nêu tên lệnh"


def _console_script() -> list[str]:
    found = shutil.which(product.SERVER_IMAGE, path=sysconfig.get_path("scripts"))
    if found is None:
        pytest.fail(f"lệnh console {product.SERVER_IMAGE} chưa được cài vào môi trường (uv sync?)")
    return [found]


@pytest.mark.parametrize(
    "command",
    [
        pytest.param(_console_script, id="lệnh console"),
        pytest.param(lambda: [sys.executable, "-m", rterm_server.__name__], id="python -m"),
    ],
)
def test_lenh_cai_dat_in_phien_ban(command):
    # Lệnh cố định do test dựng, không nhận đầu vào từ ngoài.
    result = subprocess.run(  # noqa: S603
        [*command(), "version"], capture_output=True, text=True, check=False, timeout=60
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout.strip() == VERSION
