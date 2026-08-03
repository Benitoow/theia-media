package profiles

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"io"

	// Registered for their decoders only. The encoder is always JPEG, so a PNG
	// bomb cannot be stored and served back as a PNG bomb.
	_ "image/gif"
	_ "image/png"
)

// maxPixels caps what will be decoded at all. A JPEG header costs a few bytes
// and can claim 60000x60000, which allocates 14 GB before any of it is drawn --
// the classic decompression bomb. MaxAvatarUpload bounds the bytes read; this
// bounds what those bytes are allowed to claim.
const maxPixels = 64 << 20

// Normalise turns whatever the viewer picked into the one shape Theia stores: a
// square JPEG of AvatarSize, upright, stripped of every original byte of
// metadata.
//
// Nothing of the source survives except pixels. That is the point: the upload
// endpoint is unauthenticated on the LAN, and re-encoding is what stops it
// becoming a way to host an arbitrary file behind an image URL. GPS coordinates
// in a holiday photo do not belong in a media server either.
func Normalise(r io.Reader) ([]byte, string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxAvatarUpload+1))
	if err != nil {
		return nil, "", ErrInvalidImage
	}
	if len(raw) > MaxAvatarUpload {
		return nil, "", ErrImageTooLarge
	}
	if len(raw) == 0 {
		return nil, "", ErrInvalidImage
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", ErrInvalidImage
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, "", ErrInvalidImage
	}
	if int64(config.Width)*int64(config.Height) > maxPixels {
		return nil, "", ErrImageTooLarge
	}

	source, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", ErrInvalidImage
	}

	// Orientation is read before anything is cropped: a portrait photo stored
	// sideways with an EXIF flag would otherwise be centre-cropped across the
	// wrong axis and lose the face it was chosen for.
	source = applyOrientation(source, jpegOrientation(raw))
	square := centreCrop(source)
	resized := resize(square, AvatarSize)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, resized, &jpeg.Options{Quality: 88}); err != nil {
		return nil, "", ErrInvalidImage
	}
	return out.Bytes(), "image/jpeg", nil
}

// centreCrop takes the largest centred square. A face is near the middle far
// more often than not, and the alternative -- letterboxing into a square -- puts
// two grey bars on a television card.
func centreCrop(src image.Image) image.Image {
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x := b.Min.X + (b.Dx()-side)/2
	y := b.Min.Y + (b.Dy()-side)/2
	rect := image.Rect(x, y, x+side, y+side)

	out := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(out, out.Bounds(), src, rect.Min, draw.Src)
	return out
}

// resize reduces a square to size using a box filter: every destination pixel
// averages the source pixels it covers.
//
// The standard library has no resampler, and pulling in golang.org/x/image for
// one downscale is a dependency for forty lines. Nearest-neighbour was the other
// forty-line option and it is visibly wrong on a photograph -- aliasing turns
// hair and fabric into noise at 512px, which is exactly the detail an avatar is.
// Averaging is only correct because this always shrinks; it is never asked to
// enlarge.
func resize(src image.Image, size int) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return out
	}
	if b.Dx() <= size {
		// Already smaller than the target: enlarging it would invent detail, so
		// the picture is centred at its own size instead.
		offset := (size - b.Dx()) / 2
		draw.Draw(out, image.Rect(offset, offset, offset+b.Dx(), offset+b.Dy()),
			src, b.Min, draw.Src)
		return out
	}

	for y := 0; y < size; y++ {
		y0 := b.Min.Y + y*b.Dy()/size
		y1 := b.Min.Y + (y+1)*b.Dy()/size
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < size; x++ {
			x0 := b.Min.X + x*b.Dx()/size
			x1 := b.Min.X + (x+1)*b.Dx()/size
			if x1 <= x0 {
				x1 = x0 + 1
			}

			var r, g, bl, count uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					sr, sg, sb, _ := src.At(sx, sy).RGBA()
					r += uint64(sr >> 8)
					g += uint64(sg >> 8)
					bl += uint64(sb >> 8)
					count++
				}
			}
			if count == 0 {
				count = 1
			}
			i := out.PixOffset(x, y)
			out.Pix[i+0] = uint8(r / count)
			out.Pix[i+1] = uint8(g / count)
			out.Pix[i+2] = uint8(bl / count)
			out.Pix[i+3] = 0xff
		}
	}
	return out
}

// applyOrientation rotates and flips according to the eight EXIF values. Only
// the transforms are implemented, not a general matrix: this is the whole set.
func applyOrientation(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	swap := orientation >= 5
	outW, outH := w, h
	if swap {
		outW, outH = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, outW, outH))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2: // mirrored
				dx, dy = w-1-x, y
			case 3: // 180
				dx, dy = w-1-x, h-1-y
			case 4: // mirrored vertically
				dx, dy = x, h-1-y
			case 5: // mirrored and rotated 270 clockwise
				dx, dy = y, x
			case 6: // rotated 90 clockwise
				dx, dy = h-1-y, x
			case 7: // mirrored and rotated 90 clockwise
				dx, dy = h-1-y, w-1-x
			case 8: // rotated 270 clockwise
				dx, dy = y, w-1-x
			}
			out.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}
