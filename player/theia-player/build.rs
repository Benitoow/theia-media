fn main() {
    // A bug report has to be able to name the build it came from. The release
    // workflow knows the tag and passes it as `-Version` to build-player.ps1,
    // which sets THEIA_VERSION for this script; a build without it is a local
    // one and says so, the same answer the server gives. The var is declared as
    // watched so that changing it actually rebuilds the crate instead of
    // leaving the previous version compiled in.
    println!("cargo:rerun-if-env-changed=THEIA_VERSION");
    let version = std::env::var("THEIA_VERSION").unwrap_or_else(|_| "dev".to_string());
    println!("cargo:rustc-env=THEIA_VERSION={version}");

    // The OSD is embedded by `tauri::generate_context!` at compile time, from
    // `../ui/dist`, and cargo watches this crate's own sources - not that
    // directory. So an OSD-only change compiled a binary that still carried the
    // previous interface: measured on 20 September 2026, `player/ui/dist` named
    // `index-BC4gdl_y.js` and carried `media-preview-summary`, while the built
    // executable carried the pair of asset names from the build before it. The
    // maintainer was looking at a preview two revisions old and reported a zoom
    // that had been deleted.
    //
    // Declaring the directory here makes the crate rebuild with the interface it
    // ships. `build-player.ps1` then checks the result rather than trusting it.
    println!("cargo:rerun-if-changed=../ui/dist");
    tauri_build::build()
}
