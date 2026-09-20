package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheRoleMatrixIsWhatEachRoleImplies(t *testing.T) {
	cases := map[Role]struct {
		server  bool
		player  bool
		service bool
	}{
		RoleServer:   {server: true, player: false, service: true},
		RolePlayer:   {server: false, player: true, service: false},
		RoleAllInOne: {server: true, player: true, service: true},
	}
	for role, want := range cases {
		if got := role.WantsServer(); got != want.server {
			t.Errorf("%s.WantsServer() = %v, want %v", role, got, want.server)
		}
		if got := role.WantsPlayer(); got != want.player {
			t.Errorf("%s.WantsPlayer() = %v, want %v", role, got, want.player)
		}
		// A player-only machine has nothing to autostart: the player is opened
		// by a person. Offering it a service would be offering to start a film
		// nobody asked for.
		if got := role.OffersService(); got != want.service {
			t.Errorf("%s.OffersService() = %v, want %v", role, got, want.service)
		}
	}
}

func TestTheDefaultRoleIsTheOneSomebodyInstallingWants(t *testing.T) {
	// Spec §14.3: somebody running the installer on their own computer wants to
	// watch a film, not administer a server.
	if DefaultRole != RoleAllInOne {
		t.Errorf("DefaultRole = %q, want %q", DefaultRole, RoleAllInOne)
	}
	if got, err := ParseRole(""); err != nil || got != DefaultRole {
		t.Errorf("ParseRole(\"\") = %q, %v; want the default", got, err)
	}
	// And the default is the first thing offered, because it is the answer most
	// people want.
	if Roles()[0] != DefaultRole {
		t.Errorf("the first role offered is %q, want the default", Roles()[0])
	}
}

func TestAnUnknownRoleIsRefusedRatherThanAssumed(t *testing.T) {
	if _, err := ParseRole("nas"); err == nil {
		t.Error("ParseRole accepted a role that does not exist")
	}
	if Role("nas").Valid() {
		t.Error("an invented role reported itself as valid")
	}
}

func TestValidateRefusesWhatWouldNotStart(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-film.mkv")
	if err := os.WriteFile(file, []byte("not really a film"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		mutate func(*Plan)
		want   string
	}{
		"a port that is not a port": {
			mutate: func(p *Plan) { p.Port = 70000 },
			want:   "port",
		},
		"a folder that does not exist": {
			mutate: func(p *Plan) { p.LibraryPaths = []string{filepath.Join(dir, "nowhere")} },
			want:   "does not exist",
		},
		"a file where a folder belongs": {
			mutate: func(p *Plan) { p.LibraryPaths = []string{file} },
			want:   "not a folder",
		},
		"an invented role": {
			mutate: func(p *Plan) { p.Role = Role("nas") },
			want:   "role",
		},
		"no hostname": {
			mutate: func(p *Plan) { p.Hostname = "" },
			want:   "hostname",
		},
	}

	for name, test := range cases {
		plan := Plan{
			Role:         RoleServer,
			DataDir:      filepath.Join(dir, "data"),
			LibraryPaths: []string{dir},
			Port:         8383,
			Hostname:     "theia",
		}
		test.mutate(&plan)
		err := plan.Validate()
		if err == nil {
			t.Errorf("%s: Validate accepted it", name)
			continue
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %q does not mention %q", name, err, test.want)
		}
	}
}

func TestValidateNormalisesTheDataDirectory(t *testing.T) {
	dir := t.TempDir()
	plan := Plan{
		Role:     RoleServer,
		DataDir:  filepath.Join(dir, ".", "data", "..", "data"),
		Port:     8383,
		Hostname: "theia",
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if strings.Contains(plan.DataDir, "..") {
		t.Errorf("the data directory came back unnormalised: %s", plan.DataDir)
	}
	if !filepath.IsAbs(plan.DataDir) {
		t.Errorf("the data directory came back relative: %s", plan.DataDir)
	}
}

func TestAFolderListIsReadTheSameWayFromAFormAndFromAFlag(t *testing.T) {
	// One per line from the text field, and the same set through splitPaths: the
	// form and the flags must mean the same thing, including the blank lines
	// somebody leaves behind and the trailing separator.
	blob := "D:\\Films\r\n\r\n  /srv/media  \n"
	fromForm := splitPaths(blob)
	want := []string{"D:\\Films", "/srv/media"}
	if len(fromForm) != len(want) {
		t.Fatalf("splitPaths = %q, want %q", fromForm, want)
	}
	for i := range want {
		if fromForm[i] != want[i] {
			t.Errorf("path %d = %q, want %q", i, fromForm[i], want[i])
		}
	}
}

func TestAddingAFolderDoesNotLoseTheOthers(t *testing.T) {
	existing := []string{`D:\Films`, `E:\Series`}
	merged := mergePaths(existing, []string{`E:\Series`, `F:\Docs`})
	if len(merged) != 3 {
		t.Fatalf("mergePaths = %q, want three folders", merged)
	}
	if merged[0] != `D:\Films` || merged[1] != `E:\Series` || merged[2] != `F:\Docs` {
		t.Errorf("mergePaths = %q, want the existing order first", merged)
	}
}

func TestAddingAFolderTwiceWithDifferentCaseIsOnceOnWindows(t *testing.T) {
	// A Windows path is case-insensitive, so D:\Films and d:\films are one
	// folder; a Linux path is not, and treating them as one would drop a real
	// library.
	merged := mergePaths([]string{`D:\Films`}, []string{`d:\films`})
	if isWindowsPath() {
		if len(merged) != 1 {
			t.Errorf("mergePaths kept %d folders on Windows, want 1", len(merged))
		}
		return
	}
	if len(merged) != 2 {
		t.Errorf("mergePaths collapsed %d folders elsewhere, want 2", len(merged))
	}
}

func TestBothLanguagesSayEverything(t *testing.T) {
	// A missing sentence would print as an empty line in whichever language is
	// missing it - which is how a half-translated interface ships.
	for key, frenchText := range french {
		englishText, ok := english[key]
		if !ok {
			t.Errorf("the key %q is French-only", key)
			continue
		}
		if strings.TrimSpace(englishText) == "" || strings.TrimSpace(frenchText) == "" {
			t.Errorf("the key %q is empty in one language", key)
		}
	}
	for key := range english {
		if _, ok := french[key]; !ok {
			t.Errorf("the key %q is English-only", key)
		}
	}
}

func TestTheLanguageFallsBackToEnglishAndNormalises(t *testing.T) {
	// An explicit choice wins, and every way of writing it lands on the same
	// catalogue: this is the one place that decides, so the interface, the
	// metadata and the installer cannot drift apart.
	for _, value := range []string{"en", "EN", "en-US", "en_GB", "English"} {
		if _, code := CatalogueFor(value); code != "en" {
			t.Errorf("CatalogueFor(%q) gave %q, want en", value, code)
		}
	}
	for _, value := range []string{"fr", "FR", "fr-FR", "fr_CA"} {
		if _, code := CatalogueFor(value); code != "fr" {
			t.Errorf("CatalogueFor(%q) gave %q, want fr", value, code)
		}
	}
	// English is the base of the product, so anything that is not one of the two
	// codes is answered in English rather than in a guess (decision 137). That
	// includes the name of a language written out: `--lang` takes a code, and a
	// value that only looks like one is a mistake worth landing on the base.
	for _, value := range []string{"de", "français", "french", "en anglais"} {
		if _, code := CatalogueFor(value); code != "en" {
			t.Errorf("CatalogueFor(%q) gave %q, want the English base", value, code)
		}
	}
	// With nothing said at all the machine's own locale is a courtesy; the
	// terminal in a test says nothing, so English answers.
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		if _, code := CatalogueFor(""); code != "en" {
			t.Errorf("with no locale set, the language was %q, want en", code)
		}
	}
}
