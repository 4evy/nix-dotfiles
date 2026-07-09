from __future__ import annotations

from workstation.local.user_commands import (
    _lspci_display_devices,
    _matching_lines,
)


def test_matching_lines_supports_head_and_tail_limits() -> None:
    text = "alpha GPU\nbeta\ngamma gpu\ndelta GPU\n"

    assert _matching_lines(text, "gpu", limit=2) == "alpha GPU\ngamma gpu"
    assert _matching_lines(text, "gpu", limit=2, tail=True) == "gamma gpu\ndelta GPU"


def test_lspci_display_devices_keeps_following_context() -> None:
    text = "\n".join((
        "00:00.0 Host bridge",
        "01:00.0 VGA compatible controller",
        "    Subsystem",
        "    Kernel driver in use: nvidia",
        "    Kernel modules: nouveau, nvidia",
        "02:00.0 Audio device",
        "03:00.0 Network controller",
    ))

    result = _lspci_display_devices(text)

    assert "VGA compatible controller" in result
    assert "Kernel driver in use: nvidia" in result
    assert "02:00.0 Audio device" in result
    assert "03:00.0 Network controller" not in result
