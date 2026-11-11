// Package attachments stores photos, invoices and documents privately and
// serves them only to their household.
package attachments

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/aiulian25/keeps/internal/blob"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	KindPhoto    = "photo"
	KindInvoice  = "invoice"
	KindDocument = "document"

	mediaJPEG = "image/jpeg"
	mediaPNG  = "image/png"
	mediaWebP = "image/webp"
	mediaPDF  = "application/pdf"

	sniffBytes             = 512
	FilenameMaximumRunes   = 120
	defaultFilename        = "file"
	concurrentImageDecodes = 1
)

// allowedTypes maps each accepted media type to its permitted extensions.
var allowedTypes = map[string][]string{
	mediaJPEG: {".jpg", ".jpeg"},
	mediaPNG:  {".png"},
	mediaWebP: {".webp"},
	mediaPDF:  {".pdf"},
}

var kindTypes = map[string][]string{
	KindPhoto:    {mediaJPEG, mediaPNG, mediaWebP},
	KindInvoice:  {mediaJPEG, mediaPNG, mediaWebP, mediaPDF},
	KindDocument: {mediaJPEG, mediaPNG, mediaWebP, mediaPDF},
}

var (
	ErrUnsupportedType = httpapi.NewError(http.StatusUnsupportedMediaType, "unsupported_file", "Use a JPG, PNG, WebP or PDF file.")
	ErrTypeMismatch    = httpapi.NewError(http.StatusUnsupportedMediaType, "file_type_mismatch", "This file's contents don't match its type. Export it again as JPG, PNG, WebP or PDF.")
	ErrPhotoType       = httpapi.NewError(http.StatusUnsupportedMediaType, "unsupported_photo", "Photos must be JPG, PNG or WebP.")
	ErrTooLarge        = httpapi.NewError(http.StatusRequestEntityTooLarge, "file_too_large", fmt.Sprintf("Files can be up to %d MB.", config.AttachmentMaximumBytes>>20))
	ErrImageTooLarge   = httpapi.NewError(http.StatusRequestEntityTooLarge, "image_too_large", "This image has too many pixels. Resize it below 25 megapixels.")
	ErrUnreadableImage = httpapi.NewError(http.StatusUnsupportedMediaType, "unreadable_image", "This image couldn't be read. Try exporting it again.")
	ErrStorageFull     = httpapi.NewError(http.StatusInsufficientStorage, "storage_full", "The server is out of storage space. Free some space and try again. Your other details are kept.")
	ErrNoFile          = httpapi.NewError(http.StatusBadRequest, "no_file", "Choose a file to upload.")
	ErrUnknownKind     = httpapi.NewError(http.StatusBadRequest, "invalid_kind", "Choose photo, invoice or document.")
)

type Attachment struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Filename   string `json:"filename"`
	MediaType  string `json:"mediaType"`
	SizeBytes  int64  `json:"sizeBytes"`
	HasPreview bool   `json:"hasPreview"`
	CreatedAt  string `json:"createdAt"`
}

type Service struct {
	store       *store.Store
	blobs       *blob.Store
	dataPath    string
	imageSlots  chan struct{}
	freeSpaceOf func(path string) (uint64, error)
}

func NewService(database *store.Store, blobs *blob.Store, dataPath string) *Service {
	return &Service{store: database, blobs: blobs, dataPath: dataPath, imageSlots: make(chan struct{}, concurrentImageDecodes), freeSpaceOf: freeSpace}
}

func freeSpace(path string) (uint64, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return 0, err
	}
	return stats.Bavail * uint64(stats.Bsize), nil
}

// CleanFilename keeps only a safe display name; it is never used as a path.
func CleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) || strings.ContainsRune(`/\:*?"<>|`, character) {
			return -1
		}
		return character
	}, name)
	name = strings.TrimSpace(strings.Trim(name, "."))
	runes := []rune(name)
	if len(runes) > FilenameMaximumRunes {
		extension := []rune(filepath.Ext(name))
		// An extension that leaves no room for the name is cut with the rest.
		if len(extension) >= FilenameMaximumRunes {
			extension = nil
		}
		name = string(runes[:FilenameMaximumRunes-len(extension)]) + string(extension)
	}
	if name == "" {
		return defaultFilename
	}
	return name
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// detectType requires the extension, the declared type and the sniffed
// content to agree. None of them alone is trusted.
func detectType(filename string, declared string, head []byte, kind string) (string, error) {
	sniffed, _, _ := mime.ParseMediaType(http.DetectContentType(head))
	extensions, supported := allowedTypes[sniffed]
	if !supported {
		return "", ErrUnsupportedType
	}
	extension := strings.ToLower(filepath.Ext(filename))
	declaredType, _, _ := mime.ParseMediaType(declared)
	declaredMatches := declaredType == "" || declaredType == "application/octet-stream" || declaredType == sniffed || (declaredType == "image/jpg" && sniffed == mediaJPEG)
	if !contains(extensions, extension) || !declaredMatches {
		return "", ErrTypeMismatch
	}
	if !contains(kindTypes[kind], sniffed) {
		return "", ErrPhotoType
	}
	return sniffed, nil
}

// Upload streams one multipart file into private storage.
func (s *Service) Upload(ctx context.Context, householdID int64, kind string, request *http.Request) (Attachment, error) {
	if _, known := kindTypes[kind]; !known {
		return Attachment{}, ErrUnknownKind
	}
	if available, err := s.freeSpaceOf(s.dataPath); err == nil && available < config.MinimumFreeDiskBytes {
		return Attachment{}, ErrStorageFull
	}
	reader, err := request.MultipartReader()
	if err != nil {
		return Attachment{}, ErrNoFile
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return Attachment{}, ErrNoFile
		}
		if err != nil {
			return Attachment{}, ErrNoFile
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		defer part.Close()
		return s.storeUpload(ctx, householdID, kind, CleanFilename(part.FileName()), part.Header.Get("Content-Type"), part)
	}
}

func (s *Service) storeUpload(ctx context.Context, householdID int64, kind string, filename string, declared string, source io.Reader) (Attachment, error) {
	head := make([]byte, sniffBytes)
	read, err := io.ReadFull(source, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Attachment{}, ErrNoFile
	}
	head = head[:read]
	mediaType, err := detectType(filename, declared, head, kind)
	if err != nil {
		return Attachment{}, err
	}
	return s.Save(ctx, householdID, kind, filename, mediaType, io.MultiReader(bytes.NewReader(head), source))
}

// Save stores already-typed content (also used by the importer for
// downloaded product photos) and links nothing yet.
func (s *Service) Save(ctx context.Context, householdID int64, kind string, filename string, mediaType string, content io.Reader) (Attachment, error) {
	written, err := s.blobs.Write(content, config.AttachmentMaximumBytes)
	if errors.Is(err, blob.ErrTooLarge) {
		return Attachment{}, ErrTooLarge
	}
	if errors.Is(err, blob.ErrStorageFull) {
		return Attachment{}, ErrStorageFull
	}
	if err != nil {
		return Attachment{}, err
	}
	keys := []string{written.Key}
	cleanup := func() {
		for _, key := range keys {
			s.blobs.Remove(key)
		}
	}

	var previewKey, thumbnailKey sql.NullString
	if mediaType != mediaPDF {
		preview, thumbnail, err := s.makePreviews(written.Key)
		if err != nil {
			cleanup()
			return Attachment{}, err
		}
		keys = append(keys, preview, thumbnail)
		previewKey = sql.NullString{String: preview, Valid: true}
		thumbnailKey = sql.NullString{String: thumbnail, Valid: true}
	}

	now := store.Now()
	result, err := s.store.Database.ExecContext(ctx, `INSERT INTO attachments (household_id, kind, storage_key, preview_key, thumbnail_key, filename, media_type, size_bytes, sha256, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, householdID, kind, written.Key, previewKey, thumbnailKey, filename, mediaType, written.Size, written.SHA256, now)
	if err != nil {
		cleanup()
		return Attachment{}, err
	}
	identifier, _ := result.LastInsertId()
	return Attachment{ID: identifier, Kind: kind, Filename: filename, MediaType: mediaType, SizeBytes: written.Size, HasPreview: previewKey.Valid, CreatedAt: now}, nil
}

// makePreviews checks dimensions before decoding (so a decompression bomb is
// rejected cheaply) and re-encodes previews upright, which drops EXIF and
// location metadata. The original is kept untouched as evidence.
func (s *Service) makePreviews(key string) (string, string, error) {
	s.imageSlots <- struct{}{}
	defer func() { <-s.imageSlots }()

	file, err := s.blobs.Open(key)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	configuration, _, err := image.DecodeConfig(file)
	if err != nil {
		return "", "", ErrUnreadableImage
	}
	if configuration.Width <= 0 || configuration.Height <= 0 || configuration.Width*configuration.Height > config.ImageMaximumPixels {
		return "", "", ErrImageTooLarge
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	orientation := readOrientation(file)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	decoded, _, err := image.Decode(file)
	if err != nil {
		return "", "", ErrUnreadableImage
	}
	preview, err := s.writeScaled(decoded, config.PreviewMaximumEdge, orientation)
	if err != nil {
		return "", "", err
	}
	thumbnail, err := s.writeScaled(decoded, config.ThumbnailMaximumEdge, orientation)
	if err != nil {
		s.blobs.Remove(preview)
		return "", "", err
	}
	return preview, thumbnail, nil
}

// writeScaled scales, then applies the EXIF orientation, and re-encodes
// without metadata.
func (s *Service) writeScaled(source image.Image, maximumEdge int, orientation int) (string, error) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scale := min(1, float64(maximumEdge)/float64(max(width, height)))
	targetWidth, targetHeight := max(1, int(float64(width)*scale)), max(1, int(float64(height)*scale))

	canvas := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(canvas, canvas.Bounds(), source, bounds, draw.Over, nil)

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, orient(canvas, orientation), &jpeg.Options{Quality: config.PreviewJPEGQuality}); err != nil {
		return "", err
	}
	written, err := s.blobs.Write(&encoded, config.AttachmentMaximumBytes)
	if errors.Is(err, blob.ErrStorageFull) {
		return "", ErrStorageFull
	}
	return written.Key, err
}

type storedAttachment struct {
	Attachment
	storageKey   string
	previewKey   string
	thumbnailKey string
}

// load returns the attachment only if it belongs to the household.
func (s *Service) load(ctx context.Context, householdID int64, attachmentID int64) (storedAttachment, error) {
	var stored storedAttachment
	var preview, thumbnail sql.NullString
	err := s.store.Database.QueryRowContext(ctx, `SELECT id, kind, filename, media_type, size_bytes, created_at, storage_key, preview_key, thumbnail_key
		FROM attachments WHERE id = ? AND household_id = ?`, attachmentID, householdID).
		Scan(&stored.ID, &stored.Kind, &stored.Filename, &stored.MediaType, &stored.SizeBytes, &stored.CreatedAt, &stored.storageKey, &preview, &thumbnail)
	if errors.Is(err, sql.ErrNoRows) {
		return storedAttachment{}, httpapi.ErrNotFound
	}
	stored.previewKey, stored.thumbnailKey = preview.String, thumbnail.String
	stored.HasPreview = preview.Valid
	return stored, err
}

func (s *Service) BlobReferenced(ctx context.Context, key string) (bool, error) {
	var found int
	err := s.store.Database.QueryRowContext(ctx, `SELECT 1 FROM attachments WHERE storage_key = ? OR preview_key = ? OR thumbnail_key = ? LIMIT 1`, key, key, key).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
