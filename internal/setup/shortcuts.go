package setup

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// The entries an installation leaves in front of a person.
//
// Copying the programs into a directory is only half of an installation: a
// program nobody can find is a program nobody runs. This is the half that makes
// Theia appear when somebody types its name - in the Start Menu, in a launcher
// such as Flow Launcher, and on the Desktop. Without it the installer produced a
// configuration and a folder, and the maintainer's own words were "there is no
// TA Media, no TA Server, no TA Player" when searching.
//
// The names and the descriptions are catalogue entries, like every other thing
// somebody reads: the names happen to be the same in both languages, and the
// tooltips are sentences.

// shortcutEntry is one entry and the name it carries.
type shortcutEntry struct {
	name string
	link Shortcut
}

// ShortcutTargets is where an installation puts its entries.
//
// Passed in rather than resolved inside, because a test that writes into the
// real Start Menu of whoever runs the suite is a test nobody dares run twice.
// An empty desktop means no desktop entry.
type ShortcutTargets struct {
	// StartMenu is the folder the entries are written into.
	StartMenu string
	// Desktop is empty when no desktop entry is wanted.
	Desktop string
}

// entriesFor is what this role's installation adds: the product, which starts
// the thing this machine is for, and then each program it actually installed.
//
// No entry names an icon, and that is deliberate: Windows draws the target's
// own. The server and the installer used to carry none - a Go binary has no
// resource unless one is given to it - so every shortcut borrowed the player's
// icon, and a machine with only the server had nothing to borrow. Both binaries
// embed the product's mark now (cmd/*/rsrc_windows_*.syso), so each entry shows
// what it actually starts.
func entriesFor(plan Plan, text Catalogue) []shortcutEntry {
	installDir := plan.InstallDir
	executable := func(base string) string {
		if runtime.GOOS == "windows" {
			return filepath.Join(installDir, base+".exe")
		}
		return filepath.Join(installDir, base)
	}

	// The product entry comes first: it is the one somebody looks for by name,
	// and the only one that also goes on the Desktop.
	// The product entry is what somebody opens to watch something. On an
	// all-in-one machine the server is infrastructure started in the background;
	// making it the primary shortcut exposed a console and left the actual player
	// as a second application the viewer had to discover.
	primary := "theia-server"
	if plan.Role.WantsPlayer() {
		primary = "theia-player"
	}
	entries := []shortcutEntry{{
		name: text["shortcutTheiaName"],
		link: Shortcut{
			Target:      executable(primary),
			WorkingDir:  installDir,
			Description: text["shortcutTheia"],
		},
	}}

	if plan.Role.WantsServer() {
		entries = append(entries, shortcutEntry{
			name: text["shortcutServerName"],
			link: Shortcut{
				Target:      executable("theia-server"),
				WorkingDir:  installDir,
				Description: text["shortcutServer"],
			},
		})
	}
	if plan.Role.WantsPlayer() {
		entries = append(entries, shortcutEntry{
			name: text["shortcutPlayerName"],
			link: Shortcut{
				Target:      executable("theia-player"),
				WorkingDir:  installDir,
				Description: text["shortcutPlayer"],
			},
		})
	}
	return entries
}

// createShortcuts writes the entries into the Start Menu and, for the product
// itself, onto the Desktop.
//
// A shortcut that cannot be created is reported rather than fatal: the programs
// are installed and they run, and refusing the whole installation because a
// launcher's folder was read-only would be the installer's convenience winning
// over the person's. The returned sentence is empty when everything was written.
func createShortcuts(plan Plan, targets ShortcutTargets, text Catalogue) ([]Action, string) {
	if runtime.GOOS != "windows" {
		// Only Windows is verified in V3.3, and a .desktop file written by a
		// guess would be an unverified claim about somebody's menu. The programs
		// are installed; the entries wait for a platform that has actually been
		// run, which is the rule the whole project is held to.
		return nil, ""
	}
	if strings.TrimSpace(targets.StartMenu) == "" {
		return nil, ""
	}

	entries := entriesFor(plan, text)
	actions := make([]Action, 0, len(entries)+1)
	failures := make([]string, 0, 1)

	for index, entry := range entries {
		// A program that is not there gets no entry: a shortcut to a missing
		// file is a broken promise somebody double-clicks.
		if !fileExists(entry.link.Target) {
			failures = append(failures, fmt.Sprintf("%s: %s", entry.name, entry.link.Target))
			continue
		}

		// One folder for the product's entries, which is what a launcher groups
		// and what somebody finds in the Start Menu under one name. It carries
		// the product's name, not the wordmark: "THEIA" is how the header is
		// drawn, "Theia" is how a folder is spelled.
		link := entry.link
		link.Path = filepath.Join(targets.StartMenu, text["shortcutTheiaName"], entry.name+".lnk")
		if err := WriteShortcut(link); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		actions = append(actions, Action{Kind: "created-shortcut", Path: link.Path, Detail: "start-menu"})

		// The product alone goes on the Desktop. Three icons for one program is
		// how a Desktop stops being a place somebody put things.
		if index != 0 || strings.TrimSpace(targets.Desktop) == "" {
			continue
		}
		onDesktop := entry.link
		onDesktop.Path = filepath.Join(targets.Desktop, entry.name+".lnk")
		if err := WriteShortcut(onDesktop); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		actions = append(actions, Action{Kind: "created-shortcut", Path: onDesktop.Path, Detail: "desktop"})
	}

	if len(failures) > 0 {
		return actions, strings.Join(failures, "; ")
	}
	return actions, ""
}

// InstalledTargets is where this machine keeps its entries: the Start Menu folder
// every user has, and the Desktop the shell reports - which is not always
// %USERPROFILE%\Desktop, since OneDrive moves it.
func InstalledTargets() ShortcutTargets {
	targets := ShortcutTargets{}
	if dir, err := StartMenuDir(); err == nil {
		targets.StartMenu = dir
	}
	if dir, err := DesktopDir(); err == nil {
		targets.Desktop = dir
	}
	return targets
}
