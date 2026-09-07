# Spark View

GPU inference dashboard for the Arbiter server on spark (10.0.0.254:8400).

## Architecture

- **Go + Gio** (immediate-mode desktop GUI)
- Polls `GET /v1/ps` with 3s sleep between requests for VRAM, model, and queue status
- The optional `active_jobs_detail` /v1/ps panel feeds inline caller-provenance
  badges on the model table itself — no separate section, so it costs zero
  extra vertical space. `modelProvenance()` picks one representative live job
  per model (why wins over who when both/several jobs exist); `provenanceLabel()`
  renders it as a compact amber pill in the model row's name cell, next to the
  existing host badge. Idle models (no live job) render exactly as before.
  Absent/empty `active_jobs_detail` (older server, or truly idle system) simply
  means no badges anywhere — the table layout never changes shape.
- Dark theme with cyan/green/purple accent palette
- Window size/position persistence via daz-golang-gio/persist
- Right-click context menu on rows via daz-golang-gio/menu (e.g. change max instances)
- Write operations use `PATCH /v1/models/{id}` (e.g. `{"max_instances": N}`)

## Build & Deploy

- `./run rebuild` — build the signed bundle, (re)install the LaunchAgent, restart (the ONLY way to deploy changes)
- `./run install` — write/load `~/Library/LaunchAgents/com.darrenoakey.sparkview.plist`
- `./run logs` — tail the live log
- **Builds produce a code-signed `output/SparkView.app`** (stable identity:
  bundle id `com.darrenoakey.sparkview` + Apple Development Team — see Local
  Network privacy below). The loose `output/bin/sparkview` still exists for
  tests/dev. Signing identity is auto-detected (`signing_identity` in `run`);
  ad-hoc fallback only warns (ad-hoc breaks the persistent Local Network grant).

## Logging

The app logs to **stderr** (Go `log`, microsecond timestamps, `sparkview` prefix):
startup/pid, App Nap disabled, poll connect/disconnect/reconnect transitions
(NOT every poll — transitions only), right-click write actions and their
failures, and window close. The LaunchAgent redirects stderr to
`~/local/auto/output/logs/sparkview/launchagent.log`. Use `./run logs` to tail it;
it is the place to look first when diagnosing a freeze or a "server down" report.

## Supervision (how it runs)

Spark View runs as its **own LaunchAgent** — `com.darrenoakey.sparkview`
(template in `src/launchagent/`, installed to `~/Library/LaunchAgents/`).
`launchd` execs the signed `.app` binary **directly** (no bash launcher, NOT under
`auto`), with `KeepAlive`/`RunAtLoad`, `LimitLoadToSessionType=Aqua`.

Why this exact shape — it is load-bearing for the Local Network grant (see below):
launching the binary directly under launchd makes Spark View its **own
responsible process**, so macOS attributes Local Network access to the bundle's
signed identity. An intermediary (the old `auto` → bash-launcher chain) becomes
the responsible process instead, and the LAN grant never sticks.

`KeepAlive` relaunches on an *actual* exit (crash, Gio teardown panic, clean
window-close) — launchd throttles respawns (~10s min). There is deliberately **NO
self-restart watchdog**; an earlier version exited the process to "self-heal" on
poll-stall / no-route and just produced visible restart *bouncing*. The real
causes are fixed at the source (below), so the app simply runs.

Verify it is loaded: `launchctl print gui/$(id -u)/com.darrenoakey.sparkview`.

### Death mode: alive-but-frozen (App Nap)

A bare Mach-O has no `LSAppNapIsDisabled` key, so when the window sat in the
background macOS throttled the process — the 3s poll loop stopped and the occluded
window never redrew, lingering on a stale "server down" frame while still alive.
**App Nap is disabled** two ways now: `LSAppNapIsDisabled` in the bundle
`Info.plist`, and at startup via an `NSProcessInfo` activity assertion
(`appnap_darwin.go`, `NSActivityUserInitiatedAllowingIdleSystemSleep`, token held
for process life). The poll loop keeps running backgrounded; the window draws
current data the moment it is revealed.

### The fifth death mode: stuck on "no route to host" (EHOSTUNREACH)

A dial to spark can fail instantly with `connect: no route to host`
(EHOSTUNREACH) — zero packets on the wire, so `curl` and fresh processes reach
spark fine while Spark View is stuck "down". Two distinct causes produce it, both
only affecting a *same-subnet* host like spark:

1. **Multi-homing.** Two active interfaces on one subnet (Ethernet `en7` + Wi-Fi
   `en0`, both 10.0.0.0/24) make macOS flap the route. The `wifi-auto` daemon
   (separate repo) powers Wi-Fi off while Ethernet is up to prevent this.
2. **macOS Local Network privacy (the recurring one).** Since macOS 15, LAN
   access is gated by TCC and attributed to the *responsible process*. Originally
   Spark View was an **unsigned, bundle-less binary run via a bash launcher under
   `auto`** — so the responsible process was the launcher/auto chain (un-grantable,
   never even listed in System Settings → Local Network). macOS allowed it for a
   grace window (~80–95 min observed) then re-evaluated to **denied**, and every
   LAN dial returned EHOSTUNREACH. Proof: the *same* binary connects from Terminal
   (inherits Terminal's grant) and when run directly via LaunchServices/launchd,
   but fails under the bash launcher.

**Fix — TWO things, both required:**
1. **Stable signed identity** — Spark View ships as a code-signed `.app` (see
   Build & Deploy), so the grant can be recorded against `com.darrenoakey.sparkview`
   and **persists across rebuilds** (DR is constant: `codesign -d -r- output/SparkView.app`).
2. **Launch so the app is its own responsible process** — its own LaunchAgent
   execs the bundle binary directly (see Supervision). Only then does macOS list
   "Spark View" in System Settings → Privacy & Security → Local Network, where it
   was granted **once**. Running via a bash launcher / under `auto` breaks this —
   that was the whole bug.

Multi-homing is handled separately by the `wifi-auto` daemon (Wi-Fi off while
Ethernet is up).

If spark ever shows "down" while `curl http://10.0.0.254:8400/v1/ps` works from
the shell: check `launchctl print gui/$(id -u)/com.darrenoakey.sparkview` is
loaded, and that **Spark View** is still enabled in System Settings → Privacy &
Security → Local Network.

## Stale telemetry must never look live

A failed poll leaves the last good `Status` in memory on purpose — `Refresh`
only overwrites `a.status` on success, so the numbers you saw a second before a
network drop are still there. That history is useful, but the status bar renders
it in the SAME frame as the giant red `SERVER DOWN` panel, and the numeric half
wins: a frozen `VRAM 24 (80) / 90 GB · GPU 0% · 1 run` is indistinguishable
from a live idle GPU.

That exact frame was misread on **2026-08-12**: spark had been dead and off the
network for 84 minutes (`connect: host is down`, ARP incomplete even from the
Mac mini) and the bar was read as "something is running but the GPU is at 0%,
so it's just not working" — a software diagnosis for a box that had
hard-crashed on unified-memory exhaustion.

Two rules, both enforced in code:

1. **Every value painted from `a.status` passes through `mutedIfStale(clr,
   connected)`** — gauges, VRAM/GPU numbers, queue counts, remote-host dots and
   labels. Stale reads as neutral grey; only a live poll gets accent colour.
   `offline` itself turns red. When adding a status-bar widget, thread `live`
   into it; a new widget that ignores it silently reintroduces the lie.
2. **`lastGood` (not `lastRefresh`) is the age of the data.** `lastRefresh`
   advances on failed polls too, so it can never answer "how old is what I am
   looking at". The `SERVER DOWN` panel states it outright via `lastDataText`:
   `Last data 17:08:11 — 1h 24m ago`, and stays silent when no poll ever
   succeeded (nothing stale to explain).

Verify a change to any of this against a **local** throwaway Arbiter that you
then kill — that reproduces the frozen-bar state in seconds and needs no spark.

## Gio Gotchas

- `app.Main()` MUST stay on main goroutine; event loop in goroutine
- Widget state (`widget.Clickable`, `widget.List`) must persist across frames — store as struct fields
- `material.NewTheme()` created once, never per frame
- Never do blocking I/O in frame handlers on macOS — use goroutines
- Use `font.Bold` not `text.Bold`
