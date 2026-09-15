package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// ErrInterrupted is control-C on the progress screen.
//
// It is deliberately not ErrCancelled, which is the answer "no" to the last
// question and promises that nothing was written. By the time a download is
// running that promise no longer holds: what was already installed stays.
var ErrInterrupted = errors.New("setup: interrupted")

// The bar somebody watches while a hundred megabytes arrive.
//
// It exists because the installer now downloads, and a download with no sign of
// life is indistinguishable from a hang. The first version of this printed
// nothing at all until it was finished, which is why it was replaced by
// something that can be watched - and then watched, on a real screen, at a real
// size.
//
// It is Bubble Tea, like the form, so the two read as one program rather than as
// a form that spawns a progress bar. What it says comes from the catalogue:
// phases arrive as codes, the sentences are here with every other sentence.

// barWidth is how wide the drawn bar gets. Narrow enough for an eighty-column
// console with the numbers beside it.
const barWidth = 24

// RunProgress runs work while drawing what it is doing, and returns the error
// the work produced.
//
// A pipe or a log gets lines instead of a bar: a progress bar writes control
// characters, and control characters in a log file are a bug report waiting to
// be written.
func RunProgress(output io.Writer, language Catalogue, work func(context.Context, Reporter) error) error {
	if output == nil {
		output = os.Stdout
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, _, err := termSize(output); err != nil {
		plain := &plainReporter{out: output, language: language}
		return work(ctx, plain)
	}

	model := &progressModel{language: language}
	program := tea.NewProgram(model, tea.WithOutput(output))
	reporter := &teaReporter{program: program}

	go func() {
		program.Send(finishedMsg{err: work(ctx, reporter)})
	}()

	if _, err := program.Run(); err != nil {
		return err
	}
	if model.cancel {
		// Stopping the work matters more than tidying up after it: the download
		// is cancelled through the context, and this process is about to end.
		// Whatever was staged is refused by name rather than renamed into place,
		// so an interrupted download cannot look installed.
		cancel()
		return ErrInterrupted
	}
	return model.err
}

// termSize asks a writer how big the terminal behind it is.
func termSize(output io.Writer) (int, int, error) {
	file, ok := output.(*os.File)
	if !ok {
		return 0, 0, os.ErrInvalid
	}
	width, height, err := term.GetSize(file.Fd())
	if err != nil || width <= 0 {
		return 0, 0, os.ErrInvalid
	}
	return width, height, nil
}

// The messages the work sends to the drawing.
type (
	phaseMsg struct {
		phase  Phase
		detail string
	}
	bytesMsg    struct{ done, total int64 }
	finishedMsg struct{ err error }
)

// teaReporter is the Reporter a Bubble Tea program needs. Send is safe from
// another goroutine, which is the whole reason the work runs on one.
type teaReporter struct{ program *tea.Program }

func (r *teaReporter) Phase(phase Phase, detail string) {
	r.program.Send(phaseMsg{phase: phase, detail: detail})
}

func (r *teaReporter) Progress(done, total int64) {
	r.program.Send(bytesMsg{done: done, total: total})
}

type progressModel struct {
	language Catalogue
	phase    Phase
	detail   string
	done     int64
	total    int64
	cancel   bool
	err      error
}

func (m *progressModel) Init() tea.Cmd { return nil }

func (m *progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case phaseMsg:
		m.phase = msg.phase
		m.detail = msg.detail
		// A new phase starts its own count: carrying the previous program's
		// bytes into the next download would draw a bar that begins half full.
		m.done, m.total = 0, 0
	case bytesMsg:
		m.done, m.total = msg.done, msg.total
	case finishedMsg:
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		// The only key this screen takes is the one that stops it. Escape is
		// what a person presses, and control-C is what they press when the first
		// one did nothing; a progress screen that ignores both is worse than no
		// screen at all.
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.cancel = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *progressModel) View() string {
	var view strings.Builder
	view.WriteString(brandStyle.Render(m.language["brand"]))
	view.WriteString("\n\n")
	view.WriteString(m.sentence())
	view.WriteString("\n")
	view.WriteString(m.bar())
	view.WriteString("\n\n")
	view.WriteString(hintStyle.Render(m.language["progressInterrupt"]))
	view.WriteString("\n")
	// Indented like the form, because it is the same program: the two screens
	// have to share a left edge or they read as two tools.
	return screenStyle.Render(view.String())
}

var screenStyle = lipgloss.NewStyle().PaddingLeft(2)

var hintStyle = lipgloss.NewStyle().Foreground(muted)

// brandStyle is the header the form also wears, so the two screens are visibly
// the same program.
var brandStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)

// sentence is what this phase is, in words. An unknown program name is shown as
// it arrived rather than swallowed: a blank line where a name belongs is how a
// missing catalogue entry goes unnoticed.
func (m *progressModel) sentence() string {
	switch m.phase {
	case PhaseDownloading:
		return fmt.Sprintf(m.language["progressDownload"], m.program())
	case PhaseExtracting:
		return fmt.Sprintf(m.language["progressExtract"], m.program())
	case PhaseInstalling:
		return fmt.Sprintf(m.language["progressInstall"], m.detail)
	case PhaseDone:
		return m.language["progressDone"]
	default:
		return m.language["progressChecking"]
	}
}

func (m *progressModel) program() string {
	if label, ok := m.language["program."+m.detail]; ok {
		return label
	}
	return m.detail
}

// bar draws the bar, the percentage and the bytes. It is empty for a phase that
// has no count, and the line stays so the screen does not jump between phases.
func (m *progressModel) bar() string {
	if m.phase != PhaseDownloading {
		return ""
	}
	if m.total <= 0 {
		// The size is known before the first byte in practice; until then the
		// honest thing is to show that something is happening.
		return faintStyle.Render(strings.Repeat("░", barWidth)) + "  " + m.language["progressWaiting"]
	}
	fraction := float64(m.done) / float64(m.total)
	if fraction > 1 {
		fraction = 1
	}
	filled := int(fraction * barWidth)
	bar := accentStyle.Render(strings.Repeat("█", filled)) +
		faintStyle.Render(strings.Repeat("░", barWidth-filled))
	return fmt.Sprintf("%s  %3d %%  %s / %s",
		bar, int(fraction*100), humanSize(m.done, m.language), humanSize(m.total, m.language))
}

var (
	accentStyle = lipgloss.NewStyle().Foreground(accent)
	faintStyle  = lipgloss.NewStyle().Foreground(faint)
)

// humanSize writes a size the way a person reads one: whole megabytes, and
// gigabytes with one decimal, in the catalogue's own separator. A size is a
// sentence too, which is why the units and the separator are translated and the
// arithmetic is not.
func humanSize(bytes int64, language Catalogue) string {
	if bytes < 0 {
		bytes = 0
	}
	const (
		megabyte = 1 << 20
		gigabyte = 1 << 30
	)
	if bytes >= gigabyte {
		return decimal(float64(bytes)/float64(gigabyte), 1, language) + " " + language["unitGB"]
	}
	return fmt.Sprintf("%d %s", bytes/megabyte, language["unitMB"])
}

func decimal(value float64, places int, language Catalogue) string {
	text := fmt.Sprintf("%.*f", places, value)
	if separator := language["decimalSeparator"]; separator != "" && separator != "." {
		text = strings.Replace(text, ".", separator, 1)
	}
	return text
}

// plainReporter is the Reporter for somewhere that is not a screen: one line per
// phase, and nothing that would have to be erased.
type plainReporter struct {
	out      io.Writer
	language Catalogue
	last     Phase
	detail   string
}

// Phase prints a line when the phase changes, and also when the program changes
// within one phase: fetching the server and fetching the player are two facts,
// and a log that announced only the first would hide half the installation.
func (r *plainReporter) Phase(phase Phase, detail string) {
	if phase == r.last && detail == r.detail {
		return
	}
	r.last, r.detail = phase, detail
	model := &progressModel{language: r.language, phase: phase, detail: detail}
	fmt.Fprintln(r.out, model.sentence())
}

// Progress is ignored here on purpose: a log wants the phases, not four hundred
// lines about bytes.
func (r *plainReporter) Progress(int64, int64) {}
