from __future__ import annotations

import importlib
import sys
import shutil
from collections.abc import Iterable
from functools import cached_property
from typing import Any

from spectrum_build.core.common import CommandRunner, fail


def _is_repository_package_spec(package: str) -> bool:
    return (
        "://" not in package
        and not package.endswith(".rpm")
        and not package.startswith("/")
    )


class Dnf:
    def __init__(self, runner: CommandRunner) -> None:
        self.runner = runner

    @cached_property
    def command(self) -> tuple[str, ...]:
        return self._find_command()

    def install(
        self,
        packages: Iterable[str],
        *,
        optional: bool = False,
        nogpgcheck: bool = False,
    ) -> None:
        packages = tuple(packages)
        if not packages:
            return

        if (
            self.libdnf5 is not None
            and not optional
            and not nogpgcheck
            and all(_is_repository_package_spec(package) for package in packages)
        ):
            self._install_with_libdnf5(packages)
            return

        self._install_with_cli(
            packages,
            optional=optional,
            nogpgcheck=nogpgcheck,
        )

    def clean(self) -> None:
        self.runner.run([*self.command, "clean", "all"])

    @cached_property
    def libdnf5(self) -> Any | None:
        try:
            return importlib.import_module("libdnf5")
        except ImportError:
            return None

    def _install_with_cli(
        self,
        packages: tuple[str, ...],
        *,
        optional: bool,
        nogpgcheck: bool,
    ) -> None:
        self.runner.run(
            [
                *self.command,
                "-y",
                "install",
                "--setopt=install_weak_deps=False",
                *(("--skip-unavailable",) if optional else ()),
                *(("--nogpgcheck",) if nogpgcheck else ()),
                *packages,
            ]
        )

    def _install_with_libdnf5(self, packages: tuple[str, ...]) -> None:
        libdnf5 = self.libdnf5
        if libdnf5 is None:
            fail("libdnf5 Python bindings are not available")

        print(f"+ libdnf5 install {' '.join(packages)}", file=sys.stderr)
        try:
            base = self._libdnf5_base(libdnf5)
            goal = libdnf5.base.Goal(base)
            for package in packages:
                goal.add_install(package)

            transaction = goal.resolve()
            self._report_transaction(transaction)
            transaction.download()
            result = transaction.run()
        except Exception as error:
            fail(f"libdnf5 failed to install packages {', '.join(packages)}: {error}")

        if result is not None and self._transaction_failed(libdnf5, result):
            fail(
                "libdnf5 transaction failed: "
                f"{self._transaction_result(libdnf5, result)}"
            )

    @staticmethod
    def _libdnf5_base(libdnf5: Any) -> Any:
        base = libdnf5.base.Base()
        base.load_config()
        config = base.get_config()
        config.get_install_weak_deps_option().set(False)
        base.setup()

        repo_sack = base.get_repo_sack()
        repo_sack.create_repos_from_system_configuration()
        repo_sack.load_repos()
        return base

    @staticmethod
    def _report_transaction(transaction: Any) -> None:
        packages = getattr(transaction, "get_transaction_packages", lambda: ())()
        for package in packages:
            item = package.get_package()
            print(f"  {package.get_action()} {item.get_nevra()}", file=sys.stderr)

    @staticmethod
    def _transaction_failed(libdnf5: Any, result: Any) -> bool:
        success = getattr(libdnf5.base, "TransactionRunResult_SUCCESS", None)
        return success is not None and result != success

    @staticmethod
    def _transaction_result(libdnf5: Any, result: Any) -> str:
        result_to_string = getattr(libdnf5.base, "transaction_result_to_string", None)
        if result_to_string is not None:
            return str(result_to_string(result))
        return str(result)

    @staticmethod
    def _find_command() -> tuple[str, ...]:
        if command := shutil.which("dnf5") or shutil.which("dnf"):
            return (command,)
        fail("required command not found: dnf5 or dnf")
