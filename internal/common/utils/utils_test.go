package utils

import "testing"

func TestCalculateElectricityBill(t *testing.T) {
	cases := []struct {
		kwh  float64
		want float64
	}{
		{0, 0},
		// 100*1901 = 190 100; VAT 15 208 -> 205 308
		{100, 205308},
		// 100*1901 + 68*2289 = 345 752; VAT 27 660 -> 373 412
		{168, 373412},
		// 190 100 + 228 900 + 200*2894 = 997 800; VAT 79 824 -> 1 077 624
		{400, 1077624},
		// 997 800 + 300*3456 + 100*3844 = 2 419 000; VAT 193 520 -> 2 612 520
		{800, 2612520},
	}
	for _, c := range cases {
		if got := CalculateElectricityBill(c.kwh); got != c.want {
			t.Errorf("CalculateElectricityBill(%v) = %v, want %v", c.kwh, got, c.want)
		}
	}
}
