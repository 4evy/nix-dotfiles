#!/usr/bin/env python3
from __future__ import annotations

import argparse
import ast


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("binding_path")
    parser.add_argument("current_bindings")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    binding_path = args.binding_path
    current = args.current_bindings.strip()
    if current.startswith("@as "):
        current = current[4:].strip()

    try:
        bindings = ast.literal_eval(current)
    except (SyntaxError, ValueError):
        bindings = []

    if not isinstance(bindings, list):
        bindings = []

    bindings = [binding for binding in bindings if isinstance(binding, str)]
    if binding_path not in bindings:
        bindings.append(binding_path)

    print(repr(bindings))


if __name__ == "__main__":
    main()
