# Spark View

GPU inference dashboard for the Arbiter server on spark (10.0.0.254:8400).

## Architecture

- **Go + Gio** (immediate-mode desktop GUI)
- Polls `GET /v1/ps` with 3s sleep between requests for VRAM, model, and queue status
- Dark theme with cyan/green/purple accent palette
- Window size/position persistence via daz-golang-gio/persist
- Right-click context menu on rows via daz-golang-gio/menu (e.g. change max instances)
- Write operations use `PATCH /v1/models/{id}` (e.g. `{"max_instances": N}`)

## Build & Deploy

- `./run rebuild` — build, install the launcher, and restart via auto (the ONLY way to deploy changes)
- `./run install` — (re)install the supervising launcher to `~/bin/sparkview`
- Never launch the binary directly — always use auto for process management

## Supervision (why it never dies)

`auto` runs `~/bin/sparkview`, which is the supervising launcher
(`src/launcher/sparkview-launcher`), NOT the GUI binary directly. Two layers keep
Spark View alive:

1. **auto watch agent** (`com.darrenoakey.auto` LaunchAgent, KeepAlive/RunAtLoad)
   keeps the launcher process alive and brings it back after a reboot.
2. **The launcher** relaunches the GUI binary (`output/bin/sparkview`) on EVERY
   exit after a short delay.

This defeats the three historical death modes seen in `output/logs/sparkview/`:
- `Resource deadlock avoided` (EDEADLK) re-execing the Mach-O — a shell script
  never EDEADLKs, and it only relaunches the GUI *after* the prior instance has
  fully exited, so the overlapping-image race cannot occur.
- `panic: runtime/cgo: misuse of an invalid Handle` — a Gio teardown panic when no
  GUI session is available (boot/login, fast-user-switch, screen lock); the
  launcher just retries until a session exists.
- A clean window close — reopened, matching keep-alive intent.

If the launcher itself is loaded but the watch agent is not, restarts still happen
at the GUI level but not for the launcher process / reboots — verify with
`launchctl list | grep darrenoakey.auto`.

## Gio Gotchas

- `app.Main()` MUST stay on main goroutine; event loop in goroutine
- Widget state (`widget.Clickable`, `widget.List`) must persist across frames — store as struct fields
- `material.NewTheme()` created once, never per frame
- Never do blocking I/O in frame handlers on macOS — use goroutines
- Use `font.Bold` not `text.Bold`
