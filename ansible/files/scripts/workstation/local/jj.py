from __future__ import annotations

import datetime as dt
import json
import os
import re
import subprocess
import sys
from typing import Any, cast

from workstation.errors import DotfilesError
from workstation.lib.commands import output, run, which
from workstation.lib.files import write_if_changed


def _jj() -> tuple[str, ...]:
    root = os.environ.get("JJ_WORKSPACE_ROOT")
    return ("jj", "-R", root) if root else ("jj",)


def _git(*args: str, check: bool = True) -> str:
    return output(("git", *args), check=check)


def _shallow() -> bool:
    return _git("rev-parse", "--is-shallow-repository", check=False) == "true"


def _github_repo(value: str) -> str | None:
    for prefix in ("git@github.com:", "ssh://git@github.com/", "https://github.com/"):
        if value.startswith(prefix):
            value = value.removeprefix(prefix)
            break
    value = value.split("/pull/", 1)[0].removesuffix(".git").strip("/")
    return value if value.count("/") == 1 else None


def _normalize_repo(value: str) -> str | None:
    return _github_repo(_git("remote", "get-url", value, check=False) or value)


def _gh_json(*args: str) -> dict[str, object]:
    if which("gh") is None:
        raise DotfilesError("jj-get: gh is required for PR numbers")
    return json.loads(output(("gh", *args)))


def _infer_pr_repo() -> str:
    info = _gh_json("repo", "view", "--json", "nameWithOwner,parent")
    parent = info.get("parent")
    if isinstance(parent, dict):
        parent_info = cast("dict[str, Any]", parent)
        owner_info = parent_info.get("owner")
        if isinstance(owner_info, dict):
            owner = owner_info.get("login")
            if owner and parent_info.get("name"):
                return f"{owner}/{parent_info['name']}"
    value = info.get("nameWithOwner")
    if not isinstance(value, str):
        raise DotfilesError("jj-get: could not infer GitHub repository")
    return value


def _fetch_url(repo: str) -> str:
    info = _gh_json("repo", "view", repo, "--json", "sshUrl,url")
    value = info.get("sshUrl") or (
        f"{info['url']}.git" if isinstance(info.get("url"), str) else None
    )
    if not isinstance(value, str):
        raise DotfilesError(f"jj-get: could not resolve fetch URL for {repo}")
    return value


def _canonical_url(value: str) -> str:
    return (
        value
        .removeprefix("git@github.com:")
        .removeprefix("ssh://git@github.com/")
        .removeprefix("https://github.com/")
        .removesuffix(".git")
    )


def _remote_for(url: str) -> str | None:
    target = _canonical_url(url)
    return next(
        (
            remote
            for remote in _git("remote").splitlines()
            if _canonical_url(_git("remote", "get-url", remote, check=False)) == target
        ),
        None,
    )


def _resolve_pr(number: str, repo_arg: str | None) -> None:
    repo = _normalize_repo(
        repo_arg or os.environ.get("JJ_GET_REPO") or _infer_pr_repo()
    )
    if repo is None:
        raise DotfilesError("jj-get: invalid GitHub repository")
    info = _gh_json(
        "pr",
        "view",
        number,
        "-R",
        repo,
        "--json",
        "baseRefName,headRefName,headRepository",
    )
    base, head = info.get("baseRefName"), info.get("headRefName")
    if not isinstance(base, str) or not isinstance(head, str):
        raise DotfilesError(f"jj-get: could not resolve PR {number} in {repo}")
    head_info = info.get("headRepository")
    head_repo = head_info.get("nameWithOwner") if isinstance(head_info, dict) else None
    remote = os.environ.get("JJ_GET_PR_REMOTE", "github-pr")
    if isinstance(head_repo, str) and head_repo:
        url = _fetch_url(head_repo)
        remote = _remote_for(url) or remote
        bookmark, remote_head = head, head
        refspec = f"+refs/heads/{head}:refs/remotes/{remote}/{head}"
    else:
        url = _fetch_url(repo)
        bookmark, remote_head = f"pr/{number}", number
        refspec = f"+refs/pull/{number}/head:refs/remotes/{remote}/{number}"
    args = ["git", "fetch"]
    if _shallow():
        args.append(f"--shallow-exclude=refs/heads/{base}")
    run((*args, "--prune", "--no-write-fetch-head", "--no-tags", "--", url, refspec))
    run((*_jj(), "git", "import"))
    if _shallow():
        run((*_jj(), "--quiet", "debug", "reindex"))
    run((
        *_jj(),
        "bookmark",
        "set",
        "--allow-backwards",
        bookmark,
        "-r",
        f"{remote_head}@{remote}",
    ))


def _infer_base(remote: str) -> str:
    value = _git(
        "symbolic-ref",
        "--quiet",
        "--short",
        f"refs/remotes/{remote}/HEAD",
        check=False,
    )
    if value:
        return value.removeprefix(f"{remote}/")
    for line in _git("ls-remote", "--symref", remote, "HEAD", check=False).splitlines():
        fields = line.split()
        if fields[:1] == ["ref:"] and len(fields) > 1:
            return fields[1].removeprefix("refs/heads/")
    raise DotfilesError("jj-get: could not infer default branch; pass BASE")


def _resolve_branch(bookmark: str, remote: str | None, base: str | None) -> None:
    if "@" in bookmark:
        bookmark, suffix = bookmark.rsplit("@", 1)
        base, remote = (remote or base), suffix
    if not remote:
        remotes = _git("remote").splitlines()
        remote = remotes[0] if len(remotes) == 1 else "origin"
    if not _git("remote", "get-url", remote, check=False):
        raise DotfilesError(f"jj-get: unknown remote: {remote}")
    if _shallow():
        base = base or os.environ.get("JJ_GET_BASE") or _infer_base(remote)
        base = base.removeprefix(f"{remote}/")
        base_ref = base if base.startswith("refs/") else f"refs/heads/{base}"
        refspec = f"+refs/heads/{bookmark}:refs/remotes/{remote}/{bookmark}"
        run((
            "git",
            "fetch",
            f"--shallow-exclude={base_ref}",
            "--prune",
            "--no-write-fetch-head",
            "--no-tags",
            "--",
            remote,
            refspec,
        ))
        run((*_jj(), "git", "import"))
        run((*_jj(), "--quiet", "debug", "reindex"))
    else:
        run((*_jj(), "git", "fetch", "--remote", remote, "--branch", bookmark))
    run((*_jj(), "bookmark", "track", f"{bookmark}@{remote}"))


def jj_get_entrypoint() -> None:
    args = sys.argv[1:]
    if args[:1] in (["-h"], ["--help"]):
        print("usage: jj-get TARGET [REMOTE_OR_REPO] [BASE]")
        return
    if not 1 <= len(args) <= 3 or any(value.startswith("-") for value in args):
        raise SystemExit("usage: jj-get TARGET [REMOTE_OR_REPO] [BASE]")
    is_pr_number = args[0].isdigit()
    is_pr_url = re.fullmatch(
        r"https://github\.com/([^/]+)/([^/]+)/pull/(\d+)(?:/.*)?", args[0]
    )
    if (is_pr_number and len(args) > 2) or (is_pr_url and len(args) > 1):
        raise SystemExit("usage: jj-get TARGET [REMOTE_OR_REPO] [BASE]")
    if not _git("rev-parse", "--git-dir", check=False):
        raise SystemExit("jj-get: this requires a colocated Git repository")
    try:
        if is_pr_number:
            _resolve_pr(args[0], args[1] if len(args) == 2 else None)
        elif is_pr_url:
            _resolve_pr(
                is_pr_url.group(3),
                f"{is_pr_url.group(1)}/{is_pr_url.group(2)}",
            )
        else:
            _resolve_branch(
                args[0],
                args[1] if len(args) > 1 else None,
                args[2] if len(args) > 2 else None,
            )
    except DotfilesError as error:
        raise SystemExit(str(error)) from error


def _shim_state(value: str) -> None:
    if path := os.environ.get("JJ_GIT_FETCH_SHIM_STATE"):
        write_if_changed(path, value + "\n")


def jj_git_fetch_entrypoint() -> None:
    _shim_state("delegate")
    args = list(sys.argv[1:])
    if (
        os.environ.get("JJ_GIT_FETCH_SHALLOW_SHIM", "1") == "0"
        or args[:2] != ["git", "fetch"]
        or not _shallow()
    ):
        return
    remotes: list[str] = []
    branches: list[str] = []
    all_remotes = explicit = False
    args = args[2:]
    index = 0
    while index < len(args):
        value = args[index]
        if value == "--remote" and index + 1 < len(args):
            remotes.append(args[index + 1])
            index += 2
        elif value.startswith("--remote="):
            remotes.append(value.split("=", 1)[1])
            index += 1
        elif value in {"-b", "--branch", "--bookmark"} and index + 1 < len(args):
            branches.append(args[index + 1])
            explicit = True
            index += 2
        elif value.startswith(("--branch=", "--bookmark=")):
            branches.append(value.split("=", 1)[1])
            explicit = True
            index += 1
        elif value == "--all-remotes":
            all_remotes = True
            index += 1
        else:
            return
    if all_remotes and remotes:
        return
    if all_remotes:
        remotes = _git("remote").splitlines()
    elif not remotes:
        found = _git("remote").splitlines()
        if len(found) == 1:
            remotes = found
        elif _git("remote", "get-url", "origin", check=False):
            remotes = ["origin"]
        else:
            return
    if any(re.search(r"[*?|~()]", value) for value in (*remotes, *branches)):
        return
    _shim_state("handled")
    for remote in remotes:
        refspecs = (
            [
                f"+refs/heads/{branch}:refs/remotes/{remote}/{branch}"
                for branch in branches
            ]
            or _git(
                "config", "--get-all", f"remote.{remote}.fetch", check=False
            ).splitlines()
            or [f"+refs/heads/*:refs/remotes/{remote}/*"]
        )
        no_tags = ("--no-tags",) if explicit else ()
        run((
            "git",
            "fetch",
            f"--depth={os.environ.get('JJ_GIT_FETCH_DEPTH', '1')}",
            "--prune",
            "--no-write-fetch-head",
            "--verbose",
            "--progress",
            *no_tags,
            "--",
            remote,
            *refspecs,
        ))
    run(("jj", "git", "import"))


def _redate_args(args: list[str]) -> list[str]:
    result: list[str] = []
    while args:
        value = args.pop(0)
        if value in {"-h", "--help"}:
            raise SystemExit("usage: jj-redate [-r REVSET] [REVSETS]...")
        if value in {"-r", "--revision"}:
            if not args:
                raise SystemExit("jj-redate: --revision requires a value")
            result.append(args.pop(0))
        elif value.startswith("--revision="):
            result.append(value.split("=", 1)[1])
        elif value == "--":
            result.extend(args)
            break
        elif value.startswith("-"):
            raise SystemExit(f"jj-redate: unknown option: {value}")
        else:
            result.append(value)
    return result or ["@"]


def _prompt(label: str, default: str) -> str:
    use_gum = (
        not os.environ.get("JJ_REDATE_NO_GUM")
        and sys.stdin.isatty()
        and sys.stdout.isatty()
        and which("gum") is not None
    )
    if use_gum:
        return output(("gum", "input", "--prompt", label, "--value", default))
    try:
        return input(label) or default
    except EOFError:
        if sys.stdin.isatty():
            return default
        raise DotfilesError(f"no input received for {label}") from None


def _timestamp() -> str:
    now = dt.datetime.now().astimezone()
    date_value = _prompt("Date (YYYY-MM-DD): ", now.strftime("%Y-%m-%d"))
    time_value = _prompt("Time (HH[:MM[:SS]]): ", now.strftime("%H:%M:%S"))
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}", date_value):
        raise DotfilesError(f"invalid date: {date_value!r}")
    if re.fullmatch(r"\d{1,2}", time_value):
        time_value += ":00:00"
    elif re.fullmatch(r"\d{1,2}:\d{2}", time_value):
        time_value += ":00"
    elif not re.fullmatch(r"\d{1,2}:\d{2}:\d{2}", time_value):
        raise DotfilesError(f"invalid time: {time_value!r}")
    try:
        value = dt.datetime.strptime(
            f"{date_value} {time_value} {now:%z}",
            "%Y-%m-%d %H:%M:%S %z",
        )
    except ValueError as error:
        raise DotfilesError(str(error)) from error
    return value.astimezone().isoformat(timespec="seconds")


def _confirm_redate(revisions: list[str], timestamp: str) -> bool:
    label = " ".join(revisions)
    if (
        not os.environ.get("JJ_REDATE_NO_GUM")
        and sys.stdin.isatty()
        and sys.stdout.isatty()
        and which("gum") is not None
    ):
        return (
            run(
                (
                    "gum",
                    "confirm",
                    f"Set author and committer timestamp on {label} to {timestamp}?",
                ),
                check=False,
                capture=True,
            ).returncode
            == 0
        )
    print(
        f"Setting author and committer timestamp on {label} to {timestamp}",
        file=sys.stderr,
    )
    return True


def _log(revset: str, template: str, reverse: bool = False) -> str:
    args = [*_jj(), "--color", "never", "--no-pager", "log", "-r", revset]
    if reverse:
        args.append("--reversed")
    return output((*args, "--no-graph", "--template", template))


def _timestamp_run(timestamp: str, *args: str) -> None:
    run(
        (*_jj(), "--config", f'debug.commit-timestamp="{timestamp}"', *args),
        env={"JJ_TIMESTAMP": timestamp},
    )


def _verify(ids: list[str], timestamp: str) -> bool:
    template = (
        'author.timestamp().format("%Y-%m-%dT%H:%M:%S%:z") ++ "\\t" ++ '
        'committer.timestamp().format("%Y-%m-%dT%H:%M:%S%:z") ++ "\\n"'
    )
    return all(
        all(
            line.split("\t") == [timestamp, timestamp]
            for line in _log(f"change_id({change})", template).splitlines()
        )
        for change in ids
    )


def _rewrite_commit(commit: str, timestamp: str) -> str:
    value = dt.datetime.fromisoformat(timestamp)
    replacement = f" {int(value.timestamp())} {value.strftime('%z')}".encode()
    git = which("git")
    if git is None:
        raise DotfilesError("required command is not available: git")
    raw = subprocess.check_output((git, "cat-file", "commit", commit))
    raw = re.sub(
        rb"^(author .+?) \d+ [+-]\d{4}$", rb"\1" + replacement, raw, flags=re.MULTILINE
    )
    raw = re.sub(
        rb"^(committer .+?) \d+ [+-]\d{4}$",
        rb"\1" + replacement,
        raw,
        flags=re.MULTILINE,
    )
    return (
        subprocess
        .check_output((git, "hash-object", "-t", "commit", "-w", "--stdin"), input=raw)
        .decode()
        .strip()
    )


def _git_fallback(ids: list[str], timestamp: str) -> None:
    for change in ids:
        for commit in _log(f"change_id({change})", 'commit_id ++ "\\n"').splitlines():
            refs = _git(
                "for-each-ref",
                "--format=%(objectname)%09%(refname:strip=2)",
                "refs/heads",
            ).splitlines()
            bookmark = next(
                (
                    line.split("\t", 1)[1]
                    for line in refs
                    if line.startswith(f"{commit}\t")
                ),
                "",
            )
            temporary = not bookmark
            if temporary:
                base = f"jj-redate-tmp-{change[:12]}-{commit[:12]}"
                bookmark = base
                index = 0
                while (
                    run(
                        (
                            "git",
                            "show-ref",
                            "--verify",
                            "--quiet",
                            f"refs/heads/{bookmark}",
                        ),
                        check=False,
                        capture=True,
                    ).returncode
                    == 0
                ):
                    index += 1
                    bookmark = f"{base}-{index}"
                run(("git", "update-ref", f"refs/heads/{bookmark}", commit))
                run((*_jj(), "--quiet", "git", "import"))
            new_commit = _rewrite_commit(commit, timestamp)
            run(("git", "update-ref", f"refs/heads/{bookmark}", new_commit, commit))
            run((*_jj(), "--quiet", "git", "import"))
            if temporary:
                run((*_jj(), "--quiet", "bookmark", "forget", bookmark))


def jj_redate_entrypoint() -> None:
    try:  # noqa: PLW0717 - one recovery boundary must restore descendant timestamps
        arguments = list(sys.argv[1:])
        if any(value in {"-h", "--help"} for value in arguments):
            print("usage: jj-redate [-r REVSET] [REVSETS]...")
            return
        revisions = _redate_args(arguments)
        revset = " | ".join(f"({value})" for value in revisions)
        timestamp = _timestamp()
        if not _confirm_redate(revisions, timestamp):
            return
        ids = _log(revset, 'change_id ++ "\\n"', True).splitlines()
        descendants = _log(
            f"({revset}):: ~ ({revset})",
            'change_id ++ "\\t" ++ committer.timestamp().format("%Y-%m-%dT%H:%M:%S%.3f%:z") ++ "\\n"',
            True,
        )
        edited = False
        try:
            _timestamp_run(
                timestamp,
                "metaedit",
                "--author-timestamp",
                timestamp,
                "--force-rewrite",
                "-r",
                revset,
            )
            edited = True
            if not _verify(ids, timestamp):
                _git_fallback(ids, timestamp)
                if not _verify(ids, timestamp):
                    raise DotfilesError("jj-redate: timestamp verification failed")
        finally:
            if edited:
                for line in descendants.splitlines():
                    change, original = line.split("\t", 1)
                    _timestamp_run(
                        original,
                        "--quiet",
                        "metaedit",
                        "--force-rewrite",
                        "-r",
                        f"change_id({change})",
                    )
    except DotfilesError as error:
        raise SystemExit(str(error)) from error
