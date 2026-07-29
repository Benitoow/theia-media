package profile

import (
	"encoding/binary"
	"image"
	"image/color"
)

// jpegOrientation reads the small EXIF field phones use instead of rotating
// their JPEG pixels. Invalid or absent metadata deliberately means normal:
// orientation is presentation data, never a reason to reject a valid photo.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}

	for offset := 2; offset+1 < len(data); {
		if data[offset] != 0xff {
			return 1
		}
		for offset < len(data) && data[offset] == 0xff {
			offset++
		}
		if offset >= len(data) {
			return 1
		}

		marker := data[offset]
		offset++
		if marker == 0xd9 || marker == 0xda {
			return 1
		}
		// TEM and restart markers carry no length or payload.
		if marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if offset+2 > len(data) {
			return 1
		}

		length := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if length < 2 || offset+length > len(data) {
			return 1
		}
		payload := data[offset+2 : offset+length]
		if marker == 0xe1 && len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
			return tiffOrientation(payload[6:])
		}
		offset += length
	}
	return 1
}

func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 1
	}

	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}

	ifdOffset := uint64(order.Uint32(data[4:8]))
	if ifdOffset+2 > uint64(len(data)) {
		return 1
	}
	count := uint64(order.Uint16(data[ifdOffset : ifdOffset+2]))
	firstEntry := ifdOffset + 2
	if firstEntry+count*12 > uint64(len(data)) {
		return 1
	}

	for i := uint64(0); i < count; i++ {
		entry := firstEntry + i*12
		if order.Uint16(data[entry:entry+2]) != 0x0112 ||
			order.Uint16(data[entry+2:entry+4]) != 3 ||
			order.Uint32(data[entry+4:entry+8]) != 1 {
			continue
		}
		value := int(order.Uint16(data[entry+8 : entry+10]))
		if value >= 1 && value <= 8 {
			return value
		}
		return 1
	}
	return 1
}

// orientedImage presents EXIF orientation as pixels without allocating a
// second full-resolution bitmap. Catmull-Rom then samples this view directly
// into the final 512 px square.
type orientedImage struct {
	source       image.Image
	orientation  int
	sourceWidth  int
	sourceHeight int
	bounds       image.Rectangle
}

func applyOrientation(source image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return source
	}
	sourceBounds := source.Bounds()
	width, height := sourceBounds.Dx(), sourceBounds.Dy()
	orientedWidth, orientedHeight := width, height
	if orientation >= 5 {
		orientedWidth, orientedHeight = height, width
	}
	return orientedImage{
		source:       source,
		orientation:  orientation,
		sourceWidth:  width,
		sourceHeight: height,
		bounds:       image.Rect(0, 0, orientedWidth, orientedHeight),
	}
}

func (o orientedImage) ColorModel() color.Model { return o.source.ColorModel() }
func (o orientedImage) Bounds() image.Rectangle { return o.bounds }

func (o orientedImage) At(x, y int) color.Color {
	if !image.Pt(x, y).In(o.bounds) {
		return color.RGBA{}
	}

	var sourceX, sourceY int
	switch o.orientation {
	case 2:
		sourceX, sourceY = o.sourceWidth-1-x, y
	case 3:
		sourceX, sourceY = o.sourceWidth-1-x, o.sourceHeight-1-y
	case 4:
		sourceX, sourceY = x, o.sourceHeight-1-y
	case 5:
		sourceX, sourceY = y, x
	case 6:
		sourceX, sourceY = y, o.sourceHeight-1-x
	case 7:
		sourceX, sourceY = o.sourceWidth-1-y, o.sourceHeight-1-x
	case 8:
		sourceX, sourceY = o.sourceWidth-1-y, x
	default:
		sourceX, sourceY = x, y
	}
	sourceBounds := o.source.Bounds()
	return o.source.At(sourceBounds.Min.X+sourceX, sourceBounds.Min.Y+sourceY)
}
