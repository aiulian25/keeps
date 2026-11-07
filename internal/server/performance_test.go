//go:build performance

package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/aiulian25/keeps/internal/items"
)

// Run with: go test -tags performance -run TestPerformanceTargets -v ./internal/server/
// Measures the investigation's targets (§14) on the current machine.
const (
	seededItems    = 10_000
	measuredSaves  = 200
	measuredSearch = 200
	percentile95   = 0.95
	targetLatency  = 300 * time.Millisecond
)

var searchTerms = []string{"coffee", "drill", "lamp", "kitchen", "garage", "sony", "chair", "kettle"}

func percentile(durations []time.Duration, fraction float64) time.Duration {
	sort.Slice(durations, func(left, right int) bool { return durations[left] < durations[right] })
	return durations[int(float64(len(durations)-1)*fraction)]
}

func TestPerformanceTargets(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	ctx := context.Background()

	rooms := []string{"Kitchen", "Garage", "Office", "Bedroom", "Living room"}
	started := time.Now()
	for index := range seededItems {
		input := items.Input{
			Name:  fmt.Sprintf("%s item %d", searchTerms[index%len(searchTerms)], index),
			Room:  rooms[index%len(rooms)],
			Notes: "seeded for performance measurement",
			Tags:  []string{searchTerms[(index+3)%len(searchTerms)]},
		}
		if _, err := server.application.Items.Create(ctx, 1, 1, "", input, nil); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("seeded %d items in %s", seededItems, time.Since(started).Round(time.Millisecond))

	saves := make([]time.Duration, 0, measuredSaves)
	for index := range measuredSaves {
		begin := time.Now()
		result := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": fmt.Sprintf("Measured %d", index), "room": "Kitchen", "pricePaid": map[string]string{"amount": "19.99", "currency": "GBP"}})
		saves = append(saves, time.Since(begin))
		if result.Status != http.StatusCreated {
			t.Fatalf("save failed: %s", result.Body)
		}
	}
	searches := make([]time.Duration, 0, measuredSearch)
	for index := range measuredSearch {
		begin := time.Now()
		result := client.do(http.MethodGet, "/api/v1/items?q="+searchTerms[index%len(searchTerms)], nil)
		searches = append(searches, time.Since(begin))
		if result.Status != http.StatusOK {
			t.Fatalf("search failed: %d", result.Status)
		}
	}
	saveP95, searchP95 := percentile(saves, percentile95), percentile(searches, percentile95)
	t.Logf("manual save p95 = %s (target < %s)", saveP95, targetLatency)
	t.Logf("search p95 over %d items = %s (target < %s)", seededItems, searchP95, targetLatency)
	if saveP95 > targetLatency || searchP95 > targetLatency {
		t.Errorf("a latency target was missed")
	}
}
