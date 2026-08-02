package remoteaccess

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func readPrivateKey(path string) (wgtypes.Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return wgtypes.Key{}, err
	}
	encoded := strings.TrimSpace(string(data))
	prefix := keyProtectionScheme + ":"
	if !strings.HasPrefix(encoded, prefix) {
		return wgtypes.Key{}, errors.New("remote access key uses an unsupported protection scheme")
	}
	protected, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, prefix))
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("decoding the remote access key: %w", err)
	}
	plain, err := unprotectKey(protected)
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("unprotecting the remote access key: %w", err)
	}
	key, err := wgtypes.ParseKey(strings.TrimSpace(string(plain)))
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("parsing the remote access key: %w", err)
	}
	return key, nil
}

func loadOrCreatePrivateKey(path string) (wgtypes.Key, error) {
	key, err := readPrivateKey(path)
	if err == nil {
		_ = os.Chmod(path, 0o600)
		return key, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return wgtypes.Key{}, err
	}

	key, err = wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("generating the remote access key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return wgtypes.Key{}, fmt.Errorf("creating the remote access key directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		// Another concurrent enable won the race. Its key is the authority; the
		// unused value above never crossed a boundary and can be forgotten.
		return readPrivateKey(path)
	}
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("creating the remote access key: %w", err)
	}
	written := false
	defer func() {
		file.Close()
		if !written {
			_ = os.Remove(path)
		}
	}()
	protected, err := protectKey([]byte(key.String()))
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("protecting the remote access key: %w", err)
	}
	encoded := keyProtectionScheme + ":" + base64.StdEncoding.EncodeToString(protected)
	if _, err := fmt.Fprintln(file, encoded); err != nil {
		return wgtypes.Key{}, fmt.Errorf("writing the remote access key: %w", err)
	}
	if err := file.Sync(); err != nil {
		return wgtypes.Key{}, fmt.Errorf("syncing the remote access key: %w", err)
	}
	if err := file.Close(); err != nil {
		return wgtypes.Key{}, fmt.Errorf("closing the remote access key: %w", err)
	}
	written = true
	return key, nil
}

func clientConfig(clientKey wgtypes.Key, address string, serverPublicKey wgtypes.Key, endpoint string) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32

[Peer]
PublicKey = %s
AllowedIPs = %s/32
Endpoint = %s
PersistentKeepalive = 25
`, clientKey.String(), address, serverPublicKey.String(), TunnelServerAddress, endpoint)
}
