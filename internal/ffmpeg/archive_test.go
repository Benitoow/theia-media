package ffmpeg

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestExtractRuntimeFromPortablePackages(t *testing.T) {
	want := []byte("a verified ffmpeg executable")

	t.Run("zip", func(t *testing.T) {
		packagePath := filepath.Join(t.TempDir(), "ffmpeg.zip")
		file, err := os.Create(packagePath)
		if err != nil {
			t.Fatal(err)
		}
		archive := zip.NewWriter(file)
		for name, contents := range map[string][]byte{
			"ffprobe.exe": []byte("not installed"),
			"ffmpeg.exe":  want,
		} {
			entry, err := archive.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(contents); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}

		assertExtractedRuntime(t, packagePath, "ffmpeg.exe", archiveZip, want)
	})

	t.Run("tar xz", func(t *testing.T) {
		packagePath := filepath.Join(t.TempDir(), "ffmpeg.tar.xz")
		file, err := os.Create(packagePath)
		if err != nil {
			t.Fatal(err)
		}
		compressed, err := xz.NewWriter(file)
		if err != nil {
			t.Fatal(err)
		}
		archive := tar.NewWriter(compressed)
		for name, contents := range map[string][]byte{
			"ffprobe": []byte("not installed"),
			"ffmpeg":  want,
		} {
			if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(contents))}); err != nil {
				t.Fatal(err)
			}
			if _, err := archive.Write(contents); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}

		assertExtractedRuntime(t, packagePath, "ffmpeg", archiveTarXZ, want)
	})
}

func TestExtractRuntimeRequiresTheExactExecutable(t *testing.T) {
	packagePath := filepath.Join(t.TempDir(), "ffmpeg.zip")
	file, err := os.Create(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("nested/ffmpeg.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("wrong path")); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "ffmpeg.exe")
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractRuntime(packagePath, destination, "ffmpeg.exe", archiveZip); err == nil {
		t.Fatal("an archive without the exact runtime name was accepted")
	}
}

func assertExtractedRuntime(t *testing.T, packagePath, runtimeName string, format archiveFormat, want []byte) {
	t.Helper()
	destination := filepath.Join(t.TempDir(), runtimeName)
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractRuntime(packagePath, destination, runtimeName, format); err != nil {
		t.Fatalf("extracting runtime: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("runtime = %q, want %q", got, want)
	}
}
