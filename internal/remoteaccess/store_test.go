package remoteaccess

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func testStore(t *testing.T) (*store, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(t.Context(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return newStore(database), dir
}

func TestRemoteMigrationIsIdempotentAndDefaultsDisabled(t *testing.T) {
	store, _ := testStore(t)
	if err := db.Migrate(t.Context(), store.db); err != nil {
		t.Fatalf("second migration pass: %v", err)
	}
	cfg, err := store.config(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || cfg.ListenPort != DefaultListenPort || cfg.Endpoint != "" {
		t.Fatalf("default config = %#v", cfg)
	}
}

func TestPeerLimitAndRevokedAddressReuse(t *testing.T) {
	store, _ := testStore(t)
	var first Peer
	for i := 0; i < MaxPeers; i++ {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		peer, err := store.createPeer(t.Context(), "Device", key.PublicKey().String())
		if err != nil {
			t.Fatalf("creating peer %d: %v", i, err)
		}
		if i == 0 {
			first = peer
		}
	}
	extra, _ := wgtypes.GeneratePrivateKey()
	if _, err := store.createPeer(t.Context(), "Too many", extra.PublicKey().String()); !errors.Is(err, ErrPeerLimit) {
		t.Fatalf("peer 33 = %v, want ErrPeerLimit", err)
	}
	if err := store.revokePeer(t.Context(), first.ID); err != nil {
		t.Fatal(err)
	}
	replacement, _ := wgtypes.GeneratePrivateKey()
	peer, err := store.createPeer(t.Context(), "Replacement", replacement.PublicKey().String())
	if err != nil {
		t.Fatal(err)
	}
	if peer.Address != first.Address {
		t.Fatalf("replacement address = %s, want reusable %s", peer.Address, first.Address)
	}
	if err := store.revokePeer(t.Context(), first.ID); !errors.Is(err, ErrPeerNotFound) {
		t.Fatalf("second revoke = %v, want ErrPeerNotFound", err)
	}
}

func TestPrivateKeyFileIsStableAndRejectsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), keyFileName)
	first, err := loadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("server private key changed between reads")
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(onDisk, []byte(first.String())) {
		t.Fatal("server private key is stored as readable WireGuard text")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("key permissions = %o, want no group/world access", info.Mode().Perm())
	}
	if err := os.WriteFile(path, []byte("not-a-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreatePrivateKey(path); err == nil {
		t.Fatal("corrupt key was silently replaced")
	}
}
