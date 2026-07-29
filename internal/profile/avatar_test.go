package profile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestProcessAvatarCropsAndReencodesToJPEG(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 96, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 96; x++ {
			source.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 4), B: 80, A: 255})
		}
	}

	var input bytes.Buffer
	if err := png.Encode(&input, source); err != nil {
		t.Fatal(err)
	}
	processed, err := ProcessAvatar(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	got, format, err := image.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" {
		t.Errorf("format = %q, want jpeg", format)
	}
	if got.Bounds().Dx() != AvatarSize || got.Bounds().Dy() != AvatarSize {
		t.Errorf("bounds = %v, want %dx%d", got.Bounds(), AvatarSize, AvatarSize)
	}
}

func TestProcessAvatarAppliesJPEGEXIFOrientation(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 120, 60))
	for y := range 60 {
		for x := range 120 {
			if x < 60 {
				source.Set(x, y, color.RGBA{R: 240, G: 20, B: 20, A: 255})
			} else {
				source.Set(x, y, color.RGBA{R: 20, G: 20, B: 240, A: 255})
			}
		}
	}

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	input := jpegWithOrientation(t, encoded.Bytes(), 6)

	processed, err := ProcessAvatar(input)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := image.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatal(err)
	}

	top := color.RGBAModel.Convert(got.At(AvatarSize/2, AvatarSize/8)).(color.RGBA)
	bottom := color.RGBAModel.Convert(got.At(AvatarSize/2, AvatarSize*7/8)).(color.RGBA)
	if int(top.R) <= int(top.B)*2 || int(bottom.B) <= int(bottom.R)*2 {
		t.Errorf("orientation 6 colors = top %#v bottom %#v, want red above blue", top, bottom)
	}
}

func jpegWithOrientation(t *testing.T, input []byte, orientation uint16) []byte {
	t.Helper()
	if len(input) < 2 || input[0] != 0xff || input[1] != 0xd8 {
		t.Fatal("test input is not a JPEG")
	}

	tiff := make([]byte, 26)
	copy(tiff[:2], "MM")
	binary.BigEndian.PutUint16(tiff[2:4], 42)
	binary.BigEndian.PutUint32(tiff[4:8], 8)
	binary.BigEndian.PutUint16(tiff[8:10], 1)
	binary.BigEndian.PutUint16(tiff[10:12], 0x0112)
	binary.BigEndian.PutUint16(tiff[12:14], 3)
	binary.BigEndian.PutUint32(tiff[14:18], 1)
	binary.BigEndian.PutUint16(tiff[18:20], orientation)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	length := len(payload) + 2
	out := make([]byte, 0, len(input)+len(payload)+4)
	out = append(out, input[:2]...)
	out = append(out, 0xff, 0xe1, byte(length>>8), byte(length))
	out = append(out, payload...)
	out = append(out, input[2:]...)
	return out
}

func TestProcessAvatarRejectsInvalidAndAnimatedFormats(t *testing.T) {
	if _, err := ProcessAvatar([]byte("not an image")); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("invalid bytes error = %v, want ErrInvalidAvatar", err)
	}

	var animated bytes.Buffer
	if err := gif.Encode(&animated, image.NewRGBA(image.Rect(0, 0, 8, 8)),
		&gif.Options{NumColors: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessAvatar(animated.Bytes()); !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("GIF error = %v, want ErrInvalidAvatar", err)
	}
}

func TestProcessedAvatarContainsNoSourceTrailer(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, nil); err != nil {
		t.Fatal(err)
	}
	input.WriteString("<script>not image data</script>")

	processed, err := ProcessAvatar(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(processed, []byte("<script>")) {
		t.Error("the source trailer survived avatar re-encoding")
	}
}
