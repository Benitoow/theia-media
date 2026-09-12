# Real-media validation - a 4K HDR remux, end to end

11 September 2026. Every playback test until now ran on generated media
(`testsrc2`). This campaign ran the full product path on one real file, chosen
to be the hardest case in the library profile: a untouched UHD Blu-ray remux.

## The specimen

`Star.Wars.Episode.III.Revenge.Of.The.Sith.2005.PROPER.2160p.BluRay.REMUX.
HEVC.DTS-HD.MA.TrueHD.7.1.Atmos-FGT.mkv` - 53.79 GiB, 140 minutes.

| Property | Value |
|---|---|
| Video | HEVC Main 10, 3840×2160, 23.98 fps, **HDR10** (bt2020/smpte2084) |
| Total bitrate | **54.95 Mb/s** |
| Audio | **TrueHD Atmos 7.1** (default), DTS-HD MA 7.1, AC3 5.1 ×3, AC3 2.0, EAC3 7.1 - 7 tracks |
| Subtitles | **4 × PGS** (eng/fre/spa/jpn) + 2 × SRT |
| GOP | **0.95 s average, 1.0 s max** (measured over the first 180 s) |

The 1-second GOP settles the old buffering question for this class of file:
fragmented output emits at ~1 Hz, regular, from the source's own keyframes. A
"stutter every two seconds" cannot come from fragment cadence on a UHD remux.

## Production-path throughput on real content

Pinned runtime (b6.1.1, SHA verified), `h264_amf`, software HEVC decode - the
runtime's own picks. Bounded source slices, production args, `AudioTranscode`
as the decision flow sets it.

| Path | Synthetic (testsrc2, tranche 6) | **Real content** |
|---|---:|---:|
| Remux (video copy, TrueHD→AAC 192k stereo) | - | **13.61× real time** |
| Tone-mapped transcode at 1080p (the escalation rung) | 2.98× | **1.46× real time** |
| Tone-mapped transcode at source size (2160p) | 1.17× | **0.92× real time** |

These numbers preserve the original FFmpeg 6.1.1 campaign. The qualified
8.1.2-4 follow-up, its controlled A/B comparison and the Microsoft Edge
correction are recorded separately in
[`ffmpeg-8.1.2-validation.md`](ffmpeg-8.1.2-validation.md).

Two conclusions:

1. **The source-size HDR transcode is dead on real content.** Synthetic
   measured 1.17× - barely sustainable; the real file measures **0.92×** - the
   stream never arrives as fast as it is read, and a stall is guaranteed. The
   escalation's 1080p rung (`initialCompatibilityHeight`) is not a nicety; it
   is what makes HDR playback work at all.
2. **Even the 1080p rung is narrower than the synthetic numbers suggested**
   (1.46× vs 2.98×). It holds one session comfortably; the limiter's rule that
   a tone map consumes the whole budget (decision 87) is confirmed and now has
   real-content numbers. Two simultaneous HDR viewers are refused, as designed.

## Finding 1 - the first playback of a risky file fails on a fresh install

Reproduced twice with a clean data directory (no FFmpeg, no inspection, empty
browser profile), Chromium without proprietary codecs, the real file above.

The chain, from the server log of the first attempt:

```
T+0.00s  GET /api/stream/1/files/1/info
         → media_status=pending, transcode.available=false
           (the M1 promise holds: /info never waits on a download)
T+0.06s  "downloading ffmpeg" - kicked off by the info path, asynchronously
T+1.53s  "ffmpeg installed" (b6.1.1, SHA 04e130… - the pinned build)
T+3.24s  remux stream starts (mode=remux, audio_action=transcode - correct)
T+3.44s  stream killed at 203 ms / 35 KB: the browser aborted.
         The element reported videoWidth=0 (Chromium cannot decode HEVC),
         the player asked to escalate - but its info snapshot, taken at T+0,
         said transcode.available=false - so it hard-failed
         browser_cannot_decode_video instead of escalating.
```

By the time the player made its decision, FFmpeg had been installed for two
seconds and a fresh `/info` would have answered `available: true`. The second
attempt works immediately - which means **every new user's first risky-codec
film fails once** before it plays. With the Reddit influx, that first film is
often the first impression.

### Implemented fix

Player-side, smallest surface: when `onLoadedMetadata` proves
`videoWidth === 0` and `info.transcode.available` is false, **re-fetch the
info once** before concluding. The world moved while the remux was attempted:
FFmpeg is downloaded during the attempt itself, so the re-fetch sees the
truth. No contract change (`/info` is idempotent), no server change, one
guarded retry. A Playwright regression now forces exactly that stale first
answer, observes the second `/info`, the `video=transcode` request and decoded
frames.

Server-side alternative considered and rejected: making `/info` wait on the
download or the capabilities probe would break the M1 promise in spirit -
interrogate must stay fast and must never depend on a download.

## Validated end to end (second attempt, fresh profile)

| Step | Result |
|---|---|
| Plan | `playback_plan` mode=remux, reason_code=audio_transcode, risky=true, tone_map=true |
| Escalation | `quality_adapted` at T+0.2 s, reason_code=browser_cannot_decode_video, automatic |
| Picture | ~10 s after play clicked (remux attempt + escalation + transcode startup + 6 s buffer) |
| Delivery | MSE blob source; frames advance; no console errors |
| Deep seek | `currentTime = 3600` on a 140-min file: picture returns, playback resumes |
| Pause / resume | Clock advances after resume |
| Shutdown | 1 ffmpeg during playback → **0 after the page closes** (no orphans) |
| Diagnostics | All events captured server-side (decisions 98–102 hold on real media) |

## What this closes

- The only remaining untested claim of the refonte: playback on real library
  content, real bitrate, real HDR, real TrueHD, real PGS tracks.
- The buffering question (fragment cadence) for the UHD-remux class.
- Decision 87's tone-map budget rule, now with real-content numbers.

## What it opens

- Finding 1 is closed by the guarded `/info` refresh above. The follow-up also
  found and fixed Edge's independent byte-quota saturation; see
  [`ffmpeg-8.1.2-validation.md`](ffmpeg-8.1.2-validation.md).
- The ~10 s time-to-picture on a first risky playback is dominated by the
  remux attempt that cannot succeed (the browser has already proved it cannot
  decode this codec class on the previous film) and by transcode startup. The
  remembered-verdict path (decision 98) removes the remux attempt from the
  second film onward; the first film still pays it. A server-side hint on the
  plan (once the browser's verdict is known) could shorten it further -
  noted, not decided.
