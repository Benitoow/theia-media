package ffmpeg

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ulikunitz/xz"
)

type archiveFormat uint8

const (
	archiveZip archiveFormat = iota + 1
	archiveTarXZ
)

// A portable Jellyfin ffmpeg is currently under 110 MiB. The ceiling protects
// the data directory from a corrupt archive expanding without bound while
// leaving more than twice the measured headroom for a future runtime.
const maximumRuntimeBytes int64 = 256 << 20

// extractRuntime copies exactly one named executable out of a verified release
// package. Paths in the package are never materialised, so an archive cannot
// write beside the destination or install ffprobe as a second runtime.
func extractRuntime(packagePath, destination, runtimeName string, format archiveFormat) error {
	switch format {
	case archiveZip:
		return extractRuntimeZip(packagePath, destination, runtimeName)
	case archiveTarXZ:
		return extractRuntimeTarXZ(packagePath, destination, runtimeName)
	default:
		return fmt.Errorf("unknown package format %d", format)
	}
}

func extractRuntimeZip(packagePath, destination, runtimeName string) error {
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return err
	}
	defer packageFile.Close()
	info, err := packageFile.Stat()
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(packageFile, info.Size())
	if err != nil {
		return err
	}

	var found *zip.File
	for _, file := range archive.File {
		if file.Name != runtimeName {
			continue
		}
		if found != nil {
			return fmt.Errorf("package contains %s more than once", runtimeName)
		}
		found = file
	}
	if found == nil {
		return fmt.Errorf("package does not contain %s", runtimeName)
	}
	if found.UncompressedSize64 > uint64(maximumRuntimeBytes) {
		return fmt.Errorf("%s is larger than the %d-byte limit", runtimeName, maximumRuntimeBytes)
	}
	contents, err := found.Open()
	if err != nil {
		return err
	}
	defer contents.Close()
	return writeRuntime(destination, contents)
}

func extractRuntimeTarXZ(packagePath, destination, runtimeName string) error {
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return err
	}
	defer packageFile.Close()
	uncompressed, err := xz.NewReader(packageFile)
	if err != nil {
		return err
	}
	archive := tar.NewReader(uncompressed)
	found := false
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Name != runtimeName {
			continue
		}
		if found {
			return fmt.Errorf("package contains %s more than once", runtimeName)
		}
		// TypeRegA is the legacy writing of a regular file - a NUL type flag in
		// ustar and GNU tar alike - and an ffmpeg build archive may carry either.
		// The deprecation is about writing; this is a reader that has to accept
		// both, so the old name stays and the linter is told why.
		//
		//lint:ignore SA1019 a tar reader must accept the legacy regular-file flag
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("%s is not a regular file", runtimeName)
		}
		if header.Size > maximumRuntimeBytes {
			return fmt.Errorf("%s is larger than the %d-byte limit", runtimeName, maximumRuntimeBytes)
		}
		if err := writeRuntime(destination, archive); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("package does not contain %s", runtimeName)
	}
	return nil
}

func writeRuntime(destination string, source io.Reader) error {
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, io.LimitReader(source, maximumRuntimeBytes+1))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maximumRuntimeBytes {
		return fmt.Errorf("runtime exceeds the %d-byte limit", maximumRuntimeBytes)
	}
	return nil
}
