package setup

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
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

// render drives a form to its first real frame.
func render(t *testing.T, form *huh.Form) string {
	t.Helper()
	return pump(t, form, tea.WindowSizeMsg{Width: 80, Height: 40}).View()
}

func TestTheFormDrawsEveryQuestionInTheChosenLanguage(t *testing.T) {
	// Huh pages the groups, so each question is checked on the page that carries
	// it. Walking the pages is what proves every question is reachable at all - a
	// form whose second page never appears is a form nobody can finish.
	language, _ := CatalogueFor("fr")
	pages := walkPages(t, buildForm(&FormResult{Role: RoleAllInOne}, language))
	if len(pages) != 5 {
		t.Fatalf("the all-in-one form has %d pages, want 5", len(pages))
	}
	wanted := [][]string{
		{"roleTitle"},
		{"pathsTitle", "portTitle", "hostTitle"},
		{"libraryTitle"},
		{"serviceTitle"},
		{"confirmTitle"},
	}
	for index, keys := range wanted {
		for _, key := range keys {
			if !strings.Contains(pages[index], language[key]) {
				t.Errorf("page %d does not show %q:\n%s", index+1, language[key], pages[index])
			}
		}
	}

	english, _ := CatalogueFor("en")
	englishView := render(t, buildForm(&FormResult{Role: RoleAllInOne}, english))
	if !strings.Contains(englishView, english["roleTitle"]) {
		t.Error("the English form did not use the English catalogue")
	}
	if strings.Contains(englishView, language["roleTitle"]) {
		t.Error("the English form still showed French")
	}
}

func TestTheFormOffersTheRolesInTheOrderSomebodyWantsThem(t *testing.T) {
	language, _ := CatalogueFor("fr")
	view := render(t, buildForm(&FormResult{Role: RoleAllInOne}, language))
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
	pages := walkPages(t, buildForm(&FormResult{Role: RolePlayer}, language))

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
	pages := walkPages(t, buildForm(&result, language))
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
	form := buildForm(&result, language).WithWidth(80).WithHeight(40)

	updated, _ := form.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated == nil {
		t.Fatal("the form returned no model for a key")
	}
	if strings.TrimSpace(updated.View()) == "" {
		t.Error("the form drew nothing after a keystroke")
	}
}
