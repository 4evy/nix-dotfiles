from __future__ import annotations

from pathlib import Path

import tomlkit

from workstation.host.apps import merge_rustdesk_options


def test_merge_rustdesk_options_preserves_other_values(tmp_path: Path) -> None:
    config = tmp_path / "RustDesk2.toml"
    config.write_text('[other]\nvalue = 1\n\n[options]\ndirect-server = "N"\n')

    merge_rustdesk_options(config)

    result = tomlkit.parse(config.read_text())
    assert result["other"]["value"] == 1
    assert result["options"]["direct-server"] == "Y"
    assert result["options"]["direct-access-port"] == "21118"
