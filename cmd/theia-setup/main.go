// Command theia-setup declares what a machine is for, installs what that
// implies, and maintains it afterwards.
//
// It exists because the machine cannot be asked to guess: a mini-PC in a network
// cupboard and a home-theatre PC under a television share a hardware
// fingerprint, so the role is declared at installation time
// (docs/spec-fondatrice.md §14.3). Nothing here is imposed: it writes a
// configuration, and installs an autostart entry only when asked.
//
// Run it with no arguments for the terminal form, or with flags for a script:
//
//	theia-setup                                  the form
//	theia-setup --role server --library D:\Films --service
//	theia-setup --check [--json]                 what this machine is, now
//	theia-setup --update [--json]                check for a new version
//	theia-setup --service remove                 undo the autostart entry
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/setup"
)

// version is overwritten at build time, exactly like the server's.
var version = "dev"

// errAlreadyReported means the failure has been explained to the user in their
// own language, and main must not print the raw error again underneath it.
var errAlreadyReported = errors.New("already reported")

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errAlreadyReported) {
			os.Exit(1)
		}
		language, _ := setup.CatalogueFor("")
		fmt.Fprintf(os.Stderr, "\n%s %s\n", language["errorPrefix"], err)
		os.Exit(1)
	}
}

func run() error {
	var (
		role        = flag.String("role", "", "what this machine is for: all-in-one, server, or player")
		dataDir     = flag.String("data-dir", "", "directory holding the configuration, database and cache")
		library     = flag.String("library", "", "folders to scan, separated by the path-list separator")
		port        = flag.Int("port", 0, "port the server listens on")
		hostname    = flag.String("hostname", "", "name announced on the network")
		service     = flag.Bool("service", false, "install an autostart entry for the server")
		serviceCmd  = flag.String("service-action", "", "install, remove, or status")
		check       = flag.Bool("check", false, "print what this machine is, changing nothing")
		checkUpdate = flag.Bool("check-update", false, "ask GitHub Releases what the latest version is, downloading nothing")
		update      = flag.Bool("update", false, "install the latest version of the server, verifying its digest")
		jsonOutput  = flag.Bool("json", false, "print the result as JSON")
		language    = flag.String("lang", "", "fr or en; French by default")
		showVersion = flag.Bool("version", false, "print the version and exit")
		yes         = flag.Bool("yes", false, "assume yes where a form would ask")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("theia-setup %s\n", version)
		return nil
	}

	text, _ := setup.CatalogueFor(*language)
	defaultDir, err := config.DataDir()
	if err != nil {
		return err
	}

	switch {
	case *check:
		return reportStatus(*jsonOutput, text)
	case *checkUpdate:
		return updateAction(false, *jsonOutput, text)
	case *update:
		// `--update` updates. A flag that only checked would be a flag whose name
		// lies, which is why `--check-update` exists beside it.
		return updateAction(true, *jsonOutput, text)
	case *serviceCmd != "":
		return serviceAction(*serviceCmd, *jsonOutput, text)
	case isInteractive(*role, *library):
		result, err := setup.RunInteractive(setup.FormOptions{
			Input:    os.Stdin,
			Output:   os.Stdout,
			Language: *language,
			DataDir:  defaultDir,
		})
		if errors.Is(err, setup.ErrCancelled) {
			fmt.Println(text["cancelled"])
			return nil
		}
		if err != nil {
			return err
		}
		printResult(result, text)
		return nil
	default:
		return runOnce(onceOptions{
			role: *role, dataDir: *dataDir, library: *library, port: *port,
			hostname: *hostname, service: *service, yes: *yes,
			jsonOutput: *jsonOutput, defaultDir: defaultDir, text: text,
		})
	}
}

// isInteractive decides which of the two installation paths runs. A role or a
// library on the command line means somebody is scripting this; no flags at all
// means somebody is sitting in front of it.
func isInteractive(role, library string) bool {
	return role == "" && library == ""
}

type onceOptions struct {
	role       string
	dataDir    string
	library    string
	port       int
	hostname   string
	service    bool
	yes        bool
	jsonOutput bool
	defaultDir string
	text       setup.Catalogue
}

// runOnce is the flag path: the same Plan and the same Apply as the form, which
// is the point - a scripted installation and a typed one cannot diverge.
func runOnce(opts onceOptions) error {
	parsed, err := setup.ParseRole(opts.role)
	if err != nil {
		return err
	}
	plan := setup.Plan{
		Role:         parsed,
		DataDir:      opts.dataDir,
		LibraryPaths: splitList(opts.library),
		Port:         opts.port,
		Hostname:     opts.hostname,
		Service:      opts.service,
	}.WithDefaults(opts.defaultDir)

	if err := plan.Validate(); err != nil {
		return err
	}
	// Installing an autostart entry is a change to somebody's machine, so the
	// flag path asks once - unless the caller said --yes, which is what a script
	// or an unattended install passes.
	if plan.Service && !opts.yes {
		return fmt.Errorf("%s --service --yes", opts.text["serviceTitle"])
	}

	result, err := setup.Apply(plan)
	if err != nil {
		return err
	}
	if opts.jsonOutput {
		return printJSON(result)
	}
	printResult(result, opts.text)
	return nil
}

func serviceAction(action string, jsonOutput bool, text setup.Catalogue) error {
	switch action {
	case "install":
		plan, err := currentPlan()
		if err != nil {
			return err
		}
		plan.Service = true
		result, err := setup.Apply(plan)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(result)
		}
		printResult(result, text)
		return nil
	case "remove":
		removed, err := setup.RemoveAutostart()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(removed)
		}
		if removed.Kind == "" {
			fmt.Println(text["serviceNone"])
			return nil
		}
		fmt.Printf("%s %s\n", text["serviceRemoved"], removed.Kind)
		return nil
	case "status":
		status, err := setup.Inspect(selfPath())
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(status.Autostart)
		}
		if !status.Autostart.Installed {
			fmt.Println(text["serviceNone"])
			return nil
		}
		fmt.Printf("%s %s (%s)\n", text["statusAuto"], status.Autostart.Kind, status.Autostart.Path)
		return nil
	default:
		return fmt.Errorf("--service-action takes install, remove or status, not %q", action)
	}
}

// currentPlan reads what this machine is configured as, so a service can be
// installed later without re-answering every question.
func currentPlan() (setup.Plan, error) {
	dir, err := config.DataDir()
	if err != nil {
		return setup.Plan{}, err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return setup.Plan{}, err
	}
	return setup.Plan{
		Role:         setup.RoleServer,
		DataDir:      dir,
		LibraryPaths: cfg.LibraryPaths,
		Port:         cfg.Port,
		Hostname:     cfg.Hostname,
	}.WithDefaults(dir), nil
}

func reportStatus(jsonOutput bool, text setup.Catalogue) error {
	status, err := setup.Inspect(selfPath())
	if err != nil {
		return err
	}
	if jsonOutput {
		return printJSON(status)
	}
	fmt.Printf("\n  %s\n\n", text["statusTitle"])
	fmt.Printf("  %-18s %s\n", text["statusData"], status.DataDir)
	configured := text["statusNo"]
	if status.Configured {
		configured = text["statusYes"]
	}
	fmt.Printf("  %-18s %s\n", text["statusConfig"], configured)
	if status.Configured {
		fmt.Printf("  %-18s %d\n", text["statusPort"], status.Port)
		fmt.Printf("  %-18s %s\n", text["statusHost"], status.Hostname)
	}
	folders := text["statusNone"]
	if len(status.LibraryPaths) > 0 {
		folders = strings.Join(status.LibraryPaths, ", ")
	}
	fmt.Printf("  %-18s %s\n", text["statusLibrary"], folders)
	autostart := text["statusNone"]
	if status.Autostart.Installed {
		autostart = status.Autostart.Kind + " (" + status.Autostart.Path + ")"
	}
	fmt.Printf("  %-18s %s\n", text["statusAuto"], autostart)
	if len(status.Artifacts) > 0 {
		// The label once, then the list under it: a repeated label on every line
		// reads as a table that lost its column, and the second one is always the
		// same word as the first.
		fmt.Printf("  %s\n", text["statusServers"])
		for _, artifact := range status.Artifacts {
			state := text["statusMissing"]
			if artifact.Found {
				state = artifact.Path
			}
			// The name it was found under, not the name looked for first: on a
			// downloaded release those differ, and showing the wrong one makes a
			// correct installation look broken.
			name := artifact.Name
			if artifact.Found {
				name = filepath.Base(artifact.Path)
			}
			fmt.Printf("    %-34s %s\n", name, state)
		}
	}
	fmt.Println()
	return nil
}

// updateAction checks, and installs only when told to. The report is the same
// either way, so a script can read one shape.
func updateAction(install bool, jsonOutput bool, text setup.Catalogue) error {
	target, err := setup.ResolveUpdateTarget()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if !jsonOutput {
		fmt.Println(text["updateChecking"])
	}
	status, err := setup.CheckForUpdate(ctx, target)
	if err != nil {
		return err
	}

	if !status.Available || !install {
		if jsonOutput {
			return printJSON(status)
		}
		switch {
		case status.Available:
			fmt.Printf("%s %s -> %s\n", text["updateAvailable"], status.Current, status.Latest)
		case status.State == "idle" && status.Reason == "up_to_date":
			fmt.Printf("%s %s (%s)\n", text["updateNewest"], status.Current, status.Latest)
		default:
			// Not available is not the same as up to date. The updater refuses to
			// touch a development build, and reporting that as "up to date" is a
			// lie the person reading it has no way to see through - the first
			// version of this printed exactly that.
			fmt.Printf("%s %s\n", text["updateHeld"], reasonSentence(status, text))
		}
		return nil
	}

	if !jsonOutput {
		fmt.Printf("%s %s -> %s\n", text["updateAvailable"], status.Current, status.Latest)
	}
	applied, err := setup.ApplyUpdate(ctx, target)
	if err != nil {
		if jsonOutput {
			return printJSON(applied)
		}
		// The log line first, in English, for whoever has to diagnose it - the
		// same shape the server logs - and then the sentence, in the user's
		// language. Printing the Go error at somebody is how a French page once
		// showed a Windows syscall name (decision 25), and this tool did exactly
		// that on its first real failure.
		fmt.Fprintf(os.Stderr, "theia-setup: update failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "%s %s\n", text["updateFailed"], reasonSentence(applied, text))
		return errAlreadyReported
	}
	if jsonOutput {
		return printJSON(applied)
	}
	fmt.Printf("%s %s\n", text["updateApplied"], applied.Latest)
	return nil
}

// reasonSentence turns the updater's reason code into a sentence, and admits
// when it does not know one rather than printing nothing.
//
// The updater sends codes because the interface owns every word (decision 25);
// this is that interface, in two languages, at a terminal.
func reasonSentence(status setup.UpdateStatusView, text setup.Catalogue) string {
	known := map[string]string{
		"development_build":      "reasonDevelopmentBuild",
		"up_to_date":             "reasonUpToDate",
		"no_release":             "reasonNoRelease",
		"github_unreachable":     "reasonGitHub",
		"no_binary_for_platform": "reasonNoBinary",
	}
	if key, ok := known[status.Reason]; ok {
		return text[key]
	}
	if status.Reason != "" {
		return text["reasonOther"] + " " + status.Reason
	}
	return status.Message
}

func printResult(result setup.Result, text setup.Catalogue) {
	fmt.Printf("\n%s\n\n", text["done"])
	for _, action := range result.Actions {
		switch action.Kind {
		case "created-dir":
			fmt.Printf("  %s %s\n", text["actDir"], action.Path)
		case "wrote-config":
			fmt.Printf("  %s %s\n", text["actConfig"], action.Path)
		case "already-configured":
			fmt.Printf("  %s %s\n", text["actKnown"], action.Path)
		case "installed-service":
			fmt.Printf("  %s %s (%s)\n", text["actServ"], action.Detail, text["mechanism"])
		case "kept-service":
			fmt.Printf("  %s\n", text["actNone"])
		}
	}
	if result.ServiceError != "" {
		fmt.Printf("  %s %s\n", text["servFail"], result.ServiceError)
	}
	fmt.Println()
}

func printJSON(value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func selfPath() string {
	path, err := os.Executable()
	if err != nil {
		return "."
	}
	return path
}

// splitList reads a path list with the platform's own separator: ';' on
// Windows, ':' elsewhere - the same convention PATH uses, because a Windows path
// contains a colon and a Unix one does not.
func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	separator := string(os.PathListSeparator)
	parts := strings.Split(value, separator)
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		absolute, err := filepath.Abs(trimmed)
		if err != nil {
			paths = append(paths, trimmed)
			continue
		}
		paths = append(paths, absolute)
	}
	return paths
}
