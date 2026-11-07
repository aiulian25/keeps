package items

import (
	"context"
	"database/sql"

	"github.com/aiulian25/keeps/internal/money"
)

// CostSummary totals what was paid for a set of items, one amount per
// currency. Currencies are never added together. Items with no price paid
// are counted separately so a total is never silently incomplete.
type CostSummary struct {
	Totals        map[string]string `json:"totals"`
	PricedItems   int               `json:"pricedItems"`
	UnpricedItems int               `json:"unpricedItems"`
}

type RoomCosts struct {
	Rooms      map[int64]CostSummary `json:"rooms"`
	AllRooms   CostSummary           `json:"allRooms"`
	Unassigned CostSummary           `json:"unassigned"`
}

// costAccumulator sums exact minor units before formatting once at the end.
type costAccumulator struct {
	minor    map[string]int64
	priced   int
	unpriced int
}

func newCostAccumulator() *costAccumulator {
	return &costAccumulator{minor: map[string]int64{}}
}

func (a *costAccumulator) add(currency sql.NullString, minor int64, count int) {
	if !currency.Valid {
		a.unpriced += count
		return
	}
	a.minor[currency.String] += minor
	a.priced += count
}

func (a *costAccumulator) summary() CostSummary {
	totals := map[string]string{}
	for currency, minor := range a.minor {
		totals[currency] = money.FormatDecimal(minor, currency)
	}
	return CostSummary{Totals: totals, PricedItems: a.priced, UnpricedItems: a.unpriced}
}

// RoomCosts returns the price paid (unit price × quantity) per room, across
// all rooms, and for items not in any room. Deleted items are excluded.
func (s *Service) RoomCosts(ctx context.Context, householdID int64) (RoomCosts, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT coalesce(room_id, 0), paid_currency, coalesce(sum(paid_unit_minor * quantity), 0), count(*)
		FROM items WHERE household_id = ? AND deleted_at IS NULL GROUP BY room_id, paid_currency`, householdID)
	if err != nil {
		return RoomCosts{}, err
	}
	defer rows.Close()
	perRoom := map[int64]*costAccumulator{}
	allRooms, unassigned := newCostAccumulator(), newCostAccumulator()
	for rows.Next() {
		var roomID, minor int64
		var currency sql.NullString
		var count int
		if err := rows.Scan(&roomID, &currency, &minor, &count); err != nil {
			return RoomCosts{}, err
		}
		if roomID == 0 {
			unassigned.add(currency, minor, count)
			continue
		}
		if perRoom[roomID] == nil {
			perRoom[roomID] = newCostAccumulator()
		}
		perRoom[roomID].add(currency, minor, count)
		allRooms.add(currency, minor, count)
	}
	if err := rows.Err(); err != nil {
		return RoomCosts{}, err
	}
	costs := RoomCosts{Rooms: map[int64]CostSummary{}, AllRooms: allRooms.summary(), Unassigned: unassigned.summary()}
	for roomID, accumulator := range perRoom {
		costs.Rooms[roomID] = accumulator.summary()
	}
	return costs, nil
}
