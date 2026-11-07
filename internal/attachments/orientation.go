package attachments

import (
	"bytes"
	"encoding/binary"
	"image"
	"io"
)

// EXIF orientation values.
const (
	orientationMirrored             = 2
	orientationRotated180           = 3
	orientationFlipped              = 4
	orientationTransposed           = 5
	orientationRotatedClockwise     = 6
	orientationTransversed          = 7
	orientationRotatedAnticlockwise = 8
)

const (
	// exifSearchBytes bounds how much of a file is read for its orientation;
	// the EXIF block sits at the start of a JPEG.
	exifSearchBytes       = 64 << 10
	jpegMarkerPrefix      = 0xFF
	jpegStartOfImage      = 0xD8
	jpegApplication1      = 0xE1
	jpegStartOfScan       = 0xDA
	orientationTag        = 0x0112
	tiffShortType         = 3
	ifdEntryBytes         = 12
	orientationUpright    = 1
	orientationMaximum    = orientationRotatedAnticlockwise
	segmentLengthBytes    = 2
	tiffHeaderBytes       = 8
	tiffOffsetPosition    = 4
	ifdCountBytes         = 2
	entryTypePosition     = 2
	entryValuePosition    = 8
	exifHeader            = "Exif\x00\x00"
	littleEndianSignature = "II"
	bigEndianSignature    = "MM"
)

// readOrientation returns the EXIF orientation (1–8) of a JPEG, or 1 when
// there is none or it can't be read. Only the first exifSearchBytes are read.
func readOrientation(source io.Reader) int {
	head := make([]byte, exifSearchBytes)
	read, _ := io.ReadFull(source, head)
	data := head[:read]
	if len(data) < 2 || data[0] != jpegMarkerPrefix || data[1] != jpegStartOfImage {
		return orientationUpright
	}
	position := 2
	for position+4 <= len(data) {
		if data[position] != jpegMarkerPrefix {
			return orientationUpright
		}
		marker := data[position+1]
		length := int(binary.BigEndian.Uint16(data[position+2:]))
		segmentStart := position + 2 + segmentLengthBytes
		segmentEnd := position + 2 + length
		if marker == jpegStartOfScan || length < segmentLengthBytes || segmentEnd > len(data) {
			return orientationUpright
		}
		if marker == jpegApplication1 && bytes.HasPrefix(data[segmentStart:segmentEnd], []byte(exifHeader)) {
			return tiffOrientation(data[segmentStart+len(exifHeader) : segmentEnd])
		}
		position = segmentEnd
	}
	return orientationUpright
}

func tiffOrientation(tiff []byte) int {
	if len(tiff) < tiffHeaderBytes {
		return orientationUpright
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case littleEndianSignature:
		order = binary.LittleEndian
	case bigEndianSignature:
		order = binary.BigEndian
	default:
		return orientationUpright
	}
	ifd := int(order.Uint32(tiff[tiffOffsetPosition:]))
	if ifd < tiffHeaderBytes || ifd+ifdCountBytes > len(tiff) {
		return orientationUpright
	}
	count := int(order.Uint16(tiff[ifd:]))
	for index := range count {
		entry := ifd + ifdCountBytes + index*ifdEntryBytes
		if entry+ifdEntryBytes > len(tiff) {
			return orientationUpright
		}
		if order.Uint16(tiff[entry:]) != orientationTag || order.Uint16(tiff[entry+entryTypePosition:]) != tiffShortType {
			continue
		}
		value := int(order.Uint16(tiff[entry+entryValuePosition:]))
		if value < orientationUpright || value > orientationMaximum {
			return orientationUpright
		}
		return value
	}
	return orientationUpright
}

// orient turns pixels the way EXIF orientation says the camera meant them.
// It runs on the scaled preview, so it never copies a full-size photo.
func orient(source *image.RGBA, orientation int) *image.RGBA {
	if orientation <= orientationUpright || orientation > orientationMaximum {
		return source
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	swapped := orientation >= orientationTransposed
	targetWidth, targetHeight := width, height
	if swapped {
		targetWidth, targetHeight = height, width
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := range height {
		for x := range width {
			targetX, targetY := orientedPosition(orientation, x, y, width, height)
			target.SetRGBA(targetX, targetY, source.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return target
}

// orientedPosition maps a stored pixel to where it belongs for each EXIF
// orientation: 2 mirror, 3 rotate 180, 4 flip, 5 transpose, 6 rotate 90°
// clockwise, 7 transverse, 8 rotate 90° anticlockwise.
func orientedPosition(orientation int, x int, y int, width int, height int) (int, int) {
	switch orientation {
	case orientationMirrored:
		return width - 1 - x, y
	case orientationRotated180:
		return width - 1 - x, height - 1 - y
	case orientationFlipped:
		return x, height - 1 - y
	case orientationTransposed:
		return y, x
	case orientationRotatedClockwise:
		return height - 1 - y, x
	case orientationTransversed:
		return height - 1 - y, width - 1 - x
	case orientationRotatedAnticlockwise:
		return y, width - 1 - x
	}
	return x, y
}
