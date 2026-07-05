# system-clipboard.yazi

Copy selected files, or the hovered file when nothing is selected, to the system clipboard.

Supported backends:

- Linux Wayland: `wl-copy`
- macOS: `pbcopy`

Default behavior on Wayland is desktop-aware:

- GNOME-like sessions use `x-special/gnome-copied-files`, which Nautilus/GNOME Files expects for file copy and cut operations.
- KDE/Plasma, LXQt, and unknown window-manager sessions use `text/uri-list`, the standard URI-list format understood by many file managers.

`wl-copy` advertises one MIME type per clipboard owner, so the plugin chooses the best single format instead of pretending it can offer every file-manager-specific format at once. On macOS, `pbcopy` receives newline-separated paths or file URIs because `pbcopy` is a text clipboard tool.

## Keymap

```toml
{ on = "Y", run = "plugin system-clipboard", desc = "Copy selected files to system clipboard" }
```

Optional modes:

```toml
{ on = ["c", "p"], run = "plugin system-clipboard --paths", desc = "Copy paths as text" }
{ on = ["c", "u"], run = "plugin system-clipboard --uris", desc = "Copy file URIs as text" }
{ on = ["c", "g"], run = "plugin system-clipboard --gnome", desc = "Copy files for GNOME/Nautilus" }
{ on = ["c", "k"], run = "plugin system-clipboard --kde", desc = "Copy files as URI list" }
```
