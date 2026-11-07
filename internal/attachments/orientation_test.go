package attachments

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"path/filepath"
	"testing"

	"github.com/aiulian25/keeps/internal/blob"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	fixtureWidth  = 64
	fixtureHeight = 32
)

// orientedJPEG is a 64×32 JPEG whose top-left quarter is red and the rest
// blue, carrying the given EXIF orientation (0 for none).
func orientedJPEG(t *testing.T, orientation uint16, exifPayload []byte) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, fixtureWidth, fixtureHeight))
	for y := range fixtureHeight {
		for x := range fixtureWidth {
			pixel := color.RGBA{B: 220, A: 255}
			if x < fixtureWidth/2 && y < fixtureHeight/2 {
				pixel = color.RGBA{R: 220, A: 255}
			}
			picture.SetRGBA(x, y, pixel)
		}
	}
	var encoded bytes.Buffer
	jpeg.Encode(&encoded, picture, &jpeg.Options{Quality: 95})
	plain := encoded.Bytes()
	if orientation == 0 && exifPayload == nil {
		return plain
	}
	if exifPayload == nil {
		var tiff bytes.Buffer
		tiff.WriteString("II")
		binary.Write(&tiff, binary.LittleEndian, uint16(42))
		binary.Write(&tiff, binary.LittleEndian, uint32(8))
		binary.Write(&tiff, binary.LittleEndian, uint16(1))
		binary.Write(&tiff, binary.LittleEndian, []uint16{orientationTag, tiffShortType})
		binary.Write(&tiff, binary.LittleEndian, uint32(1))
		binary.Write(&tiff, binary.LittleEndian, []uint16{orientation, 0})
		binary.Write(&tiff, binary.LittleEndian, uint32(0))
		exifPayload = append([]byte(exifHeader), tiff.Bytes()...)
	}
	segment := []byte{jpegMarkerPrefix, jpegApplication1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(exifPayload)+segmentLengthBytes))
	withExif := append([]byte{}, plain[:2]...)
	withExif = append(withExif, segment...)
	withExif = append(withExif, exifPayload...)
	return append(withExif, plain[2:]...)
}

func isRed(pixel color.Color) bool {
	red, green, blue, _ := pixel.RGBA()
	return red > blue+0x4000 && red > green+0x4000
}

func TestPreviewsAreUprightForEveryOrientation(t *testing.T) {
	directory := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(directory, "keeps.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.Database.Exec(`INSERT INTO households (id, name, created_at) VALUES (1, 'Home', 'now')`)
	blobs, _ := blob.New(filepath.Join(directory, "blobs"), filepath.Join(directory, "tmp"))
	service := NewService(database, blobs, directory)

	cases := map[string]struct {
		orientation uint16
		payload     []byte
		upright     int
	}{
		"untagged":         {0, nil, orientationUpright},
		"out of range":     {9, nil, orientationUpright},
		"truncated":        {0, []byte(exifHeader + "II*"), orientationUpright},
		"wrong byte order": {0, []byte(exifHeader + "XX*\x00\x08\x00\x00\x00"), orientationUpright},
	}
	for orientation := uint16(1); orientation <= orientationMaximum; orientation++ {
		cases[string(rune('0'+orientation))] = struct {
			orientation uint16
			payload     []byte
			upright     int
		}{orientation, nil, int(orientation)}
	}
	for name, testCase := range cases {
		original := orientedJPEG(t, testCase.orientation, testCase.payload)
		saved, err := service.Save(context.Background(), 1, KindPhoto, "photo.jpg", "image/jpeg", bytes.NewReader(original))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var originalKey, previewKey string
		database.Database.QueryRow(`SELECT storage_key, preview_key FROM attachments WHERE id = ?`, saved.ID).Scan(&originalKey, &previewKey)
		stored, _ := blobs.Open(originalKey)
		storedBytes, _ := io.ReadAll(stored)
		stored.Close()
		if !bytes.Equal(storedBytes, original) {
			t.Fatalf("%s: the original was changed", name)
		}
		previewFile, _ := blobs.Open(previewKey)
		preview, err := jpeg.Decode(previewFile)
		previewFile.Close()
		if err != nil {
			t.Fatalf("%s: preview: %v", name, err)
		}
		redX, redY := orientedPosition(testCase.upright, fixtureWidth/4, fixtureHeight/4, fixtureWidth, fixtureHeight)
		blueX, blueY := orientedPosition(testCase.upright, fixtureWidth*3/4, fixtureHeight*3/4, fixtureWidth, fixtureHeight)
		if !isRed(preview.At(redX, redY)) || isRed(preview.At(blueX, blueY)) {
			t.Errorf("%s: preview %v is not oriented as EXIF %d says", name, preview.Bounds().Size(), testCase.upright)
		}
		wantWide := testCase.upright < orientationTransposed
		if (preview.Bounds().Dx() > preview.Bounds().Dy()) != wantWide {
			t.Errorf("%s: preview shape %v", name, preview.Bounds().Size())
		}
	}
}

// Hand-worked expectations, independent of orientedPosition: where the red
// corner of the stored picture must end up once upright.
func TestCommonOrientationsByHand(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, fixtureWidth, fixtureHeight))
	canvas.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	cases := map[int]image.Point{
		orientationMirrored:             {fixtureWidth - 1, 0},
		orientationRotated180:           {fixtureWidth - 1, fixtureHeight - 1},
		orientationRotatedClockwise:     {fixtureHeight - 1, 0},
		orientationRotatedAnticlockwise: {0, fixtureWidth - 1},
	}
	for orientation, want := range cases {
		upright := orient(canvas, orientation)
		if !isRed(upright.At(want.X, want.Y)) {
			t.Errorf("orientation %d: the stored top-left corner is not at %v", orientation, want)
		}
	}
}
