# Native player protocol

The numbered gestures that decide whether `theia-player` behaves, written after
the phase-0 campaign of 15 September 2026 measured the shipped build and found
three things no earlier check looked at: the OSD declared no `@font-face` at all,
its timeline measured 24px against the 44×44 floor, and the window-level
shortcuts swallowed `k` and Space while somebody typed a server address — `l`
switching the interface to English mid-address. The findings, with their
measurements, are in the phase-0 diagnostic; this file is the gesture list that
phase 2 is accepted against.

**Register what was seen, not what was intended.** A gesture with no observation
is recorded as `non vérifié`, and a gesture proves nothing outside the class it
belongs to.

## The three classes of evidence, which are not interchangeable

| Class | Means | Proves | Cannot prove |
|---|---|---|---|
| **Simulation** | Chromium, with `window.__TAURI__` replaced (`player/ui/scripts/render-check.mjs`) | what the OSD page draws, measures and answers | nothing about WebView2, mpv, or the window |
| **Automation** | the real program, driven by command-line flags and its own `--diagnostics` | the engine path, the session, the status the OSD reads | anything a person has to look at |
| **Manual observation** | a person in front of the running window | the cursor, composition over moving picture, focus the eye can see | nothing reproducible by a machine, unless photographed |

The cursor belongs entirely to the third class. `scripts/capture-window.ps1`
photographs the screen with `CopyFromScreen`, which **does not draw the pointer**:
its absence from a PNG proves nothing at all. Any cursor verdict needs either a
direct look, or a Win32 query (`GetCursorInfo`) taken while the window is
foreground.

## Fixtures

```powershell
# A playable library in a throwaway directory. 4 films, 1 series, 45 s each.
go run ./internal/testfixture --data-dir <dir>

# The engine, pinned and digest-verified. Never fetched from a running release.
./build-player.ps1 -Release -Bundle
# or, for a development build:
$env:THEIA_LIBMPV = '<repo>\dist\theia-player-windows-amd64\libmpv-2.dll'
```

The fixture's films are 45 seconds long. **Any gesture that needs a measurement
longer than that — 60 s of playback, a seek past the end, a resume — uses a real
file from the folder the maintainer authorised (`C:\Users\starx\Documents\Films`),
scanned into an isolated data directory.** A fixture cannot stand in for that.

Run everything at the window size a person actually has, and say which it was:
on this machine (200 % scaling) a 1100×700 window is **550×350 CSS pixels**. The
Tauri window accepts down to `minWidth` 640 × `minHeight` 360 physical, which is
**320×180 CSS** here — a size the phase-0 measurements found overflowing.

## The gestures

Each one names its class, its gesture, its timing, and the result that counts as
passing. `Expected` is written as what a person would see or what a measurement
would read.

### The pointer

| # | Class | Gesture | Timing | Expected |
|---|---|---|---|---|
| G1 | Manual | Open the player with no server running, so the connect screen is up. Move the pointer across the window without clicking. | immediately | **The pointer is visible and moves.** |
| G2 | Manual | Rest the pointer on the address field, then on each of the visible buttons. | immediately | The pointer is an I-beam over the field and a hand over every button. |
| G3 | Manual | Start a film, let the furniture hide, then move the pointer. | within 0.5 s | The pointer and the furniture come back together — one sign of life, one result. |
| G4 | Manual | With the furniture hidden, click once on the picture. | immediately | Playback toggles **once**; the pointer reappears with the furniture. |
| G5 | Manual | Pause the film, then leave the pointer still for 5 s. | 5 s | The pointer stays visible. A paused player with an invisible pointer is a player nobody can restart. |
| G5b | Manual | Play a film long enough that it does not reach its end, move the pointer once, then leave it still and watch for 30 s. | 30 s | **The pointer stays hidden for the whole idle period.** Measured on 16 September 2026 and **not achieved**: the native hide is called with the right arguments — traced in the player's own stderr — and the pointer was reported hidden during the idle window, but it does not stay hidden. Traces show hide and restore cycling, and `GetCursorInfo` read "showing" in 19 samples out of 20. The mechanism works; its persistence does not, and the cause is not established. |

### The furniture's three seconds

| # | Class | Gesture | Timing | Expected |
|---|---|---|---|---|
| G6 | Simulation + Manual | Start a film and move the pointer over the picture. Look at the bar. | at 0 s and 2.5 s | Visible both times. |
| G7 | Simulation + Manual | Same, and wait without touching anything. | at 3.5 s | Hidden. (The simulation asserts 2.5 s visible / 3.5 s hidden; a person confirms the same window.) |
| G8 | Simulation + Manual | Move the pointer after G7. | within 0.5 s | Furniture and pointer back. |
| G9 | Simulation + Manual | Pause, then wait 5 s. | 5 s | Furniture up, and it never leaves. |
| G10 | Manual | Drag the scrub bar, hold still mid-drag for 5 s, then release. | 5 s during | Furniture up throughout the drag. |
| G11 | Manual | Open the track menu, then wait 5 s without moving. | 5 s | Menu up. Nothing hides while a menu the viewer opened is on screen. |
| G12 | Manual | Trigger the audio-fallback notice (a film whose bitstream the endpoint refuses), then wait. | 5 s | The notice stays long enough to read; it is not swept away by the idle timer. |

### Keyboard and input

| # | Class | Gesture | Timing | Expected |
|---|---|---|---|---|
| G13 | Simulation + Manual | Put the caret in the address field and type `http://127.0.0.1:8395/klfmc` at reading speed. | — | **The field holds exactly that string.** Fixed on 15 September 2026, when it held `…/lfmc` — `k` was swallowed — and the interface switched to English on `l`. Asserted by the render check. |
| G14 | Simulation | Type the same address and watch the commands the OSD issues. | — | No playback command is issued while typing. Fixed at the same time, when typing issued `player_toggle_pause` and `player_set_muted`. Asserted. |
| G15 | Simulation | With a film playing, click each visible control exactly once. | 150 ms between | **One press, one command.** No control issues two state changes. |
| G16 | Simulation + Manual | Open the track menu, then press the arrow keys. | — | The menu stays open and **no seek happens** — the film does not move. |
| G17 | Simulation + Manual | Press `Échap` with the menu open, then again. | — | First press closes the menu and returns focus to the button that opened it (D2 = A: the menu closes before the player does). Second closes the player. |
| G18 | Automation | Type a complete address and submit it. | — | The library panel lists the films, and `--diagnostics` reports a connected session. A submit that silently does nothing is the failure this catches. |

### Discovery

| # | Class | Gesture | Timing | Expected |
|---|---|---|---|---|
| G19 | Automation | Start with **no** server on the network. Press "Chercher un serveur". | within 10 s | An empty list and the sentence that says so. No error, no spinner left running. |
| G20 | Automation | Start with **exactly one** server announcing itself. Press "Chercher un serveur". | within 10 s | It connects. |
| G21 | Automation | Start with **two or more** servers. Press "Chercher un serveur". | within 10 s | Both are listed with a name and an address, and choosing one connects. |
| G22 | Automation | Type an address that answers nothing (a closed port, a wrong host). | within 10 s | A catalogued sentence, in the active language, that says the connection failed. No raw syscall name, no English in the French interface. |

**G19–G21 cannot be proven by a mock.** mDNS on this machine answers nothing at
all and the Go responder refuses an IPv6 multicast bind (`docs/v3.3.md`), so a
faked discovery list proves the drawing and nothing about the network. Any claim
about finding a server on the LAN needs a second real machine or a real
announcement, and says so.

### Full screen, composition, and the way out

| # | Class | Gesture | Timing | Expected |
|---|---|---|---|---|
| G23 | Manual | Press the full-screen control. | — | The window fills the screen, and the picture with it. |
| G24 | Manual | In full screen, press `Échap`. | — | Leaves full screen **first**; the player stays open. A second `Échap` closes it. This sequence is the one D2 asked the maintainer to validate. |
| G25 | Manual | Play a film and look at the OSD over moving picture. | 30 s | The film is visible through the OSD, the scrims carry the text, and **the desktop never shows through** — the failure `force-window` was added for. |
| G25b | Manual | Let a film play to its end and wait. | 30 s | Something says the film is over and offers a way on. Measured on 16 September 2026: the engine keeps the last frame with `keep-open` and reports `pause: true`, so the furniture stays up and the picture freezes. Whether that is the intended end-of-film state, or a dead end that needs a sentence or a return to the library, is a decision rather than a defect, and it is recorded here as one. |
| G26 | Manual | With a film playing, look at the whole frame at 550×350 CSS. | — | Nothing is cut off: the whole control row and the clock are inside the window. At 320×180 CSS the row is measured to overflow today, cutting the close control. |

### What this machine cannot decide

Named so that no run implies otherwise: there is **no AVR and no HDMI audio
path** here — one WASAPI endpoint, the Realtek speakers. A bitstream passthrough
request can be observed being made and observed being refused; it cannot be
observed *succeeding*. Nothing here proves Atmos or DTS-HD MA reaching an
amplifier, Dolby Vision profile 7, another operating system, or legibility at
three metres.

## Recording a run

One line per gesture, in the campaign's journal:

| Field | Content |
|---|---|
| Gesture | `G7` |
| Class | simulation / automation / manual |
| Environment | commit, binary path and digest, engine digest, window size **in CSS pixels and DPI**, fixture or real file |
| Exact command or gesture | the command line, or the keys and clicks in order |
| Result | what was observed, with the number if there is one |
| Status | verified / failed / not verified / awaiting decision |
| Evidence | path to the PNG, the log, or the JSON |
| Limit | what this result does not prove |

A gesture is repeated only after a change, a failure, or a new doubt. A run that
reports "everything works" without naming what was not tested is the one result
this project treats as a failure in itself.
