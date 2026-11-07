package server

import (
	"net/http"
	"testing"
)

func TestCommaAmountsAreNeverSilentlyRescaled(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	refused := []string{"79,99", "1,5", "1,234", "12,34.50", "1.234,56"}
	for _, amount := range refused {
		result := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": "Lamp", "pricePaid": map[string]string{"amount": amount, "currency": "EUR"}})
		var body struct {
			Fields map[string]string `json:"fields"`
		}
		result.json(t, &body)
		if result.Status != http.StatusBadRequest || body.Fields["pricePaid"] == "" {
			t.Errorf("%q was not refused with a price error: %d %s", amount, result.Status, result.Body)
		}
	}
	var created int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM items`).Scan(&created)
	if created != 0 {
		t.Fatalf("refused amounts created %d items", created)
	}

	accepted := []struct{ amount, currency, want string }{
		{"1,234.50", "GBP", "1234.50"},
		{" 79.99 ", "EUR", "79.99"},
		{"0", "GBP", "0.00"},
		{"1500", "JPY", "1500"},
		{"1.234", "BHD", "1.234"},
	}
	for _, testCase := range accepted {
		item := client.createItem(map[string]any{"name": "Lamp", "pricePaid": map[string]string{"amount": testCase.amount, "currency": testCase.currency}})
		if item.PricePaid == nil || (*item.PricePaid)["amount"] != testCase.want {
			t.Errorf("%q %s saved as %v, want %s", testCase.amount, testCase.currency, item.PricePaid, testCase.want)
		}
	}
	if unknown := client.createItem(map[string]any{"name": "Unknown price"}); unknown.PricePaid != nil {
		t.Fatal("a missing price became a value")
	}
}
