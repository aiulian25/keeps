//go:build retailerbenchmark

package importer

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aiulian25/keeps/internal/importer/extract"
	"github.com/aiulian25/keeps/internal/safefetch"
)

// Live, permitted public-page benchmark (IMP-038). Not part of CI.
// KEEPS_BENCHMARK_URLS=<list> KEEPS_BENCHMARK_OUTPUT=<csv> go test -tags retailerbenchmark -run TestRetailerBenchmark -v ./internal/importer/
const pauseBetweenRequests = 3 * time.Second

func TestRetailerBenchmark(t *testing.T) {
	list, err := os.Open(os.Getenv("KEEPS_BENCHMARK_URLS"))
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()
	output, err := os.Create(os.Getenv("KEEPS_BENCHMARK_OUTPUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	writer := csv.NewWriter(output)
	writer.Write([]string{"retailer", "case", "host", "robots_allowed", "outcome", "elapsed_ms", "candidates", "name", "listing_price", "currency", "images", "partial", "sources"})

	fetcher := safefetch.New()
	service := &Service{fetcher: fetcher}
	scanner := bufio.NewScanner(list)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, " | ")
		retailer, kind, link := parts[0], parts[1], parts[2]
		parsed, _ := url.Parse(link)
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		started := time.Now()
		allowed, _ := service.robotsAllow(ctx, link)
		row := []string{retailer, kind, parsed.Hostname(), fmt.Sprint(allowed)}
		if !allowed {
			row = append(row, "robots_disallow", fmt.Sprint(time.Since(started).Milliseconds()))
			writer.Write(row)
			cancel()
			continue
		}
		page, err := fetcher.Get(ctx, link, safefetch.DefaultPageLimits(), pageAccept)
		elapsed := fmt.Sprint(time.Since(started).Milliseconds())
		cancel()
		if err != nil {
			row = append(row, fetchErrorCode(err), elapsed)
			writer.Write(row)
			t.Logf("%-12s %-20s %s", retailer, kind, fetchErrorCode(err))
			time.Sleep(pauseBetweenRequests)
			continue
		}
		result, err := extract.Extract(page.FinalURL, page.Body)
		if err != nil || len(result.Candidates) == 0 {
			row = append(row, "no_details", elapsed, "0", result.PageTitle)
			writer.Write(row)
			t.Logf("%-12s %-20s no_details (title %q)", retailer, kind, result.PageTitle)
			time.Sleep(pauseBetweenRequests)
			continue
		}
		first := result.Candidates[0]
		price, currency := "", ""
		for _, offer := range first.Offers {
			if offer.Price != nil {
				price, currency = fmt.Sprint(offer.Price.MinorUnits), offer.Price.Currency
				break
			}
		}
		sources := []string{}
		for field, source := range result.Sources {
			sources = append(sources, field+"="+source)
		}
		row = append(row, "ok", elapsed, fmt.Sprint(len(result.Candidates)), first.Name, price, currency, fmt.Sprint(len(first.ImageURLs)), fmt.Sprint(result.Partial), strings.Join(sources, ";"))
		writer.Write(row)
		t.Logf("%-12s %-20s ok  %q price=%s %s images=%d", retailer, kind, first.Name, price, currency, len(first.ImageURLs))
		time.Sleep(pauseBetweenRequests)
	}
	writer.Flush()
}
