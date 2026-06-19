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
- `./run logs` — tail the live log
- Never launch the binary directly — always use auto for process management
- **Builds produce a code-signed `output/SparkView.app`** (stable identity:
  bundle id `com.darrenoakey.sparkview` + Apple Development Team — see Local
  Network privacy below). The launcher runs the executable INSIDE the bundle. The
  loose `output/bin/sparkview` still exists for tests/dev. Signing identity is
  auto-detected (`signing_identity` in `run`); ad-hoc fallback only warns.

## Logging

The app logs to **stderr** (Go `log`, microsecond timestamps, `sparkview` prefix):
startup/pid, App Nap disabled, poll connect/disconnect/reconnect transitions
(NOT every poll — transitions only), right-click write actions and their
failures, and window close. `auto` captures stderr into a
timestamped per-launch file under
`~/local/auto/output/logs/sparkview/YYYY/MM/sparkview_*.log`. Use `./run logs`
to tail the most recent one; it is the place to look first when diagnosing a
freeze or a "server down" report.

## Supervision (why it never dies)

`auto` runs `~/bin/sparkview`, which is the supervising launcher
(`src/launcher/sparkview-launcher`), NOT the GUI binary directly. Two layers keep
Spark View alive:

1. **auto watch agent** (`com.darrenoakey.auto` LaunchAgent, KeepAlive/RunAtLoad)
   keeps the launcher process alive and brings it back after a reboot.
2. **The launcher** relaunches the GUI binary (`output/bin/sparkview`) on EVERY
   exit after a short delay.

This defeats the three historical death modes seen in the logs (see Logging below):
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

### The fourth death mode: alive-but-frozen (App Nap)

The launcher only relaunches on *exit*. A process that is alive but has stopped
updating slips past it. This is what App Nap caused: Spark View ships as a bare
Mach-O (no `.app`/Info.plist), so it has no `LSAppNapIsDisabled` key. When its
window sat in the background, macOS throttled the process — the 3s poll loop
stopped firing, and because an occluded window is never redrawn, the last frame
("server down") lingered. It looked dead while still running.

Defense: **App Nap is disabled** two ways — `LSAppNapIsDisabled` in the bundle
`Info.plist`, and at startup via an `NSProcessInfo` activity assertion
(`appnap_darwin.go`, `NSActivityUserInitiatedAllowingIdleSystemSleep`, token
retained for process life). The poll loop keeps running while backgrounded, so the
in-memory state stays fresh and the window draws current data the moment it is
revealed.

> There is deliberately **NO self-restart watchdog**. An earlier version exited
> the process to "self-heal" on poll-stall / no-route; it caused visible restart
> bouncing and was removed. The real causes are fixed at the source (App Nap
> disabled, signed bundle for Local Network, `wifi-auto` for multi-homing), so the
> app just runs. Supervision is only the launcher relaunching on an actual exit.

### The fifth death mode: stuck on "no route to host" (EHOSTUNREACH)

A dial to spark can fail instantly with `connect: no route to host`
(EHOSTUNREACH) — zero packets on the wire, so `curl` and fresh processes reach
spark fine while Spark View is stuck "down". Two distinct causes produce it, both
only affecting a *same-subnet* host like spark:

1. **Multi-homing.** Two active interfaces on one subnet (Ethernet `en7` + Wi-Fi
   `en0`, both 10.0.0.0/24) make macOS flap the route. The `wifi-auto` daemon
   (separate repo) powers Wi-Fi off while Ethernet is up to prevent this.
2. **macOS Local Network privacy (the recurring one).** Since macOS 15, LAN
   access is gated by TCC and attributed to the *responsible process* — here the
   long-lived launcher. Spark View is an **unsigned, bundle-less binary launched
   by a LaunchAgent**, so it cannot show the permission prompt; macOS allows it
   for a grace window (~80 min observed) then re-evaluates to **denied**, and
   every LAN dial then returns EHOSTUNREACH. Proof: the *same* binary connects
   from Terminal (inherits Terminal's grant) and when run directly/under a fresh
   process tree, but fails under the long-lived launcher; restarting the
   **launcher** (a fresh responsible process) recovers it. (Apple-stack apps
   don't hit this — they're signed/bundled and granted once.)

**Fixes (both at the source — no self-restart):**
- Multi-homing → the `wifi-auto` daemon powers Wi-Fi off while Ethernet is up.
- Local Network privacy → Spark View ships as a **code-signed `.app` with a stable
  identity** (see Build & Deploy), so macOS attributes LAN access to the bundle,
  not the launcher, and a one-time grant in System Settings → Privacy & Security →
  Local Network persists across rebuilds (DR is constant: `codesign -d -r-
  output/SparkView.app`).

If spark ever shows "down" while `curl http://10.0.0.254:8400/v1/ps` works from
the shell, check that Local Network grant first.

## Gio Gotchas

- `app.Main()` MUST stay on main goroutine; event loop in goroutine
- Widget state (`widget.Clickable`, `widget.List`) must persist across frames — store as struct fields
- `material.NewTheme()` created once, never per frame
- Never do blocking I/O in frame handlers on macOS — use goroutines
- Use `font.Bold` not `text.Bold`
