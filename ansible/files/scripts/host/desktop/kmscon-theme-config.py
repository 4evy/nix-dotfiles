#!/usr/bin/env python3
from __future__ import annotations

import argparse
import datetime as dt
import json
import math
import os
import pathlib
import tempfile
from dataclasses import dataclass

ANSI_ROLE_MAP = {
    "black": "surface1",
    "red": "red",
    "green": "green",
    "yellow": "yellow",
    "blue": "blue",
    "magenta": "pink",
    "cyan": "teal",
    "light-grey": "subtext0",
    "dark-grey": "surface2",
    "light-red": "red",
    "light-green": "green",
    "light-yellow": "yellow",
    "light-blue": "blue",
    "light-magenta": "pink",
    "light-cyan": "teal",
    "white": "subtext1",
    "foreground": "text",
    "background": "base",
}

LATTE_ROLE_OVERRIDES = {
    "black": "subtext1",
    "white": "surface1",
    "light-grey": "surface2",
    "dark-grey": "subtext0",
}


@dataclass(frozen=True)
class ThemeChoice:
    name: str
    source: str


DEFAULT_LATITUDE = 42.6977
DEFAULT_LONGITUDE = 23.3219
SUN_ZENITH_DEGREES = 90.8333


def hex_to_rgb_csv(value: str) -> str:
    value = value.removeprefix("#")
    if len(value) != 6:
        raise ValueError(f"invalid hex color: {value!r}")
    return ",".join(str(int(value[index : index + 2], 16)) for index in (0, 2, 4))


def coordinate_from_env(name: str, minimum: float, maximum: float) -> float | None:
    value = os.environ.get(name)
    if not value:
        return None
    coordinate = float(value)
    if coordinate < minimum or coordinate > maximum:
        raise ValueError(
            f"{name} must be between {minimum} and {maximum}: {coordinate}"
        )
    return coordinate


def normalize_degrees(value: float) -> float:
    return value % 360.0


def normalize_hours(value: float) -> float:
    return value % 24.0


def sun_event_utc_hour(
    date: dt.date,
    latitude: float,
    longitude: float,
    *,
    sunrise: bool,
) -> float | None:
    day_of_year = date.timetuple().tm_yday
    longitude_hour = longitude / 15.0
    approximate_time = day_of_year + (
        ((6.0 if sunrise else 18.0) - longitude_hour) / 24.0
    )

    mean_anomaly = (0.9856 * approximate_time) - 3.289
    true_longitude = normalize_degrees(
        mean_anomaly
        + (1.916 * math.sin(math.radians(mean_anomaly)))
        + (0.020 * math.sin(math.radians(2.0 * mean_anomaly)))
        + 282.634
    )

    right_ascension = normalize_degrees(
        math.degrees(math.atan(0.91764 * math.tan(math.radians(true_longitude))))
    )
    longitude_quadrant = math.floor(true_longitude / 90.0) * 90.0
    ascension_quadrant = math.floor(right_ascension / 90.0) * 90.0
    right_ascension = (right_ascension + longitude_quadrant - ascension_quadrant) / 15.0

    sin_declination = 0.39782 * math.sin(math.radians(true_longitude))
    cos_declination = math.cos(math.asin(sin_declination))
    latitude_radians = math.radians(latitude)
    cos_hour_angle = (
        math.cos(math.radians(SUN_ZENITH_DEGREES))
        - (sin_declination * math.sin(latitude_radians))
    ) / (cos_declination * math.cos(latitude_radians))
    if cos_hour_angle > 1.0 or cos_hour_angle < -1.0:
        return None

    hour_angle = math.degrees(math.acos(cos_hour_angle))
    if sunrise:
        hour_angle = 360.0 - hour_angle
    hour_angle /= 15.0

    local_mean_time = (
        hour_angle + right_ascension - (0.06571 * approximate_time) - 6.622
    )
    return normalize_hours(local_mean_time - longitude_hour)


def local_sun_event(
    date: dt.date,
    timezone: dt.tzinfo,
    latitude: float,
    longitude: float,
    *,
    sunrise: bool,
) -> dt.datetime | None:
    utc_hour = sun_event_utc_hour(
        date,
        latitude,
        longitude,
        sunrise=sunrise,
    )
    if utc_hour is None:
        return None
    utc_midnight = dt.datetime.combine(date, dt.time(), tzinfo=dt.UTC)
    return (utc_midnight + dt.timedelta(hours=utc_hour)).astimezone(timezone)


def daylight_theme() -> ThemeChoice:
    forced = os.environ.get("DOTFILES_KMSCON_THEME")
    if forced:
        if forced not in {"latte", "frappe"}:
            raise ValueError(f"unsupported DOTFILES_KMSCON_THEME: {forced}")
        return ThemeChoice(forced, "DOTFILES_KMSCON_THEME")
    desktop_scheme = os.environ.get("COLOR_SCHEME") or os.environ.get("GTK_THEME")
    if desktop_scheme and "dark" in desktop_scheme.lower():
        return ThemeChoice("frappe", "desktop-env")
    if desktop_scheme and "light" in desktop_scheme.lower():
        return ThemeChoice("latte", "desktop-env")
    latitude = coordinate_from_env("DOTFILES_KMSCON_LATITUDE", -90.0, 90.0)
    longitude = coordinate_from_env("DOTFILES_KMSCON_LONGITUDE", -180.0, 180.0)
    if latitude is None and longitude is None:
        latitude = DEFAULT_LATITUDE
        longitude = DEFAULT_LONGITUDE
        location_source = "default-sofia"
    else:
        location_source = "env"
    if latitude is None or longitude is None:
        raise ValueError(
            "set both DOTFILES_KMSCON_LATITUDE and DOTFILES_KMSCON_LONGITUDE for sun-based theme selection"
        )
    now = dt.datetime.now().astimezone()
    if now.tzinfo is None:
        raise RuntimeError("local timezone is unavailable")
    sunrise = local_sun_event(
        now.date(),
        now.tzinfo,
        latitude,
        longitude,
        sunrise=True,
    )
    sunset = local_sun_event(
        now.date(),
        now.tzinfo,
        latitude,
        longitude,
        sunrise=False,
    )
    if sunrise is None or sunset is None:
        return ThemeChoice(
            "frappe", f"polar-day-night latitude={latitude} longitude={longitude}"
        )
    if sunset <= sunrise:
        if now < sunrise:
            sunrise -= dt.timedelta(days=1)
        else:
            sunset += dt.timedelta(days=1)
    theme = "latte" if sunrise <= now < sunset else "frappe"
    return ThemeChoice(
        theme, f"sun {location_source} latitude={latitude} longitude={longitude}"
    )


def render_config(palette: dict[str, dict[str, str]]) -> str:
    choice = daylight_theme()
    colors = palette[choice.name]
    role_map = dict(ANSI_ROLE_MAP)
    if choice.name == "latte":
        role_map.update(LATTE_ROLE_OVERRIDES)
    lines = [
        "# Managed by dotfiles.",
        f"# Theme: catppuccin-{choice.name}-pink",
        f"# Theme source: {choice.source}",
        "term=kmscon",
        "font-engine=freetype",
        "font-name=Noto Sans Mono",
        "font-size=18",
        "sb-size=10000",
        "mouse",
        "dpms-timeout=600",
        "palette=custom",
    ]
    for option, role in role_map.items():
        lines.append(f"palette-{option}={hex_to_rgb_csv(colors[role])}")
    return "\n".join(lines) + "\n"


def write_if_changed(path: pathlib.Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists() and path.read_text(encoding="utf-8") == text:
        return
    with tempfile.NamedTemporaryFile(
        "w", encoding="utf-8", dir=path.parent, delete=False
    ) as handle:
        handle.write(text)
        tmp_path = pathlib.Path(handle.name)
    tmp_path.chmod(0o644)
    tmp_path.replace(path)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("palette_json", type=pathlib.Path)
    parser.add_argument("output_config", type=pathlib.Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    with args.palette_json.open(encoding="utf-8") as handle:
        palette = json.load(handle)
    output_path = args.output_config
    write_if_changed(output_path, render_config(palette))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
