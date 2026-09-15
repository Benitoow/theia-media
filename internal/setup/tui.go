package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// The terminal interface: a short form, then the changes.
//
// Five things are deliberate here, and the first four were learned by looking at
// the thing on a real screen rather than by reading the code.
//
// **It is one form.** The first version asked the role in a form of its own and
// then rebuilt a second form whose first group was - the role again. The step
// really did appear twice, and the way it failed was worse than it sounds: the
// answer was already set, so pressing enter appeared to do nothing until it was
// pressed again. One form, with groups that hide themselves, is both shorter and
// honest about what is being asked.
//
// **The width is the terminal's.** A fixed 72 columns clipped every long line on
// a narrower console - "un mini-PC dans", "sans écran" - which is how a form
// looks broken while working perfectly.
//
// **The hint under the form is ours.** Huh builds its own from the focused
// field's key bindings, which is how a real screen came to read "↑ monter •
// ↓ descendre • / filtrer • ↓ descendre" - one word twice, in a language half
// translated, and never a word about leaving. It is a catalogue sentence now,
// drawn by formModel, and escape really does cancel instead of merely being
// printed as if it did.
//
// **Nothing is written before the confirmation**, which is the only promise an
// installer really has to keep. The last page shows the whole plan, live.
//
// The colours come from the design system: the accent is the same gold as the
// rest of the product.

// accent and friends are --color-accent, --color-bone, --color-parchment,
// --color-muted, --color-faint, --color-ink and --color-error from
// web/src/lib/tokens.css. Kept as literals because a terminal cannot read a CSS
// custom property, and named so the next person knows where they came from.
var (
	accent   = lipgloss.Color("#c8a24a")
	bone     = lipgloss.Color("#ede7dc")
	parchmnt = lipgloss.Color("#d6cfc2")
	muted    = lipgloss.Color("#8c857a")
	faint    = lipgloss.Color("#5a544c")
	ink      = lipgloss.Color("#0b0a09")
	danger   = lipgloss.Color("#d06a5d")
)

// maxFormWidth is where a form stops getting wider on a big screen. Beyond this
// the eye loses the left edge between a label and its answer; the design
// system's reading measure is 46rem and this is in the same territory.
const maxFormWidth = 76

// FormResult is what the form collected, before it becomes a Plan. Kept apart
// from Plan because the folders arrive as one blob of text and a port as a
// string: parsing is where mistakes happen, and it is tested on its own.
type FormResult struct {
	Role         Role
	DataDir      string
	InstallDir   string
	Port         string
	Hostname     string
	LibraryBlob  string
	InstallServi bool

	// Confirmed is the answer to the last question, and it is read: a form that
	// runs to completion and answers "no" is not an error, and the first version
	// of this discarded the answer and installed anyway.
	Confirmed bool
}

// Parse turns what a form (or a person) typed into a Plan.
//
// Separated from the terminal so it can be tested without one, and used by the
// flags as well: a folder list typed into a form and one passed as a flag must
// mean the same thing, including the empty lines somebody leaves behind.
func (r FormResult) Parse(defaultDataDir string) (Plan, error) {
	plan := Plan{
		Role:         r.Role,
		DataDir:      strings.TrimSpace(r.DataDir),
		InstallDir:   strings.TrimSpace(r.InstallDir),
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

// theme is Charm's own, with the product's palette on everything a person
// actually looks at.
//
// The blurred styles are not decoration: the header note rides on the first
// question and is never focusable, so it is drawn with the *blurred* title -
// which is how THEIA came out in Charm's indigo, the one colour on the screen
// that belongs to no palette. Every style a page can reach is set here, on both
// halves, rather than only the half that happened to show the fault.
func theme() *huh.Theme {
	t := huh.ThemeCharm()

	for _, styles := range []*huh.FieldStyles{&t.Focused, &t.Blurred} {
		styles.Title = styles.Title.Foreground(accent).Bold(true)
		styles.Description = styles.Description.Foreground(parchmnt)
		styles.ErrorIndicator = styles.ErrorIndicator.Foreground(danger)
		styles.ErrorMessage = styles.ErrorMessage.Foreground(danger)
		styles.NoteTitle = styles.NoteTitle.Foreground(accent).Bold(true)

		styles.SelectSelector = styles.SelectSelector.Foreground(accent)
		styles.SelectedOption = styles.SelectedOption.Foreground(accent)
		styles.UnselectedOption = styles.UnselectedOption.Foreground(parchmnt)
		styles.Option = styles.Option.Foreground(parchmnt)

		styles.TextInput.Prompt = styles.TextInput.Prompt.Foreground(accent)
		styles.TextInput.Text = styles.TextInput.Text.Foreground(bone)
		styles.TextInput.Cursor = styles.TextInput.Cursor.Foreground(accent)
		styles.TextInput.Placeholder = styles.TextInput.Placeholder.Foreground(faint)

		styles.FocusedButton = styles.FocusedButton.
			Foreground(ink).Background(accent).Bold(true)
		styles.BlurredButton = styles.BlurredButton.
			Foreground(parchmnt).Background(faint)
	}

	// A page that is not the current one is drawn muted rather than invisible.
	t.Blurred.Title = t.Blurred.Title.Foreground(muted)
	t.Blurred.Description = t.Blurred.Description.Foreground(muted)

	t.Help.Ellipsis = t.Help.Ellipsis.Foreground(muted)
	t.Help.ShortKey = t.Help.ShortKey.Foreground(parchmnt)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(muted)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(faint)
	return t
}

// keyMap is Huh's, with escape added to the way out.
//
// Huh binds ctrl+c and nothing else, so the key everybody presses to leave a
// form did nothing here - while the previous version of this screen printed
// "esc quitter" under the question, because SetHelp changes the label and not
// the binding. Escape now cancels from any page, and offering it is safe: this
// installer writes nothing before the confirmation.
func keyMap() *huh.KeyMap {
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	return keys
}

// helpLine is the single line of instruction printed under the form.
//
// Huh writes its own from the focused field's bindings, and on a real screen it
// read "↑ monter • ↓ descendre • / filtrer • ↓ descendre": the same word twice,
// because two bindings move the cursor down, and no mention of leaving, because
// Quit is the form's binding and not the field's. The sentences live in the
// catalogue with every other thing a person reads, and each one states the keys
// that page really accepts - enter validates an input but starts a new line in
// the folder box, which is exactly the kind of difference a hint is for.
func helpLine(field huh.Field, language Catalogue) string {
	switch field.(type) {
	case *huh.Select[Role]:
		return language["helpSelect"]
	case *huh.Text:
		return language["helpText"]
	case *huh.Confirm:
		return language["helpConfirm"]
	case *huh.Input:
		return language["helpInput"]
	}
	return ""
}

// footerStyle lines the hint up with the questions, which Huh indents inside its
// card. Left against the border it looks like output that fell out of the form.
var footerStyle = lipgloss.NewStyle().Foreground(muted).PaddingLeft(2)

// formModel draws the form and then the hint.
//
// Huh runs its own program inside Form.Run, which leaves no room between the
// form and the screen edges. This is that program, with one line added and one
// key bound: the form is unchanged and still updates its own fields.
type formModel struct {
	form     *huh.Form
	language Catalogue
	aborted  bool
}

func (m *formModel) Init() tea.Cmd { return m.form.Init() }

func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.form.Update(msg)
	if form, ok := updated.(*huh.Form); ok {
		m.form = form
	}
	if m.form.State == huh.StateAborted {
		m.aborted = true
	}
	// Huh only issues its finishing command when it owns the program, and here
	// it does not: the end of the form is ours to notice.
	if m.form.State != huh.StateNormal {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *formModel) View() string {
	view := m.form.View()
	if line := helpLine(m.form.GetFocusedField(), m.language); line != "" {
		view += "\n" + footerStyle.Render(line)
	}
	return view
}

// buildForm assembles the form. It is separate from Run so a test can drive it
// with key messages and assert what it draws, which is the only way to check a
// terminal interface without a terminal.
//
// The confirmation's description is a function, not a string: Huh evaluates a
// plain description when the form is built, so the summary would show the
// defaults rather than the answers somebody just typed.
func buildForm(result *FormResult, language Catalogue, width, height int) *huh.Form {
	// The header sits with the first question and is not focusable: a page that
	// only says hello and waits for enter is a step somebody has to take for
	// nothing, and this installer has already been accused of inventing steps.
	header := huh.NewNote().
		Title(language["brand"]).
		Description(language["intro"]).
		Next(false)

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
				return errors.New(language["pathsTitle"])
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
				return errors.New(language["portInvalid"])
			}
			return nil
		})

	host := huh.NewInput().
		Title(language["hostTitle"]).
		Description(language["hostDescription"]).
		Value(&result.Hostname).
		Validate(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(language["hostInvalid"])
			}
			return nil
		})

	folders := huh.NewText().
		Title(language["libraryTitle"]).
		Description(language["libraryHint"]).
		// A placeholder because an empty text box with no prompt is
		// indistinguishable from a page that failed to draw: on a real screen the
		// folder question was a blank rectangle under its own description.
		Placeholder(language["libraryPlaceholder"]).
		Lines(4).
		CharLimit(2000).
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

	// The role is asked once. Everything a player-only machine would not use is
	// hidden rather than asked and ignored: a question that does not apply is
	// worse than no question, because it makes somebody think it matters.
	serving := func() bool { return result.Role.WantsServer() }

	return huh.NewForm(
		huh.NewGroup(header, role),
		huh.NewGroup(where, port, host).WithHideFunc(func() bool { return !serving() }),
		huh.NewGroup(folders).WithHideFunc(func() bool { return !serving() }),
		huh.NewGroup(service).WithHideFunc(func() bool { return !serving() }),
		huh.NewGroup(confirm),
	).
		WithTheme(theme()).
		WithKeyMap(keyMap()).
		WithWidth(width).
		WithHeight(height).
		// The hint is drawn by formModel, under the form, from the catalogue.
		// Huh's own is the one with the repeated word in it.
		WithShowHelp(false)
}

// summary is what is about to happen, in the same words the result will use.
//
// It leads with the role, because that is the decision everything else follows
// from - and because a player-only machine has no port, no folders and no
// business writing a server's configuration, so without this line its
// confirmation would say almost nothing.
func summary(plan Plan, language Catalogue) string {
	lines := []string{fmt.Sprintf("%s : %s", language["statusRole"], roleLabel(plan.Role, language))}
	if strings.TrimSpace(plan.InstallDir) != "" {
		lines = append(lines, fmt.Sprintf("%s : %s", language["statusPrograms"], plan.InstallDir))
	}
	if plan.Role.WantsServer() {
		lines = append(lines,
			fmt.Sprintf("%s : %s", language["statusData"], plan.DataDir),
			fmt.Sprintf("%s : %d", language["statusPort"], plan.Port),
			fmt.Sprintf("%s : %s", language["statusHost"], plan.Hostname),
		)
		if len(plan.LibraryPaths) > 0 {
			lines = append(lines, fmt.Sprintf("%s : %s", language["statusLibrary"], strings.Join(plan.LibraryPaths, ", ")))
		} else {
			lines = append(lines, fmt.Sprintf("%s : %s", language["statusLibrary"], language["libraryNone"]))
		}
	}
	lines = append(lines, fmt.Sprintf("%s : %s", language["statusAuto"],
		map[bool]string{true: language["statusYes"], false: language["statusNo"]}[plan.Service]))
	return strings.Join(lines, "\n")
}

// FormOptions is how a caller supplies a terminal. Tests pass buffers.
type FormOptions struct {
	Input    io.Reader
	Output   io.Writer
	Language string
	// DataDir is the default the form proposes, already resolved by the caller.
	DataDir string
	// InstallDir is where the programs will be copied. Empty means this user's
	// standard place, and the form says which one before anything is written.
	InstallDir string
	// Source says where a program that is not on this machine comes from: a
	// folder or an archive somebody already has, or the release page.
	Source Source
	// Port, Hostname, Library and Service are answers somebody may already have
	// typed on the command line. They seed the form rather than replacing it:
	// `theia-setup --data-dir D:\Theia` still asks every question, with that
	// answer in it. Ignoring them was a real fault - the flag was accepted, and
	// the form then proposed somewhere else.
	Port     int
	Hostname string
	Library  []string
	Service  bool
}

// formDefaults is what the form starts with: this machine's defaults, with
// whatever the command line already settled placed on top.
//
// The confirmation starts on "install". It is the last page, the plan is
// printed above it, and the alternative is a hidden trap: somebody pressing
// enter out of habit would cancel the installation and be told nothing was
// written, which reads like a failure rather than like an answer.
func formDefaults(opts FormOptions) (FormResult, error) {
	defaults := Plan{}.WithDefaults(opts.DataDir)
	installDir := strings.TrimSpace(opts.InstallDir)
	if installDir == "" {
		resolved, err := DefaultInstallDir()
		if err != nil {
			return FormResult{}, err
		}
		installDir = resolved
	}
	result := FormResult{
		Role:         DefaultRole,
		DataDir:      defaults.DataDir,
		InstallDir:   installDir,
		Port:         strconv.Itoa(defaults.Port),
		Hostname:     defaults.Hostname,
		InstallServi: opts.Service,
		Confirmed:    true,
	}
	if opts.Port != 0 {
		result.Port = strconv.Itoa(opts.Port)
	}
	if hostname := strings.TrimSpace(opts.Hostname); hostname != "" {
		result.Hostname = hostname
	}
	if len(opts.Library) > 0 {
		result.LibraryBlob = strings.Join(opts.Library, "\n")
	}
	return result, nil
}

// formSize asks the terminal how big it is.
//
// The width leaves a margin, because a form that fills the last column is a form
// with its border against the edge. The height is deliberately tighter than the
// screen: Huh reserves the height it is given for every page, so a tall number
// fills the screen with empty space under a five-line question - which is what
// "there are empty spaces" meant.
//
// A size that cannot be read - a pipe, a test - falls back to something sensible
// rather than to something narrow: nothing here knows better than the screen.
func formSize(output io.Writer) (int, int) {
	if file, ok := output.(*os.File); ok {
		if width, height, err := term.GetSize(file.Fd()); err == nil && width > 0 {
			if width-6 < maxFormWidth {
				return width - 6, clamped(height)
			}
			return maxFormWidth, clamped(height)
		}
	}
	return maxFormWidth, clamped(0)
}

func clamped(height int) int {
	if height <= 0 {
		return 14
	}
	// Enough for the tallest page - a folder box of four lines under its title -
	// and no more.
	if height-8 < 16 {
		return height - 8
	}
	return 16
}

// RunInteractive shows the installer, then installs what was agreed: the
// programs, the configuration and the autostart entry, in that order.
//
// The programs come after the confirmation and never before it, which is the
// promise the first page makes. A download is the one step here that can take
// minutes, so it is the one step that draws what it is doing.
func RunInteractive(opts FormOptions) (Result, error) {
	language, _ := CatalogueFor(opts.Language)
	result, err := formDefaults(opts)
	if err != nil {
		return Result{}, err
	}

	width, height := formSize(opts.Output)
	form := buildForm(&result, language, width, height)
	// Huh issues these two when it runs its own program; formModel runs it
	// instead, so the end of the form has to be able to stop the program.
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Quit

	program := []tea.ProgramOption{tea.WithOutput(opts.Output)}
	if opts.Input != nil {
		program = append(program, tea.WithInput(opts.Input))
	}
	model := &formModel{form: form, language: language}
	if _, err := tea.NewProgram(model, program...).Run(); err != nil {
		return Result{}, err
	}
	if model.aborted || !result.Confirmed {
		return Result{}, ErrCancelled
	}

	plan, err := result.Parse(opts.DataDir)
	if err != nil {
		return Result{}, err
	}
	if err := plan.Validate(); err != nil {
		return Result{}, err
	}

	var installed Result
	err = RunProgress(opts.Output, language, func(ctx context.Context, report Reporter) error {
		installed, err = Install(ctx, plan, opts.Source, language, report)
		return err
	})
	if err != nil {
		return installed, err
	}
	return installed, nil
}

// ErrCancelled is the last question answered "no". It is not a failure: nothing
// was written, which is exactly what was asked for.
var ErrCancelled = errors.New("setup: cancelled")
