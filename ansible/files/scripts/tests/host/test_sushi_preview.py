import json
from pathlib import Path

from workstation.host.sushi_preview import update_manifest


def test_update_sushi_manifest_replaces_profile(tmp_path: Path) -> None:
    manifest = tmp_path / "manifest.json"
    manifest.write_text(
        json.dumps({
            "app-id": "old",
            "modules": [
                {"name": "dependency"},
                {"name": "sushi", "config-opts": ["-Dprofile=old", "-Ddocs=false"]},
            ],
        })
    )

    update_manifest(manifest, app_id="org.example.Sushi", profile="preview")

    result = json.loads(manifest.read_text())
    assert result["app-id"] == "org.example.Sushi"
    assert result["modules"][1]["config-opts"] == [
        "-Dprofile=preview",
        "-Ddocs=false",
    ]
