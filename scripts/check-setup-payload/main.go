// check-setup-payload verifies that a public one-file setup contains its own
// complete product and the exact server binary the updater downloads.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	setup := flag.String("setup", "", "published standalone setup")
	server := flag.String("server", "", "published server binary")
	target := flag.String("target", "", "windows-amd64, windows-arm64, darwin-arm64 or darwin-amd64")
	flag.Parse()
	if *setup == "" || *server == "" || *target == "" {
		fmt.Fprintln(os.Stderr, "usage: check-setup-payload -setup <file> -server <file> -target <platform>")
		os.Exit(2)
	}
	if err := check(*setup, *server, *target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(setup, server, target string) error {
	want := []string{"START-HERE.txt"}
	member := "theia-server"
	switch target {
	case "windows-amd64", "windows-arm64":
		member += ".exe"
		want = append(want, member, "theia-setup.exe", "theia.exe", "theia-player.exe", "libmpv-2.dll", "LICENSE-libmpv.txt", "NOTICE.md")
	case "darwin-arm64", "darwin-amd64":
		want = append(want, member, "theia-setup", "theia", "Theia.app/Contents/MacOS/theia-player", "Theia.app/Contents/Resources/LICENSE-libmpv.txt", "Theia.app/Contents/Resources/NOTICE.md", "Theia.app/Contents/Frameworks/libmpv.2.dylib")
	default:
		return fmt.Errorf("unsupported target %s", target)
	}
	z, err := zip.OpenReader(setup)
	if err != nil {
		return fmt.Errorf("read %s: %w", setup, err)
	}
	defer z.Close()
	found := map[string]*zip.File{}
	for _, f := range z.File {
		name := strings.TrimPrefix(f.Name, "./")
		if _, duplicate := found[name]; duplicate {
			return fmt.Errorf("duplicate setup member %s", name)
		}
		found[name] = f
	}
	for _, name := range want {
		if found[name] == nil {
			return fmt.Errorf("setup lacks %s", name)
		}
	}
	inside, err := found[member].Open()
	if err != nil {
		return err
	}
	defer inside.Close()
	outside, err := os.Open(server)
	if err != nil {
		return err
	}
	defer outside.Close()
	a, err := digest(inside)
	if err != nil {
		return err
	}
	b, err := digest(outside)
	if err != nil {
		return err
	}
	if a != b {
		return fmt.Errorf("embedded server differs from %s", server)
	}
	fmt.Printf("%s: %d payload members; server matches public updater asset\n", target, len(z.File))
	return nil
}

func digest(r io.Reader) ([sha256.Size]byte, error) {
	h := sha256.New()
	_, err := io.Copy(h, r)
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, err
}
