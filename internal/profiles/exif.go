package profiles

import "encoding/binary"

// jpegOrientation returns the EXIF orientation of a JPEG, or 0 when there is
// none to find.
//
// Only one tag is read. A full EXIF parser would be a dependency and a much
// larger attack surface for a single number, and every other field in there is
// something Theia deliberately throws away (decision 48). Anything malformed
// returns 0, which means "leave the picture alone" -- a photo shown sideways is
// a poor result, whereas trusting a hostile length field is a worse one.
func jpegOrientation(raw []byte) int {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return 0 // not a JPEG; PNG and GIF carry no orientation
	}

	for i := 2; i+4 <= len(raw); {
		if raw[i] != 0xFF {
			return 0 // out of step with the marker structure
		}
		marker := raw[i+1]
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		if marker == 0xDA || marker == 0xD9 {
			return 0 // image data begins; EXIF would have appeared before it
		}
		if i+4 > len(raw) {
			return 0
		}
		length := int(binary.BigEndian.Uint16(raw[i+2 : i+4]))
		if length < 2 || i+2+length > len(raw) {
			return 0
		}
		if marker == 0xE1 {
			if o := exifOrientation(raw[i+4 : i+2+length]); o != 0 {
				return o
			}
		}
		i += 2 + length
	}
	return 0
}

// exifOrientation walks the TIFF header inside an APP1 segment to IFD0 and
// reads tag 0x0112.
func exifOrientation(segment []byte) int {
	const header = "Exif\x00\x00"
	if len(segment) < len(header)+8 || string(segment[:len(header)]) != header {
		return 0
	}
	tiff := segment[len(header):]

	var order binary.ByteOrder
	switch {
	case tiff[0] == 'I' && tiff[1] == 'I':
		order = binary.LittleEndian
	case tiff[0] == 'M' && tiff[1] == 'M':
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0
	}

	offset := int(order.Uint32(tiff[4:8]))
	if offset < 8 || offset+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[offset : offset+2]))
	entry := offset + 2

	for n := 0; n < count; n++ {
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:entry+2]) == 0x0112 {
			// A SHORT value lives in the first two bytes of the value field.
			if order.Uint16(tiff[entry+2:entry+4]) != 3 {
				return 0
			}
			value := int(order.Uint16(tiff[entry+8 : entry+10]))
			if value >= 1 && value <= 8 {
				return value
			}
			return 0
		}
		entry += 12
	}
	return 0
}
