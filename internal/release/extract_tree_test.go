package release

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A macOS player is published as an application *tree*, not as a list of files:
// the zip's members are Theia.app/... and the installation is that directory. So
// a requested name that is a directory means "everything under it" - and the
// flat behaviour the Windows bundle depends on has to keep working exactly as it
// did, which is the first thing these tests pin.

const (
	tree         = "Theia.app"
	treeExe      = tree + "/Contents/MacOS/theia-player"
	treeEngine   = tree + "/Contents/Frameworks/libmpv.dylib"
	treeLicence  = tree + "/Contents/Resources/LICENSE-libmpv.txt"
	treeNotice   = tree + "/Contents/Resources/NOTICE.md"
	treeBackup   = "Theia.app.backup/old"
	treeStranger = "START-HERE.txt"
)

// playerTree is what the macOS bundle holds, plus the two members an extraction
// must never take: a directory whose name merely *begins* like the tree, and the
// note an archive ships to the person who downloaded it.
func playerTree() map[string]string {
	return map[string]string{
		tree + "/":   "",
		treeExe:      "player",
		treeEngine:   "engine",
		treeLicence:  "lgpl",
		treeNotice:   "notice",
		treeBackup:   "not ours",
		treeStranger: "read me first",
	}
}

// TestExtractStillTakesOneFileAtATime is the old behaviour, spelled out: a name
// that is a file is one file, and nothing beside it comes along. It is here
// because the directory support above it is the kind of change that quietly
// widens a rule - asking for the Windows player's four members must still take
// four files and not the tree of whatever else the archive holds.
func TestExtractStillTakesOneFileAtATime(t *testing.T) {
	archive := makeArchive(t, playerTree())
	dir := t.TempDir()

	written, err := Extract(archive, dir, []string{treeExe, treeEngine})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(written) != 2 {
		t.Errorf("extracted %v, want the two files asked for", written)
	}
	if body := string(mustReadFile(t, filepath.Join(dir, treeExe))); body != "player" {
		t.Errorf("%s holds %q", treeExe, body)
	}
	if _, err := os.Stat(filepath.Join(dir, treeLicence)); err == nil {
		t.Error("a file nobody asked for was extracted")
	}
}

func TestExtractTakesAWholeTreeWhenATreeIsAskedFor(t *testing.T) {
	archive := makeArchive(t, playerTree())
	dir := t.TempDir()

	written, err := Extract(archive, dir, []string{tree})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(written) != 4 {
		t.Errorf("extracted %v, want the four members of the tree", written)
	}
	// The tree is a directory, not a zero-byte file standing in its place: a
	// directory entry that was written as a file would have made every file
	// below it impossible to create.
	info, err := os.Stat(filepath.Join(dir, tree))
	if err != nil {
		t.Fatalf("the tree was not extracted: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("the tree was extracted as a file")
	}
	for _, member := range []string{treeExe, treeEngine, treeLicence, treeNotice} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(member))); err != nil {
			t.Errorf("%s was not extracted: %v", member, err)
		}
	}
	// And nothing that only looks like it belongs.
	if _, err := os.Stat(filepath.Join(dir, "Theia.app.backup")); err == nil {
		t.Error("a directory whose name merely begins like the tree was taken")
	}
	if _, err := os.Stat(filepath.Join(dir, treeStranger)); err == nil {
		t.Error("a file beside the tree was taken")
	}
}

func TestExtractRefusesATreeMissingARequiredMember(t *testing.T) {
	// The tree being there is not the same as the bundle being complete: a player
	// without its engine looks installed and does not start, so the members the
	// caller asked for are required by name, inside the tree or not.
	members := playerTree()
	delete(members, treeEngine)
	archive := makeArchive(t, members)

	_, err := Extract(archive, t.TempDir(), []string{tree, treeExe, treeEngine, treeLicence, treeNotice})
	if err == nil {
		t.Fatal("a bundle missing its engine was accepted")
	}
	if !strings.Contains(err.Error(), treeEngine) {
		t.Errorf("the refusal does not name the missing member: %v", err)
	}
}

func TestExtractRefusesATreeThatIsNotThere(t *testing.T) {
	// The other half of the same contract: the tree itself has to be in the
	// archive, so an archive of the Windows bundle is not mistaken for a macOS
	// one.
	archive := makeArchive(t, map[string]string{"theia-player.exe": "player"})
	_, err := Extract(archive, t.TempDir(), []string{tree})
	if err == nil {
		t.Fatal("an archive with no application tree was accepted")
	}
	if !strings.Contains(err.Error(), tree) {
		t.Errorf("the refusal does not name what was missing: %v", err)
	}
}

func TestExtractRefusesAnEntryClimbingOutOfTheTree(t *testing.T) {
	// The zip-slip guard has to cover every entry a tree brings with it, not only
	// the names that were asked for: a tree request is the one that opens the
	// whole archive up.
	members := playerTree()
	members[tree+"/../../escaped.txt"] = "nope"
	archive := makeArchive(t, members)
	dir := t.TempDir()
	inside := filepath.Join(dir, "inside")

	_, err := Extract(archive, inside, []string{tree})
	if err == nil {
		t.Fatal("an entry climbing out of the destination was extracted")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
		t.Error("the file was written outside the destination after all")
	}
}

func TestExtractPreservesFrameworkLink(t *testing.T) {
	// Windows may not grant an ordinary test process the right to create a
	// symlink. The release gate also runs this test on macOS, where the player
	// bundle actually needs the link to load libmpv.
	requireSymlinks(t)
	const member = tree + "/Contents/Frameworks/libmpv.2.dylib"
	archive := makeFrameworkArchive(t, "libmpv.2.dylib")
	dir := t.TempDir()
	for range 2 { // a force reinstall must replace the link it wrote before
		if _, err := Extract(archive, dir, []string{tree}); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		link := filepath.Join(dir, filepath.FromSlash(treeEngine))
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink: %v", link, err)
		}
		if target, err := os.Readlink(link); err != nil || target != "libmpv.2.dylib" {
			t.Errorf("link target = %q (%v)", target, err)
		}
		if body, err := os.ReadFile(link); err != nil || string(body) != "engine" {
			t.Errorf("the linked engine is unreadable: %q (%v)", body, err)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(member))); err != nil {
			t.Errorf("the link target is missing: %v", err)
		}
	}
}

func TestExtractRefusesEscapingFrameworkLink(t *testing.T) {
	archive := makeFrameworkArchive(t, "../../../../outside")
	_, err := Extract(archive, t.TempDir(), []string{tree})
	if err == nil || !strings.Contains(err.Error(), "escapes the destination") {
		t.Fatalf("an escaping link was accepted: %v", err)
	}
}

func TestExtractRefusesAnExistingSymlinkParent(t *testing.T) {
	requireSymlinks(t)
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, tree)); err != nil {
		t.Fatal(err)
	}
	_, err := Extract(makeArchive(t, playerTree()), dir, []string{tree})
	if err == nil || !strings.Contains(err.Error(), "not a real directory") {
		t.Fatalf("extraction followed a pre-existing symlink: %v", err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("extraction wrote through the link: %v (%v)", entries, err)
	}
}

func requireSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		probe := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink("target", probe); err != nil {
			t.Skipf("this Windows account cannot create symlinks: %v", err)
		}
	}
}

func makeFrameworkArchive(t *testing.T, linkTarget string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "player.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, member := range []struct{ name, body string }{
		{tree + "/Contents/Frameworks/libmpv.2.dylib", "engine"},
		{treeEngine, linkTarget},
	} {
		header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
		if member.name == treeEngine {
			header.SetMode(os.ModeSymlink | 0o755)
		} else {
			header.SetMode(0o755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(member.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archive
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
