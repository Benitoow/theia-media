// Package setup is the logic behind theia-setup: what a machine is for, what
// that implies, and the changes that follow from it.
//
// The role is declared, never detected. A mini-PC in a network cupboard and a
// home-theatre PC share a hardware fingerprint, so no probe can tell them apart
// (spec-fondatrice §14.3). Everything here takes the answer as an input.
//
// This package is deliberately free of terminal code: the TUI, the flags and the
// JSON status all call the same functions, so what the interface shows and what
// the machine does cannot drift apart.
package setup

import "fmt"

// Role is what a machine was declared to be.
type Role string

const (
	// RoleServer is a machine that only serves: a cupboard, a NAS, a spare PC.
	RoleServer Role = "server"

	// RolePlayer is a machine that only watches: a laptop pointed at somebody
	// else's library.
	RolePlayer Role = "player"

	// RoleAllInOne is both, which is the default: somebody running the installer
	// on their own computer wants to watch a film, not administer a server
	// (spec-fondatrice §14.3).
	RoleAllInOne Role = "all-in-one"
)

// DefaultRole is what the installer proposes.
const DefaultRole = RoleAllInOne

// Roles lists every role, in the order the interface offers them: the default
// first, because that is the answer most people want.
func Roles() []Role { return []Role{RoleAllInOne, RoleServer, RolePlayer} }

// ParseRole reads a role from a flag or a configuration file.
func ParseRole(value string) (Role, error) {
	switch Role(value) {
	case RoleServer:
		return RoleServer, nil
	case RolePlayer:
		return RolePlayer, nil
	case RoleAllInOne:
		return RoleAllInOne, nil
	case "":
		return DefaultRole, nil
	default:
		return "", fmt.Errorf("unknown role %q", value)
	}
}

// Valid reports whether a role names a machine at all.
func (r Role) Valid() bool {
	switch r {
	case RoleServer, RolePlayer, RoleAllInOne:
		return true
	default:
		return false
	}
}

// WantsServer reports whether this machine runs theia-server. A player-only
// machine still talks to somebody else's server, so "no" here means "do not
// install or start one", not "do not use one".
func (r Role) WantsServer() bool { return r == RoleServer || r == RoleAllInOne }

// WantsPlayer reports whether this machine runs theia-player.
func (r Role) WantsPlayer() bool { return r == RolePlayer || r == RoleAllInOne }

// OffersService reports whether an autostart entry makes sense. There is nothing
// to start on a player-only machine: the player is opened by a person.
func (r Role) OffersService() bool { return r.WantsServer() }
