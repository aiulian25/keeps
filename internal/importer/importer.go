// Package importer turns a public product link into an editable draft. It
// never saves an item by itself: the user reviews the draft first.
package importer

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aiulian25/keeps/internal/attachments"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/importer/extract"
	"github.com/aiulian25/keeps/internal/items"
	"github.com/aiulian25/keeps/internal/money"
	"github.com/aiulian25/keeps/internal/safefetch"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	StateQueued    = "queued"
	StateRunning   = "running"
	StateDone      = "done"
	StateFailed    = "failed"
	StateCancelled = "cancelled"
	StateSaved     = "saved"

	codeNoDetails  = "no_details"
	codeTimeout    = string(safefetch.CodeTimeout)
	pageAccept     = "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5"
	imageAccept    = "image/jpeg,image/png,image/webp;q=0.9"
	identifierSize = 16
	imageFilename  = "product-photo"
)

// Fetcher is the safe outbound fetcher (an interface so tests can serve
// fixtures without network access).
type Fetcher interface {
	Get(ctx context.Context, rawURL string, limits safefetch.Limits, acceptHeader string) (*safefetch.Response, error)
}

type Service struct {
	store       *store.Store
	fetcher     Fetcher
	attachments *attachments.Service
	items       *items.Service
	wake        chan struct{}
	mutex       sync.Mutex
	cancels     map[string]context.CancelFunc
}

func NewService(database *store.Store, fetcher Fetcher, attachmentService *attachments.Service, itemService *items.Service) *Service {
	return &Service{store: database, fetcher: fetcher, attachments: attachmentService, items: itemService,
		wake: make(chan struct{}, 1), cancels: map[string]context.CancelFunc{}}
}

type Candidate struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Brand        string       `json:"brand"`
	Model        string       `json:"model"`
	SKU          string       `json:"sku"`
	ListingPrice *items.Money `json:"listingPrice"`
	Availability string       `json:"availability"`
	Seller       string       `json:"seller"`
	ImageIDs     []int64      `json:"imageIds"`
}

type Result struct {
	Candidates []Candidate       `json:"candidates"`
	PageTitle  string            `json:"pageTitle"`
	Partial    bool              `json:"partial"`
	Sources    map[string]string `json:"sources"`
}

type Draft struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	ErrorCode   string `json:"errorCode,omitempty"`
	SourceURL   string `json:"sourceUrl"`
	SourceHost  string `json:"sourceHost"`
	RetrievedAt string `json:"retrievedAt,omitempty"`
	Result
}

func newIdentifier() (string, error) {
	random := make([]byte, identifierSize)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return hex.EncodeToString(random), nil
}

func fetchErrorCode(err error) string {
	var fetchError *safefetch.Error
	if errors.As(err, &fetchError) {
		return string(fetchError.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return codeTimeout
	}
	return string(safefetch.CodeFetchFailed)
}

// Start validates the link immediately (so obviously unusable links fail
// without a job) and queues the import.
func (s *Service) Start(ctx context.Context, householdID int64, userID int64, rawURL string) (Draft, error) {
	normalised, err := safefetch.NormaliseURL(rawURL)
	if err != nil {
		code := fetchErrorCode(err)
		return Draft{}, &httpapi.Error{Status: http.StatusBadRequest, Code: code, Message: "This link can't be used.", Fields: map[string]string{"url": code}}
	}
	identifier, err := newIdentifier()
	if err != nil {
		return Draft{}, err
	}
	now := time.Now()
	_, err = s.store.Database.ExecContext(ctx, `INSERT INTO imports (id, household_id, user_id, source_url, state, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		identifier, householdID, userID, normalised.String(), StateQueued, store.FormatTime(now.Add(config.ImportDraftLifetime)), store.FormatTime(now), store.FormatTime(now))
	if err != nil {
		return Draft{}, err
	}
	s.signal()
	return Draft{ID: identifier, State: StateQueued, SourceURL: normalised.String(), SourceHost: normalised.Hostname()}, nil
}

func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) Get(ctx context.Context, householdID int64, identifier string) (Draft, error) {
	var draft Draft
	var resultJSON string
	var retrieved sql.NullString
	err := s.store.Database.QueryRowContext(ctx, `SELECT id, state, error_code, source_url, result_json, retrieved_at FROM imports WHERE id = ? AND household_id = ?`,
		identifier, householdID).Scan(&draft.ID, &draft.State, &draft.ErrorCode, &draft.SourceURL, &resultJSON, &retrieved)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, httpapi.ErrNotFound
	}
	if err != nil {
		return Draft{}, err
	}
	draft.RetrievedAt = retrieved.String
	if parsed, parseErr := url.Parse(draft.SourceURL); parseErr == nil {
		draft.SourceHost = parsed.Hostname()
	}
	if resultJSON != "" {
		json.Unmarshal([]byte(resultJSON), &draft.Result)
	}
	if draft.Candidates == nil {
		draft.Candidates = []Candidate{}
	}
	for index := range draft.Candidates {
		draft.Candidates[index].ImageIDs = savedImages(draft.Candidates[index].ImageIDs)
	}
	return draft, nil
}

// savedImages drops the 0 IDs that drafts made by earlier versions could hold
// for images that failed to download.
func savedImages(identifiers []int64) []int64 {
	saved := []int64{}
	for _, identifier := range identifiers {
		if identifier != 0 {
			saved = append(saved, identifier)
		}
	}
	return saved
}

func (s *Service) Cancel(ctx context.Context, householdID int64, identifier string) error {
	result, err := s.store.Database.ExecContext(ctx, `UPDATE imports SET state = ?, updated_at = ? WHERE id = ? AND household_id = ? AND state IN (?, ?)`,
		StateCancelled, store.Now(), identifier, householdID, StateQueued, StateRunning)
	if err != nil {
		return err
	}
	s.mutex.Lock()
	if cancel, running := s.cancels[identifier]; running {
		cancel()
	}
	s.mutex.Unlock()
	if changed, _ := result.RowsAffected(); changed == 0 {
		_, err := s.Get(ctx, householdID, identifier)
		return err
	}
	return nil
}

// Save creates the item from the reviewed values. Imported values are kept
// as listing data; the price paid and purchase date come only from the user.
func (s *Service) Save(ctx context.Context, householdID int64, userID int64, identifier string, candidateIndex int, input items.Input, idempotencyKey string) (items.Item, error) {
	draft, err := s.Get(ctx, householdID, identifier)
	if err != nil {
		return items.Item{}, err
	}
	origin := &items.ImportOrigin{SourceURL: draft.SourceURL, SourceHost: draft.SourceHost, RetrievedAt: draft.RetrievedAt, ExtractorVersion: extract.Version, ImportedValues: map[string]string{}}
	hasCandidate := candidateIndex >= 0 && candidateIndex < len(draft.Candidates)
	if hasCandidate {
		candidate := draft.Candidates[candidateIndex]
		origin.Title = candidate.Name
		origin.ImportedValues = map[string]string{"name": candidate.Name, "description": candidate.Description, "brand": candidate.Brand, "model": candidate.Model}
		if candidate.ListingPrice != nil {
			minor, parseErr := money.ParseDecimal(candidate.ListingPrice.Amount, candidate.ListingPrice.Currency)
			if parseErr == nil {
				origin.ListingPrice = &minor
				origin.ListingCurrency = candidate.ListingPrice.Currency
			}
		}
	}
	if origin.RetrievedAt == "" {
		origin.RetrievedAt = store.Now()
	}
	item, err := s.items.Create(ctx, householdID, userID, idempotencyKey, input, origin)
	if err != nil {
		return items.Item{}, err
	}
	s.store.Database.ExecContext(ctx, `UPDATE imports SET state = ?, updated_at = ? WHERE id = ?`, StateSaved, store.Now(), identifier)
	return item, nil
}

// RunBackground is the single bounded import worker.
func (s *Service) RunBackground(ctx context.Context) {
	s.recoverInterrupted(ctx)
	ticker := time.NewTicker(config.ImportWorkerPollInterval)
	defer ticker.Stop()
	for {
		for s.processNext(ctx) {
		}
		s.removeExpired(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
	}
}

// recoverInterrupted requeues jobs that were running when the process
// stopped, or fails them once their attempts are used up.
func (s *Service) recoverInterrupted(ctx context.Context) {
	s.store.Database.ExecContext(ctx, `UPDATE imports SET state = CASE WHEN attempts >= ? THEN ? ELSE ? END,
		error_code = CASE WHEN attempts >= ? THEN ? ELSE error_code END, updated_at = ? WHERE state = ?`,
		config.ImportMaximumAttempts, StateFailed, StateQueued, config.ImportMaximumAttempts, codeTimeout, store.Now(), StateRunning)
}

func (s *Service) removeExpired(ctx context.Context) {
	s.store.Database.ExecContext(ctx, `DELETE FROM imports WHERE expires_at < ? OR (state IN (?, ?) AND updated_at < ?)`,
		store.Now(), StateSaved, StateCancelled, store.FormatTime(time.Now().Add(-config.ImportLeaseDuration)))
}

func (s *Service) processNext(ctx context.Context) bool {
	var identifier, sourceURL, providedDocument string
	var householdID int64
	err := s.store.Database.QueryRowContext(ctx, `UPDATE imports SET state = ?, attempts = attempts + 1, lease_expires_at = ?, updated_at = ?
		WHERE id = (SELECT id FROM imports WHERE state = ? ORDER BY created_at LIMIT 1)
		RETURNING id, source_url, household_id, provided_document`,
		StateRunning, store.FormatTime(time.Now().Add(config.ImportLeaseDuration)), store.Now(), StateQueued).Scan(&identifier, &sourceURL, &householdID, &providedDocument)
	if err != nil {
		return false
	}

	jobContext, cancel := context.WithTimeout(ctx, config.ImportTotalBudget)
	s.mutex.Lock()
	s.cancels[identifier] = cancel
	s.mutex.Unlock()
	defer func() {
		cancel()
		s.mutex.Lock()
		delete(s.cancels, identifier)
		s.mutex.Unlock()
	}()

	result, errorCode := s.run(jobContext, householdID, identifier, sourceURL, providedDocument)
	state := StateDone
	if errorCode != "" {
		state = StateFailed
	}
	encoded, _ := json.Marshal(result)
	_, err = s.store.Database.ExecContext(ctx, `UPDATE imports SET state = ?, error_code = ?, result_json = ?, retrieved_at = ?, updated_at = ? WHERE id = ? AND state = ?`,
		state, errorCode, string(encoded), store.Now(), store.Now(), identifier, StateRunning)
	if err != nil {
		slog.Warn("import result not saved", "error", err)
	}
	return true
}

func (s *Service) run(ctx context.Context, householdID int64, identifier string, sourceURL string, providedDocument string) (Result, string) {
	pageURL, document, errorCode := s.pageContent(ctx, sourceURL, providedDocument)
	if errorCode != "" {
		return Result{}, errorCode
	}
	extracted, err := extract.Extract(pageURL, document)
	if err != nil {
		return Result{}, codeNoDetails
	}
	result := Result{PageTitle: extracted.PageTitle, Partial: extracted.Partial, Sources: extracted.Sources, Candidates: []Candidate{}}
	for _, candidate := range extracted.Candidates {
		result.Candidates = append(result.Candidates, convert(candidate))
	}
	if len(result.Candidates) == 0 && extracted.PageTitle == "" {
		return result, codeNoDetails
	}
	if len(result.Candidates) == 0 {
		result.Candidates = []Candidate{{Name: extracted.PageTitle, ImageIDs: []int64{}}}
		result.Partial = true
	}
	s.downloadImages(ctx, householdID, identifier, extracted, result.Candidates)
	return result, ""
}

// pageContent returns the page to extract from: the snapshot sent by the
// browser button when there is one, otherwise the page fetched safely.
func (s *Service) pageContent(ctx context.Context, sourceURL string, providedDocument string) (*url.URL, []byte, string) {
	if providedDocument != "" {
		parsed, err := url.Parse(sourceURL)
		if err != nil {
			return nil, nil, codeNoDetails
		}
		return parsed, []byte(providedDocument), ""
	}
	if allowed, err := s.robotsAllow(ctx, sourceURL); err == nil && !allowed {
		return nil, nil, string(safefetch.CodeBlocked)
	}
	page, err := s.fetcher.Get(ctx, sourceURL, safefetch.DefaultPageLimits(), pageAccept)
	if err != nil {
		return nil, nil, fetchErrorCode(err)
	}
	return page.FinalURL, page.Body, ""
}

func convert(candidate extract.Candidate) Candidate {
	converted := Candidate{Name: candidate.Name, Description: candidate.Description, Brand: candidate.Brand, Model: candidate.Model, SKU: candidate.SKU, ImageIDs: []int64{}}
	for _, offer := range candidate.Offers {
		if offer.Price != nil && converted.ListingPrice == nil {
			converted.ListingPrice = &items.Money{Amount: money.FormatDecimal(offer.Price.MinorUnits, offer.Price.Currency), Currency: offer.Price.Currency}
		}
		if converted.Availability == "" {
			converted.Availability = offer.Availability
		}
		if converted.Seller == "" {
			converted.Seller = offer.Seller
		}
	}
	return converted
}

// downloadImages fetches up to the configured number of distinct candidate
// images through the same safe fetcher. Failures are skipped quietly: the
// draft is still useful without photos. Every attempt counts towards the
// limit, but only images that were saved and linked to the draft are shared
// with the variants that use the same URL.
func (s *Service) downloadImages(ctx context.Context, householdID int64, identifier string, extracted extract.Result, candidates []Candidate) {
	attempted := map[string]bool{}
	saved := map[string]int64{}
	for index, candidate := range extracted.Candidates {
		for _, imageURL := range candidate.ImageURLs {
			if attempted[imageURL] {
				if attachmentID, ok := saved[imageURL]; ok {
					candidates[index].ImageIDs = append(candidates[index].ImageIDs, attachmentID)
				}
				continue
			}
			if len(attempted) >= config.ImportImageCandidates || ctx.Err() != nil {
				return
			}
			attachmentID, err := s.downloadImage(ctx, householdID, identifier, imageURL, len(attempted))
			attempted[imageURL] = true
			if err != nil || attachmentID == 0 {
				continue
			}
			saved[imageURL] = attachmentID
			candidates[index].ImageIDs = append(candidates[index].ImageIDs, attachmentID)
		}
	}
}

var imageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

func (s *Service) downloadImage(ctx context.Context, householdID int64, identifier string, imageURL string, position int) (int64, error) {
	response, err := s.fetcher.Get(ctx, imageURL, safefetch.DefaultImageLimits(), imageAccept)
	if err != nil {
		return 0, err
	}
	sniffed := http.DetectContentType(response.Body)
	if !imageTypes[sniffed] {
		return 0, errors.New("not an image")
	}
	name := path.Base(response.FinalURL.Path)
	if name == "." || name == "/" || !strings.Contains(name, ".") {
		name = imageFilename
	}
	attachment, err := s.attachments.Save(ctx, householdID, attachments.KindPhoto, attachments.CleanFilename(name), sniffed, bytes.NewReader(response.Body))
	if err != nil {
		return 0, err
	}
	_, err = s.store.Database.ExecContext(ctx, `INSERT INTO import_images (import_id, attachment_id, position) VALUES (?, ?, ?)`, identifier, attachment.ID, position)
	return attachment.ID, err
}
