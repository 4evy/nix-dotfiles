#!/usr/bin/python


DOCUMENTATION = r"""
---
module: operation
short_description: Run a repository-owned dotfiles Python operation
version_added: "0.1.0"
description:
  - Executes the versioned machine interface exposed by C(dotfiles-scripts).
  - Keeps project dependencies in their isolated uv tool environment while
    returning native Ansible result fields.
options:
  executable:
    description: Absolute path to the C(dotfiles-scripts) executable.
    type: path
    required: true
  command:
    description:
      - Existing C(dotfiles-scripts) command and arguments.
      - Typer performs the same discovery and type conversion as the human CLI.
    type: list
    elements: str
    required: true
  context:
    description: Selected host and repository context passed to the operation.
    type: dict
    required: true
  chdir:
    description: Working directory used to execute the operation.
    type: path
  environment:
    description: Environment additions for the isolated operation process.
    type: dict
    default: {}
attributes:
  check_mode:
    support: full
    description: The Python operation receives check-mode state in its typed request.
  diff_mode:
    support: full
    description: Structured operation diffs are forwarded to Ansible unchanged.
author:
  - 4evy (@4evy)
"""

EXAMPLES = r"""
- name: Build a staged Kanata binary through the repository Python runtime
  evy.dotfiles.operation:
    executable: "{{ dotfiles_bin_dir }}/dotfiles-scripts"
    command: [host, keyboard, kanata-build]
    context:
      repo_root: "{{ dotfiles_repo_root }}"
      home: "{{ dotfiles_home_dir }}"
      system: "{{ ansible_facts.system }}"
      architecture: "{{ ansible_facts.architecture }}"
  register: kanata_build
"""

RETURN = r"""
data:
  description: Operation-specific structured result data.
  returned: success
  type: dict
  sample:
    installed: true
msg:
  description: Human-readable operation summary.
  returned: always
  type: str
warnings:
  description: Non-fatal warnings reported by the operation.
  returned: when present
  type: list
  elements: str
protocol:
  description: Machine protocol version used for this invocation.
  returned: always
  type: int
  sample: 1
"""

import json
import os
import pathlib

from ansible.module_utils.basic import AnsibleModule  # ty: ignore[unresolved-import]


def _request(module: AnsibleModule) -> str:
    payload = {
        "protocol": 1,
        "command": module.params["command"],
        "context": module.params["context"],
        "check": module.check_mode,
        "diff": bool(getattr(module, "_diff", False)),
    }
    return json.dumps(payload, separators=(",", ":"), sort_keys=True)


def _parse_response(
    module: AnsibleModule, stdout: str, stderr: str
) -> dict[str, object]:
    try:
        response = json.loads(stdout)
    except json.JSONDecodeError:
        module.fail_json(
            msg="dotfiles operation did not return valid JSON",
            stdout=stdout,
            stderr=stderr,
        )
        return {}
    if not isinstance(response, dict) or response.get("protocol") != 1:
        module.fail_json(
            msg="dotfiles operation returned an unsupported protocol response",
            stdout=stdout,
            stderr=stderr,
        )
        return {}
    return response


def run_module() -> None:
    module = AnsibleModule(
        argument_spec={
            "executable": {"type": "path", "required": True},
            "command": {"type": "list", "elements": "str", "required": True},
            "context": {"type": "dict", "required": True},
            "chdir": {"type": "path"},
            "environment": {"type": "dict", "default": {}},
        },
        supports_check_mode=True,
    )
    executable = module.params["executable"]
    if not pathlib.Path(executable).is_file() or not os.access(executable, os.X_OK):
        module.fail_json(
            msg=f"dotfiles operation executable is unavailable: {executable}"
        )
    rc, stdout, stderr = module.run_command(
        [executable, "_ansible-v1"],
        data=_request(module),
        binary_data=True,
        cwd=module.params["chdir"],
        environ_update=module.params["environment"],
        check_rc=False,
    )
    response = _parse_response(module, stdout, stderr)
    response["operation_stderr"] = stderr
    warnings = response.pop("warnings", [])
    if isinstance(warnings, list):
        for warning in warnings:
            module.warn(str(warning))
    if rc != 0 or response.pop("failed", False):
        module.fail_json(
            msg=response.pop("msg", None) or "dotfiles operation failed",
            rc=rc,
            **response,
        )
    module.exit_json(**response)


def main() -> None:
    run_module()


if __name__ == "__main__":
    main()
