package setup

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// The terminal interface: a short form, then the changes.
//
// Three things are deliberate here.
//
// It is a form rather than a sequence of prompts, so somebody can go back and
// change an answer, and so the whole installation is visible before any of it
// happens. Nothing is written until the last confirmation, which is the only
// promise an installer really has to keep.
//
// The colours come from the design system - the accent is the same gold as the
// rest of the product. A terminal cannot wear the interface's type or spacing,
// but it can avoid looking like a different product.
//
// Every sentence comes from the catalogue, including the ones inside the form.
// This is the first screen anybody sees of Theia, and shipping it in English
// only would be the same mistake the settings page made once (decision 25).

// accent is --color-accent from web/src/lib/tokens.css. Kept as a literal
// because a terminal cannot read a CSS custom property, and named here so the
// next person knows where it came from.
const (
	accent   = lipgloss.Color("#c8a24a")
	bone     = lipgloss.Color("#ede7dc")
	parchmnt = lipgloss.Color("#d6cfc2")
	muted    = lipgloss.Color("#8c857a")
)

// FormResult is what the form collected, before it becomes a Plan. Kept apart
// from Plan because the folders arrive as one blob of text and a port as a
// string: parsing is where mistakes happen, and it is tested on its own.
type FormResult struct {
	Role         Role
	DataDir      string
	Port         string
	Hostname     string
	LibraryBlob  string
	InstallServi bool

	// Confirmed is the answer to the last question. It is a field rather than a
	// throwaway `new(bool)` because a form that runs to completion and answers
	// "no" is not an error - and the first version of this discarded the answer
	// and installed anyway.
	Confirmed bool
}

// ParseAPI turns what a form (or a person) typed into a Plan.
//
// Separated from the terminal so it can be tested without one, and used by the
// flags as well: a folder list typed into a form and one passed as a flag must
// mean the same thing, including the empty lines somebody leaves behind.
func (r FormResult) Parse(defaultDataDir string) (Plan, error) {
	plan := Plan{
		Role:         r.Role,
		DataDir:      strings.TrimSpace(r.DataDir),
		Hostname:     strings.TrimSpace(r.Hostname),
		LibraryPaths: splitPaths(r.LibraryBlob),
		Service:      r.InstallServi,
	}
	port := strings.TrimSpace(r.Port)
	if port != "" {
		number, err := strconv.Atoi(port)
		if err != nil {
			return Plan{}, fmt.Errorf("port %q is not a number", port)
		}
		plan.Port = number
	}
	return plan.WithDefaults(defaultDataDir), nil
}

// splitPaths reads one folder per line, dropping blanks and keeping the order.
// Duplicates are left to mergePaths, which is where they matter.
func splitPaths(blob string) []string {
	paths := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(blob, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		paths = append(paths, trimmed)
	}
	return paths
}

// theme is Charm's own, with the product's palette on the parts a person looks
// at: the focused title and the selected option carry the accent, the help text
// stays muted.
func theme() *huh.Theme {
	t := huh.ThemeCharm()
	t.Focused.Title = t.Focused.Title.Foreground(accent).Bold(true)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(accent)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(accent)
	t.Focused.Description = t.Focused.Description.Foreground(parchmnt)
	t.Blurred.Title = t.Blurred.Title.Foreground(muted)
	t.Help.Ellipsis = t.Help.Ellipsis.Foreground(muted)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(muted)
	t.Help.FullDesc = t.Help.FullDesc.Foreground(muted)
	return t
}

// buildForm assembles the form. It is separate from Run so a test can drive it
// with key messages and assert what it draws, which is the only way to check a
// terminal interface without a terminal.
//
// The confirmation's description is a function, not a string: Huh evaluates a
// plain description when the form is built, so the summary would show the
// defaults rather than the answers somebody just typed.
func buildForm(result *FormResult, language Catalogue) *huh.Form {
	role := huh.NewSelect[Role]().
		Title(language["roleTitle"]).
		Description(language["roleDescription"]).
		Options(
			huh.NewOption(language["roleAllInOne"], RoleAllInOne),
			huh.NewOption(language["roleServer"], RoleServer),
			huh.NewOption(language["rolePlayer"], RolePlayer),
		).
		Value(&result.Role)

	where := huh.NewInput().
		Title(language["pathsTitle"]).
		Description(language["pathsDescription"]).
		Value(&result.DataDir).
		Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s", language["pathsTitle"])
			}
			return nil
		})

	port := huh.NewInput().
		Title(language["portTitle"]).
		Description(language["portDescription"]).
		Value(&result.Port).
		Validate(func(value string) error {
			number, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || number < 1 || number > 65535 {
				return fmt.Errorf("%s", language["portDescription"])
			}
			return nil
		})

	host := huh.NewInput().
		Title(language["hostTitle"]).
		Description(language["hostDescription"]).
		Value(&result.Hostname).
		Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s", language["hostDescription"])
			}
			return nil
		})

	folders := huh.NewText().
		Title(language["libraryTitle"]).
		Description(language["libraryHint"]).
		Lines(5).
		Value(&result.LibraryBlob)

	service := huh.NewConfirm().
		Title(language["serviceTitle"]).
		Description(language["serviceDesc"]).
		Affirmative(language["statusYes"]).
		Negative(language["statusNo"]).
		Value(&result.InstallServi)

	confirm := huh.NewConfirm().
		Title(language["confirmTitle"]).
		DescriptionFunc(func() string {
			plan, err := result.Parse("")
			if err != nil {
				return ""
			}
			return summary(plan, language)
		}, result).
		Affirmative(language["confirmYes"]).
		Negative(language["confirmNo"]).
		Value(&result.Confirmed)

	// A role that serves is asked where its data goes, what port it listens on
	// and which folders to watch; a player-only machine is asked none of that,
	// because it reads somebody else's library.
	groups := []*huh.Group{huh.NewGroup(role)}
	if result.Role.WantsServer() {
		groups = append(groups,
			huh.NewGroup(where, port, host),
			huh.NewGroup(folders),
			huh.NewGroup(service),
		)
	}
	groups = append(groups, huh.NewGroup(confirm))

	return huh.NewForm(groups...).
		WithTheme(theme()).
		WithShowHelp(true)
}

// summary is the sentence under the last question: what is about to happen, in
// the same words the result will use.
//
// It leads with the role, because that is the decision everything else follows
// from - and because a player-only machine has no port, no folders and no
// business writing a server's configuration, so without this line its
// confirmation would say almost nothing.
func summary(plan Plan, language Catalogue) string {
	lines := []string{fmt.Sprintf("%s : %s", language["statusRole"], roleLabel(plan.Role, language))}
	if plan.Role.WantsServer() {
		lines = append(lines,
			fmt.Sprintf("%s : %s", language["statusData"], plan.DataDir),
			fmt.Sprintf("%s : %d", language["statusPort"], plan.Port),
			fmt.Sprintf("%s : %s", language["statusHost"], plan.Hostname),
		)
		if len(plan.LibraryPaths) > 0 {
			lines = append(lines, fmt.Sprintf("%s : %s", language["statusLibrary"], strings.Join(plan.LibraryPaths, ", ")))
		}
	}
	return strings.Join(lines, "\n")
}

// FormOptions is how a caller supplies a terminal. Tests pass buffers.
type FormOptions struct {
	Input    io.Reader
	Output   io.Writer
	Language string
	// DataDir is the default the form proposes, already resolved by the caller.
	DataDir string
}

// RunInteractive shows the installer and applies what it collects.
//
// The role is asked first, on its own, and the rest of the form is built from
// the answer: a player-only machine is asked where its data goes by nobody,
// because it has none. The second form still shows everything before anything is
// written, and the last question is the only thing that starts the work.
func RunInteractive(opts FormOptions) (Result, error) {
	language, _ := CatalogueFor(opts.Language)
	result := FormResult{
		Role:     DefaultRole,
		DataDir:  opts.DataDir,
		Port:     strconv.Itoa(Plan{}.WithDefaults(opts.DataDir).Port),
		Hostname: Plan{}.WithDefaults(opts.DataDir).Hostname,
	}

	// The role first, because it decides which questions exist at all.
	roleOnly := FormResult{Role: DefaultRole}
	first := huh.NewForm(huh.NewGroup(
		huh.NewSelect[Role]().
			Title(language["roleTitle"]).
			Description(language["roleDescription"]).
			Options(
				huh.NewOption(language["roleAllInOne"], RoleAllInOne),
				huh.NewOption(language["roleServer"], RoleServer),
				huh.NewOption(language["rolePlayer"], RolePlayer),
			).
			Value(&roleOnly.Role),
	)).
		WithTheme(theme()).
		WithInput(opts.Input).
		WithOutput(opts.Output).
		WithWidth(72)
	if err := first.Run(); err != nil {
		return Result{}, err
	}
	result.Role = roleOnly.Role

	form := buildForm(&result, language).
		WithInput(opts.Input).
		WithOutput(opts.Output).
		WithWidth(72)
	if err := form.Run(); err != nil {
		return Result{}, err
	}
	if !result.Confirmed {
		return Result{}, ErrCancelled
	}

	plan, err := result.Parse(opts.DataDir)
	if err != nil {
		return Result{}, err
	}
	if err := plan.Validate(); err != nil {
		return Result{}, err
	}
	return Apply(plan)
}

// ErrCancelled is the last question answered "no". It is not a failure: nothing
// was written, which is exactly what was asked for.
var ErrCancelled = errors.New("setup: cancelled")
