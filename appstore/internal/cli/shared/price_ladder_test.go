package shared

import (
	"math/big"
	"strings"
	"testing"
)

func TestResolvePriceLadderPoint(t *testing.T) {
	tests := []struct {
		name       string
		candidates []PriceLadderCandidate
		desired    string
		rounding   PriceRounding
		wantID     string
		wantPrice  string
		wantErr    string
	}{
		{
			name:       "exact accepts numerically equal decimals",
			candidates: deriveCandidates("lower", "8.99", "exact", "9.00", "upper", "9.99"),
			desired:    "9",
			rounding:   PriceRoundingExact,
			wantID:     "exact",
			wantPrice:  "9.00",
		},
		{
			name:       "exact rejects a missing amount",
			candidates: deriveCandidates("lower", "89", "upper", "99"),
			desired:    "90",
			rounding:   PriceRoundingExact,
			wantErr:    "no exact target price point for 90",
		},
		{
			name:       "nearest chooses lower distance",
			candidates: deriveCandidates("upper", "99", "lower", "89"),
			desired:    "90",
			rounding:   PriceRoundingNearest,
			wantID:     "lower",
			wantPrice:  "89",
		},
		{
			name:       "nearest chooses upper distance",
			candidates: deriveCandidates("lower", "89", "upper", "99"),
			desired:    "97",
			rounding:   PriceRoundingNearest,
			wantID:     "upper",
			wantPrice:  "99",
		},
		{
			name:       "nearest tie chooses lower",
			candidates: deriveCandidates("upper", "100", "lower", "90"),
			desired:    "95",
			rounding:   PriceRoundingNearest,
			wantID:     "lower",
			wantPrice:  "90",
		},
		{
			name:       "nearest below ladder chooses minimum",
			candidates: deriveCandidates("high", "20", "low", "10"),
			desired:    "1",
			rounding:   PriceRoundingNearest,
			wantID:     "low",
			wantPrice:  "10",
		},
		{
			name:       "nearest above ladder chooses maximum",
			candidates: deriveCandidates("high", "20", "low", "10"),
			desired:    "30",
			rounding:   PriceRoundingNearest,
			wantID:     "high",
			wantPrice:  "20",
		},
		{
			name:       "up chooses exact boundary",
			candidates: deriveCandidates("high", "20.00", "low", "10"),
			desired:    "20",
			rounding:   PriceRoundingUp,
			wantID:     "high",
			wantPrice:  "20.00",
		},
		{
			name:       "up chooses smallest greater amount",
			candidates: deriveCandidates("high", "20", "low", "10", "middle", "15"),
			desired:    "12",
			rounding:   PriceRoundingUp,
			wantID:     "middle",
			wantPrice:  "15",
		},
		{
			name:       "up fails above ladder",
			candidates: deriveCandidates("high", "20", "low", "10"),
			desired:    "21",
			rounding:   PriceRoundingUp,
			wantErr:    "no target price point at or above 21",
		},
		{
			name:       "down chooses exact boundary",
			candidates: deriveCandidates("high", "20", "low", "10.00"),
			desired:    "10",
			rounding:   PriceRoundingDown,
			wantID:     "low",
			wantPrice:  "10.00",
		},
		{
			name:       "down chooses largest lower amount",
			candidates: deriveCandidates("high", "20", "low", "10", "middle", "15"),
			desired:    "18",
			rounding:   PriceRoundingDown,
			wantID:     "middle",
			wantPrice:  "15",
		},
		{
			name:       "down fails below ladder",
			candidates: deriveCandidates("high", "20", "low", "10"),
			desired:    "9",
			rounding:   PriceRoundingDown,
			wantErr:    "no target price point at or below 9",
		},
		{
			name:       "selected duplicate amount is ambiguous",
			candidates: deriveCandidates("one", "9", "two", "9.00", "other", "10"),
			desired:    "9",
			rounding:   PriceRoundingNearest,
			wantErr:    "multiple target price points match selected price 9",
		},
		{
			name:       "malformed candidate fails explicitly",
			candidates: deriveCandidates("valid", "9", "broken", "not-a-price"),
			desired:    "9",
			rounding:   PriceRoundingNearest,
			wantErr:    `target price point "broken" has invalid customer price`,
		},
		{
			name:       "free point is never selected",
			candidates: deriveCandidates("free", "0.0", "lowest", "0.29"),
			desired:    "0.05",
			rounding:   PriceRoundingNearest,
			wantID:     "lowest",
			wantPrice:  "0.29",
		},
		{
			name:       "repeated id with conflicting prices fails explicitly",
			candidates: deriveCandidates("same", "9", "same", "10"),
			desired:    "9",
			rounding:   PriceRoundingNearest,
			wantErr:    `target price point "same" has conflicting customer prices`,
		},
		{
			name:       "identical repeated resources are deduplicated",
			candidates: deriveCandidates("same", "9", "same", "9.00", "other", "10"),
			desired:    "9",
			rounding:   PriceRoundingNearest,
			wantID:     "same",
			wantPrice:  "9",
		},
		{
			name:     "empty ladder fails",
			desired:  "9",
			rounding: PriceRoundingNearest,
			wantErr:  "target price-point ladder is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			desired, ok := new(big.Rat).SetString(test.desired)
			if !ok {
				t.Fatalf("invalid test desired price %q", test.desired)
			}
			got, err := ResolvePriceLadderPoint(test.candidates, desired, test.rounding)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePriceLadderPoint() error = %v", err)
			}
			if got.PricePointID != test.wantID || got.CustomerPrice != test.wantPrice {
				t.Fatalf("resolution = %+v, want id=%q price=%q", got, test.wantID, test.wantPrice)
			}
		})
	}
}

func TestResolvePriceLadderPointUsesExactDecimalMultiplication(t *testing.T) {
	source, _ := new(big.Rat).SetString("0.1")
	multiplier, _ := new(big.Rat).SetString("3")
	desired := new(big.Rat).Mul(source, multiplier)

	got, err := ResolvePriceLadderPoint(
		deriveCandidates("exact", "0.30", "upper", "0.31"),
		desired,
		PriceRoundingExact,
	)
	if err != nil {
		t.Fatalf("ResolvePriceLadderPoint() error = %v", err)
	}
	if got.PricePointID != "exact" {
		t.Fatalf("expected exact decimal point, got %+v", got)
	}
}

func TestFormatExactPriceDecimalPreservesInputPrecision(t *testing.T) {
	value, ok := new(big.Rat).SetString("1.0000000000001")
	if !ok {
		t.Fatal("parse exact test decimal")
	}
	if got := FormatExactPriceDecimal(value); got != "1.0000000000001" {
		t.Fatalf("exact decimal = %q, want 1.0000000000001", got)
	}
}

func TestFormatPriceDecimalPreservesNonzeroValues(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		precision int
		want      string
	}{
		{name: "positive below requested precision", value: "0.0000001", precision: 6, want: "0.0000001"},
		{name: "negative below requested precision", value: "-0.0000001", precision: 6, want: "-0.0000001"},
		{name: "small recurring rational", value: "1/30000000", precision: 6, want: "0.00000003"},
		{name: "exact zero", value: "0", precision: 6, want: "0"},
		{name: "ordinary rounding remains bounded", value: "1/9", precision: 6, want: "0.111111"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, ok := new(big.Rat).SetString(test.value)
			if !ok {
				t.Fatalf("parse test value %q", test.value)
			}
			if got := FormatPriceDecimal(value, test.precision); got != test.want {
				t.Fatalf("formatted decimal = %q, want %q", got, test.want)
			}
		})
	}
}

func deriveCandidates(values ...string) []PriceLadderCandidate {
	result := make([]PriceLadderCandidate, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		result = append(result, PriceLadderCandidate{
			PricePointID:  values[i],
			CustomerPrice: values[i+1],
		})
	}
	return result
}
