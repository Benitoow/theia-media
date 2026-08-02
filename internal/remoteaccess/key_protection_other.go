//go:build !windows

package remoteaccess

const keyProtectionScheme = "file-v1"

func protectKey(plain []byte) ([]byte, error) {
	return append([]byte(nil), plain...), nil
}

func unprotectKey(protected []byte) ([]byte, error) {
	return append([]byte(nil), protected...), nil
}
