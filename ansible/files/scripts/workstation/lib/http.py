from __future__ import annotations

from collections.abc import Mapping
from functools import cache
from pathlib import Path

import httpx
from boltons.fileutils import AtomicSaver
from httpx_retries import Retry, RetryTransport

from workstation.lib.validation import safe_path

_TIMEOUT = httpx.Timeout(60.0, connect=20.0)
_RETRY = Retry(
    total=3,
    allowed_methods={"GET"},
    status_forcelist={429, 500, 502, 503, 504},
    backoff_factor=1,
    max_backoff_wait=8,
)


@cache
def client() -> httpx.Client:
    return httpx.Client(
        transport=RetryTransport(retry=_RETRY),
        follow_redirects=True,
        timeout=_TIMEOUT,
    )


def get(
    url: str,
    *,
    params: Mapping[str, str] | None = None,
    headers: Mapping[str, str] | None = None,
) -> httpx.Response:
    response = client().get(url, params=params, headers=headers)
    response.raise_for_status()
    return response


def download(url: str, destination: str | Path, mode: int = 0o644) -> Path:
    destination_path = safe_path(destination)
    destination_path.parent.mkdir(parents=True, exist_ok=True)
    with client().stream("GET", url) as response:
        response.raise_for_status()
        with AtomicSaver(
            str(destination_path),
            overwrite=True,
            text_mode=False,
            file_perms=mode,
        ) as target:
            for chunk in response.iter_bytes():
                target.write(chunk)
    destination_path.chmod(mode)
    return destination_path
