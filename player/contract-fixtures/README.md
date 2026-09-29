# Player bridge contract fixtures

- `series-detail.json`: emitted by `TestSeriesCloseoutContract` from a scanned
  SQLite library, with stable timestamps. Carries TMDB name/language, interrupted
  progress, next unwatched selection, combined metadata and playable file fields.
- `home-empty.json`: emitted by `TestEmptyHomeContractFixture` from an empty store.
- `home-null-released.json`: the empty response sent by the 3.3.1 server, retained
  for independently updated server/player installations.

Go compares the current store output with these files; Rust deserialises the same
files and asserts the fields needed for playback. Regenerate only after reviewing
an intended contract change:

```powershell
$env:THEIA_WRITE_CONTRACT='1'
go test ./internal/library -run 'TestSeriesCloseoutContract|TestEmptyHomeContractFixture'
Remove-Item Env:THEIA_WRITE_CONTRACT
cargo test --manifest-path player/Cargo.toml
```

These are disposable-library payloads, not a record of the maintainer's media.
