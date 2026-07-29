package profile

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG with image.Decode

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP with image.Decode
)

const (
	AvatarSize       = 512
	MaximumPixels    = 24_000_000
	MaximumDimension = 8192
)

var ErrInvalidAvatar = errors.New("invalid avatar image")

// ProcessAvatar verifies, crops and re-encodes a locally uploaded profile
// picture. Re-encoding strips metadata and makes the bytes we later serve an
// image rather than whatever happened to be appended to the upload.
func ProcessAvatar(data []byte) ([]byte, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !acceptedAvatarFormat(format) ||
		config.Width <= 0 || config.Height <= 0 ||
		config.Width > MaximumDimension || config.Height > MaximumDimension ||
		int64(config.Width)*int64(config.Height) > MaximumPixels {
		return nil, ErrInvalidAvatar
	}

	source, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || !acceptedAvatarFormat(decodedFormat) {
		return nil, ErrInvalidAvatar
	}
	if decodedFormat == "jpeg" {
		source = applyOrientation(source, jpegOrientation(data))
	}

	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	left := bounds.Min.X + (bounds.Dx()-side)/2
	top := bounds.Min.Y + (bounds.Dy()-side)/2
	crop := image.Rect(left, top, left+side, top+side)

	target := image.NewRGBA(image.Rect(0, 0, AvatarSize, AvatarSize))
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, crop, xdraw.Over, nil)

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, target, &jpeg.Options{Quality: 86}); err != nil {
		return nil, ErrInvalidAvatar
	}
	return encoded.Bytes(), nil
}

func acceptedAvatarFormat(format string) bool {
	switch format {
	case "jpeg", "png", "webp":
		return true
	default:
		return false
	}
}
