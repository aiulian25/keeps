package items

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Measurements are optional; each value is kept exactly as entered, with its
// unit, because "80" without a unit is no use when moving furniture.
type Measurements struct {
	Width      string `json:"width"`
	Height     string `json:"height"`
	Depth      string `json:"depth"`
	LengthUnit string `json:"lengthUnit"`
	Weight     string `json:"weight"`
	WeightUnit string `json:"weightUnit"`
	FitNote    string `json:"fitNote"`
}

var (
	lengthUnits = map[string]bool{"mm": true, "cm": true, "m": true, "in": true, "ft": true}
	weightUnits = map[string]bool{"g": true, "kg": true, "lb": true, "oz": true}
	// measurementValue allows up to six whole digits and three decimals, so
	// anything from a watch strap to a wardrobe fits, but typos stand out.
	measurementValue = regexp.MustCompile(`^\d{1,6}(\.\d{1,3})?$`)
	zeroValue        = regexp.MustCompile(`^0+(\.0+)?$`)
)

const fitNoteMaximumLength = 1000

func (m Measurements) empty() bool {
	return m == Measurements{}
}

func (m Measurements) hasLength() bool {
	return m.Width != "" || m.Height != "" || m.Depth != ""
}

// normaliseMeasurements trims everything and checks values and units. It
// returns nil when nothing was given, and field errors keyed "measurements".
func normaliseMeasurements(input *Measurements, fields map[string]string) *Measurements {
	if input == nil {
		return nil
	}
	m := Measurements{
		Width: clean(input.Width), Height: clean(input.Height), Depth: clean(input.Depth),
		LengthUnit: clean(input.LengthUnit), Weight: clean(input.Weight), WeightUnit: clean(input.WeightUnit),
		FitNote: strings.TrimSpace(input.FitNote),
	}
	if !m.hasLength() {
		m.LengthUnit = ""
	}
	if m.Weight == "" {
		m.WeightUnit = ""
	}
	if m.empty() {
		return nil
	}
	for _, value := range []string{m.Width, m.Height, m.Depth, m.Weight} {
		if value != "" && (!measurementValue.MatchString(value) || zeroValue.MatchString(value)) {
			fields["measurements"] = "Enter measurements as numbers above 0, like 80 or 12.5."
			return nil
		}
	}
	if m.hasLength() && !lengthUnits[m.LengthUnit] {
		fields["measurements"] = "Choose a unit for the size: mm, cm, m, in or ft."
		return nil
	}
	if m.Weight != "" && !weightUnits[m.WeightUnit] {
		fields["measurements"] = "Choose a unit for the weight: g, kg, lb or oz."
		return nil
	}
	if tooLong(m.FitNote, fitNoteMaximumLength) {
		fields["measurements"] = "Keep the fitting note under 1000 characters."
		return nil
	}
	return &m
}

// Summary is a one-line, unit-bearing description for history and CSV, e.g.
// "W 80 × H 120 × D 40 cm; 12.5 kg; Needs 85 cm door".
func (m *Measurements) Summary() string {
	if m == nil {
		return ""
	}
	var parts, size []string
	for _, dimension := range []struct{ letter, value string }{{"W", m.Width}, {"H", m.Height}, {"D", m.Depth}} {
		if dimension.value != "" {
			size = append(size, dimension.letter+" "+dimension.value)
		}
	}
	if len(size) > 0 {
		parts = append(parts, strings.Join(size, " × ")+" "+m.LengthUnit)
	}
	if m.Weight != "" {
		parts = append(parts, m.Weight+" "+m.WeightUnit)
	}
	if m.FitNote != "" {
		parts = append(parts, m.FitNote)
	}
	return strings.Join(parts, "; ")
}

func encodeMeasurements(m *Measurements) string {
	if m == nil {
		return ""
	}
	encoded, _ := json.Marshal(m)
	return string(encoded)
}

func decodeMeasurements(stored string) *Measurements {
	if stored == "" {
		return nil
	}
	var m Measurements
	if json.Unmarshal([]byte(stored), &m) != nil || m.empty() {
		return nil
	}
	return &m
}
