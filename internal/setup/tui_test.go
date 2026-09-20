package setup

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/Benitoow/theia-media/internal/config"
)

// The terminal interface is checked by driving the real form with the messages a
// keyboard produces, and looking at what it draws. A screenshot cannot be taken
// here, and "it compiled" says nothing about a form: the first version of this
// form threw the answer to its own confirmation question away and installed
// anyway, which no amount of reading the builder would have shown.
// walkPages renders every page of a form, in order, running the commands the
// form returns the way the Bubble Tea runtime does.
//
// Running them is not optional: Huh loads a DescriptionFunc through a command -
// it fetches the value after the frame rather than while drawing it - so the
// confirmation's summary is invisible to a test that only reads View(). That is
// exactly how the first version of this test "passed" while showing nothing.
func walkPages(t *testing.T, form *huh.Form) []string {
	t.Helper()
	model := pump(t, form, tea.WindowSizeMsg{Width: 80, Height: 40})
	pages := []string{model.View()}
	for range 12 { // a form longer than this is a form nobody finishes
		cmd := form.NextGroup()
		if cmd == nil {
			break
		}
		model = pump(t, form, cmd())
		pages = append(pages, model.View())
	}
	return pages
}

// pump feeds a message and then every command it produces, up to a bound. The
// bound matters: one of Huh's commands is a repeating tick, and a test that
// follows those forever never finishes.
func pump(t *testing.T, form *huh.Form, msg tea.Msg) tea.Model {
	t.Helper()
	model, cmd := form.Update(msg)
	for steps := 0; cmd != nil && steps < 30; steps++ {
		next := cmd()
		if commands := sequence(next); commands != nil {
			for _, inner := range commands {
				if inner == nil {
					continue
				}
				model, _ = form.Update(inner())
			}
			break
		}
		if batch, ok := next.(tea.BatchMsg); ok {
			for _, inner := range batch {
				if inner == nil {
					continue
				}
				model, _ = form.Update(inner())
			}
			break
		}
		if next == nil {
			break
		}
		model, cmd = form.Update(next)
	}
	return model
}

// sequence unwraps Bubble Tea's multi-command message, which is how Init hands
// back the work of focusing every field.
//
// The message type is unexported, so reflection is the only way in from outside
// the package. It matters that the commands run: the form focuses its first
// field during Init, and a test that skips that step is looking at a form whose
// footer - and whose focus - belong to no page at all.
func sequence(msg tea.Msg) []tea.Cmd {
	if msg == nil {
		return nil
	}
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice || value.Type().Name() != "sequenceMsg" {
		return nil
	}
	commands := make([]tea.Cmd, 0, value.Len())
	for index := 0; index < value.Len(); index++ {
		if cmd, ok := value.Index(index).Interface().(tea.Cmd); ok {
			commands = append(commands, cmd)
		}
	}
	return commands
}

// render drives a form to its first real frame.
func render(t *testing.T, form *huh.Form) string {
	t.Helper()
	return pump(t, form, tea.WindowSizeMsg{Width: 80, Height: 40}).View()
}

// flatten removes every space, tab and newline, so a sentence that has been
// wrapped by the renderer still compares equal to the sentence in the catalogue.
func flatten(text string) string {
	return strings.Join(strings.Fields(text), "")
}

func TestTheFormDrawsEveryQuestionInTheChosenLanguage(t *testing.T) {
	// Huh pages the groups, so each question is checked on the page that carries
	// it. Walking the pages is what proves every question is reachable at all - a
	// form whose second page never appears is a form nobody can finish.
	// The count matters too. It was five pages for the same questions because the
	// role was asked twice - once in a form of its own and again as the first
	// group of the second one - and that is the step which appeared to need enter
	// pressed twice. Five pages now, one question each, with the header riding
	// along with the first instead of being a page of its own.
	language, _ := CatalogueFor("fr")
	pages := walkPages(t, buildForm(&FormResult{Role: RoleAllInOne}, language, 76, 14))
	if len(pages) != 5 {
		t.Fatalf("the all-in-one form has %d pages, want 5:\n%s", len(pages), strings.Join(pages, "\n---\n"))
	}
	wanted := [][]string{
		{"brand", "intro", "roleTitle"},
		{"pathsTitle", "portTitle", "hostTitle"},
		{"libraryTitle"},
		{"serviceTitle"},
		{"confirmTitle"},
	}
	for index, keys := range wanted {
		for _, key := range keys {
			// Compared without whitespace: a sentence that wraps is still the
			// same sentence, and asserting on the raw string fails on a narrow
			// terminal rather than on a bug.
			if !strings.Contains(flatten(pages[index]), flatten(language[key])) {
				t.Errorf("page %d does not show %q:\n%s", index+1, language[key], pages[index])
			}
		}
	}
	// And the role is asked exactly once, which is what "the step repeats" meant.
	asked := 0
	for _, page := range pages {
		if strings.Contains(page, language["roleTitle"]) {
			asked++
		}
	}
	if asked != 1 {
		t.Errorf("the role question appears on %d pages, want 1", asked)
	}

	english, _ := CatalogueFor("en")
	englishView := render(t, buildForm(&FormResult{Role: RoleAllInOne}, english, 76, 14))
	if !strings.Contains(englishView, english["roleTitle"]) {
		t.Error("the English form did not use the English catalogue")
	}
	if strings.Contains(englishView, language["roleTitle"]) {
		t.Error("the English form still showed French")
	}
}

func TestTheFormOffersTheRolesInTheOrderSomebodyWantsThem(t *testing.T) {
	language, _ := CatalogueFor("fr")
	view := render(t, buildForm(&FormResult{Role: RoleAllInOne}, language, 76, 14))
	allInOne := strings.Index(view, language["roleAllInOne"])
	server := strings.Index(view, language["roleServer"])
	player := strings.Index(view, language["rolePlayer"])
	if allInOne < 0 || server < 0 || player < 0 {
		t.Fatalf("the form does not list all three roles:\n%s", view)
	}
	if !(allInOne < server && server < player) {
		t.Errorf("the roles are out of order: all-in-one=%d server=%d player=%d", allInOne, server, player)
	}
}

func TestAPlayerOnlyMachineIsNotAskedWhereItsDataGoes(t *testing.T) {
	language, _ := CatalogueFor("fr")
	pages := walkPages(t, buildForm(&FormResult{Role: RolePlayer}, language, 76, 14))

	for _, page := range pages {
		if strings.Contains(page, language["pathsTitle"]) {
			t.Error("a player-only machine was asked where to keep a database it will not have")
		}
		if strings.Contains(page, language["libraryTitle"]) {
			t.Error("a player-only machine was asked which folders to scan")
		}
		if strings.Contains(page, language["serviceTitle"]) {
			t.Error("a player-only machine was offered an autostart entry for a server it does not run")
		}
	}
	// It is still asked to confirm, and the page that asks knows what it is.
	last := pages[len(pages)-1]
	if !strings.Contains(last, language["confirmTitle"]) {
		t.Errorf("a player-only machine never got to confirm anything:\n%s", last)
	}
	if !strings.Contains(last, roleLabel(RolePlayer, language)) {
		t.Errorf("the confirmation lost the answer that produced it:\n%s", last)
	}
}

func TestTheConfirmationCarriesTheAnswersSomebodyJustGave(t *testing.T) {
	language, _ := CatalogueFor("fr")
	result := FormResult{
		Role:         RoleServer,
		DataDir:      `D:\Theia`,
		Port:         "9000",
		Hostname:     "salon",
		LibraryBlob:  "D:\\Films",
		InstallServi: true,
	}
	pages := walkPages(t, buildForm(&result, language, 76, 14))
	confirmation := pages[len(pages)-1]

	// The summary is a function precisely so it reads the live values: built from
	// the defaults it would describe an installation nobody asked for. And it has
	// to be *drawn*: Huh loads it through a command, so a form that never runs
	// its own commands shows an empty space where the summary belongs.
	for _, want := range []string{`D:\Theia`, "9000", "salon", `D:\Films`} {
		if !strings.Contains(confirmation, want) {
			t.Errorf("the confirmation does not mention %q:\n%s", want, confirmation)
		}
	}
}

func TestTheFormParsesWhatSomebodyTypesIntoAPlan(t *testing.T) {
	// The port arrives as text because that is what a keyboard produces, and the
	// folders as one blob because a text area is what the question deserves.
	result := FormResult{
		Role:         RoleAllInOne,
		DataDir:      "/srv/theia",
		Port:         " 8399 ",
		Hostname:     " salon ",
		LibraryBlob:  "/srv/films\n\n/srv/series\n",
		InstallServi: true,
	}
	plan, err := result.Parse("/default")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if plan.Port != 8399 {
		t.Errorf("port = %d, want 8399", plan.Port)
	}
	if plan.Hostname != "salon" {
		t.Errorf("hostname = %q, want it trimmed", plan.Hostname)
	}
	if len(plan.LibraryPaths) != 2 {
		t.Errorf("library paths = %q, want two", plan.LibraryPaths)
	}
	if !plan.Service {
		t.Error("the autostart answer was lost")
	}
	if plan.DataDir != "/srv/theia" {
		t.Errorf("data directory = %q", plan.DataDir)
	}

	// And a port that is not a number is refused where it was typed, rather than
	// becoming a zero port in a file.
	if _, err := (FormResult{Role: RoleServer, Port: "huit mille"}).Parse("/default"); err == nil {
		t.Error("Parse accepted a port that is not a number")
	}
}

func TestTheFormAnswersTheConfirmQuestionForReal(t *testing.T) {
	// This is the fault the first version shipped with: the confirmation's
	// answer went into a throwaway bool, so "no" installed anyway. The field is
	// now part of the result and the caller checks it.
	result := FormResult{Role: RoleServer, DataDir: t.TempDir(), Port: strconv.Itoa(freePort(t)), Hostname: "theia", Confirmed: false}
	if result.Confirmed {
		t.Fatal("a form nobody answered reported itself as confirmed")
	}
	result.Confirmed = true
	if !result.Confirmed {
		t.Fatal("the confirmation did not stick")
	}
}

func TestTheFormKeepsRunningWhenAKeyArrives(t *testing.T) {
	// A form that panics on a keystroke is a form nobody can use, and the model
	// is the only part of this testable without a terminal.
	language, _ := CatalogueFor("fr")
	result := FormResult{Role: DefaultRole}
	form := buildForm(&result, language, 76, 14)

	updated, _ := form.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated == nil {
		t.Fatal("the form returned no model for a key")
	}
	if strings.TrimSpace(updated.View()) == "" {
		t.Error("the form drew nothing after a keystroke")
	}
}

func TestTheFormStartsWithWhatTheCommandLineSaid(t *testing.T) {
	// A flag the form then ignores is worse than a flag that does not exist.
	// `--data-dir` was accepted, the form asked the question anyway, and the
	// confirmation proposed %APPDATA% - which is how this was found, by reading
	// the summary page on a screen.
	seeded, err := formDefaults(FormOptions{
		DataDir:  `D:\Theia`,
		Port:     9000,
		Hostname: " salon ",
		Library:  []string{`D:\Films`, `D:\Series`},
		Service:  true,
	})
	if err != nil {
		t.Fatalf("formDefaults: %v", err)
	}
	if seeded.DataDir != `D:\Theia` {
		t.Errorf("data dir = %q, want the one from the command line", seeded.DataDir)
	}
	if seeded.Port != "9000" {
		t.Errorf("port = %q, want 9000", seeded.Port)
	}
	if seeded.Hostname != "salon" {
		t.Errorf("hostname = %q, want it trimmed", seeded.Hostname)
	}
	if seeded.LibraryBlob != "D:\\Films\nD:\\Series" {
		t.Errorf("library blob = %q", seeded.LibraryBlob)
	}
	if !seeded.InstallServi {
		t.Error("the autostart answer from the command line was dropped")
	}
	if !seeded.Confirmed {
		t.Error("the confirmation does not start on install, so a stray enter cancels silently")
	}

	// With nothing on the command line, the machine's own defaults: the port is
	// the product's, not zero, and the form proposes a directory rather than an
	// empty box.
	plain, err := formDefaults(FormOptions{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("formDefaults: %v", err)
	}
	if plain.Port != "8383" {
		t.Errorf("port = %q, want the product's default 8383", plain.Port)
	}
	if plain.Hostname == "" || plain.DataDir == "" {
		t.Errorf("the form proposes an empty answer: %+v", plain)
	}
	if len(plain.LibraryBlob) != 0 {
		t.Errorf("a machine with no --library starts with %q", plain.LibraryBlob)
	}
}

func TestTheInstallerAsksForTheLanguageFirst(t *testing.T) {
	// The maintainer's instruction of 20 September 2026: the favourite language
	// is asked explicitly by the setup launcher. It is a page of its own, before
	// the form, because a form's labels are fixed when it is built and the rest
	// of the installer has to be drawn in the language this page answers.
	chosen := config.LanguageEnglish
	form := languageForm(&chosen, maxFormWidth, 14)
	pump(t, form, form.Init()())

	focused := form.GetFocusedField()
	if _, ok := focused.(*huh.Select[string]); !ok {
		t.Fatalf("the first question is a %T, want the language question", focused)
	}

	// English is offered first and French second; moving down once and confirming
	// is how somebody picks the second, and it has to reach the answer the rest
	// of the installer reads.
	pump(t, form, tea.KeyMsg{Type: tea.KeyDown})
	pump(t, form, tea.KeyMsg{Type: tea.KeyEnter})
	if chosen != config.LanguageFrench {
		t.Errorf("choosing the second answer gave %q, want fr", chosen)
	}
}

func TestTheHintSaysWhatThePageAccepts(t *testing.T) {
	// The line under the form was Huh's, built from the focused field's
	// bindings, and on a real screen it read
	// "↑ monter • ↓ descendre • / filtrer • ↓ descendre" - one word twice,
	// half of it in the wrong language, and never a word about leaving.
	//
	// Two things are asserted here: every kind of page has a sentence, and no
	// sentence repeats a word, which is the fault that was on the screen.
	for _, code := range []string{"fr", "en"} {
		language, _ := CatalogueFor(code)
		text := func(key string) string { return language[key] }
		hints := map[string]string{
			"select":  helpLine(&huh.Select[Role]{}, text),
			"input":   helpLine(&huh.Input{}, text),
			"text":    helpLine(&huh.Text{}, text),
			"confirm": helpLine(&huh.Confirm{}, text),
		}
		exit := map[string]string{"fr": "échap", "en": "esc"}[code]
		for kind, hint := range hints {
			if strings.TrimSpace(hint) == "" {
				t.Errorf("%s: the %s page has no hint", code, kind)
				continue
			}
			if !strings.Contains(hint, exit) {
				t.Errorf("%s: the %s hint does not say how to leave: %q", code, kind, hint)
			}
			seen := map[string]bool{}
			for _, word := range strings.Fields(hint) {
				if word == "·" {
					continue
				}
				if seen[word] {
					t.Errorf("%s: the %s hint repeats %q: %q", code, kind, word, hint)
				}
				seen[word] = true
			}
		}
	}
	// A field that is none of these - Huh's note, for one - gets no hint rather
	// than somebody else's.
	french, _ := CatalogueFor("fr")
	if hint := helpLine(&huh.Note{}, func(key string) string { return french[key] }); hint != "" {
		t.Errorf("a note page was given the hint %q", hint)
	}
}

func TestTheFooterIsDrawnAndHuhsEnglishHelpIsNot(t *testing.T) {
	// Huh's help is hidden rather than translated: its strings are English and
	// come from its own keymap, so a French screen has to draw its own line.
	language, _ := CatalogueFor("fr")
	model := &formModel{
		form: buildForm(&FormResult{Role: RoleAllInOne}, language, 76, 14),
		text: func(key string) string { return language[key] },
	}
	// Init is what focuses the first question. Without it the focused field is
	// still the header note, and the footer would be measured on a page nobody
	// ever sees.
	pump(t, model.form, model.Init()())

	view := model.View()
	if !strings.Contains(flatten(view), flatten(language["helpSelect"])) {
		t.Errorf("the form did not draw its hint:\n%s", view)
	}
	for _, english := range []string{"submit", "toggle", "filter", "new line"} {
		if strings.Contains(view, english) {
			t.Errorf("Huh's own English help is still on the screen (%q):\n%s", english, view)
		}
	}
}

func TestEscapeCancelsAndTheEndOfTheFormStopsTheProgram(t *testing.T) {
	// Huh binds ctrl+c and nothing else, so the key everybody presses to leave a
	// form did nothing - while the old screen printed "esc quitter" under the
	// question. And because this installer runs the Bubble Tea program itself
	// rather than letting Huh run it, the end of the form has to stop the
	// program: otherwise the last answer would be given and nothing would ever
	// happen.
	language, _ := CatalogueFor("fr")

	escape := &formModel{form: buildForm(&FormResult{Role: DefaultRole}, language, 76, 14), text: func(key string) string { return language[key] }}
	pump(t, escape.form, escape.Init()())
	_, cmd := escape.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !escape.aborted {
		t.Error("escape did not cancel the form")
	}
	if cmd == nil {
		t.Fatal("escape left the program running")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("escape did not stop the program")
	}

	var yes bool
	confirm := &formModel{
		form: huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Installer ?").Affirmative("Oui").Negative("Non").Value(&yes),
		)),
		text: func(key string) string { return language[key] },
	}
	confirm.form.SubmitCmd = tea.Quit
	confirm.form.CancelCmd = tea.Quit
	pump(t, confirm.form, confirm.Init()())

	// "y" answers the question and produces the message that finishes the form,
	// which pump follows the way the runtime does.
	pump(t, confirm.form, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if !yes {
		t.Error("the answer never reached the field")
	}
	if confirm.form.State != huh.StateCompleted {
		t.Fatalf("the last question left the form in state %v, not completed", confirm.form.State)
	}
	// And the wrapper stops the program there. It has to: Huh only issues its
	// quitting command when it runs the program itself, and here it does not -
	// so a form that ends without a quit is an installer that hangs after the
	// last answer.
	_, cmd = confirm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("the end of the form left the program running")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("the end of the form did not ask the program to stop")
	}
	if confirm.aborted {
		t.Error("answering the last question was recorded as a cancellation")
	}
}
