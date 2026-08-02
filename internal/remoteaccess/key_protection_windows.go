//go:build windows

package remoteaccess

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const keyProtectionScheme = "dpapi-v1"

var keyEntropy = []byte("Theia remote access server key v1")

func protectKey(plain []byte) ([]byte, error) {
	return cryptData(plain, true)
}

func unprotectKey(protected []byte) ([]byte, error) {
	return cryptData(protected, false)
}

func cryptData(input []byte, protect bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("cannot protect an empty key")
	}
	in := windows.DataBlob{Size: uint32(len(input)), Data: &input[0]}
	entropy := windows.DataBlob{Size: uint32(len(keyEntropy)), Data: &keyEntropy[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, &entropy, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) //nolint:errcheck
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}
