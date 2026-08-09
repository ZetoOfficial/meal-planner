package nutrition

import (
	"math"
	"testing"
)

func TestMacrosAdd(t *testing.T) {
	got, err := (Macros{ProteinG: 10.5, FatG: 4, CarbG: 20.25, Kcal: 160}).Add(
		Macros{ProteinG: 5, FatG: 2.5, CarbG: 10.25, Kcal: 80},
	)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	want := Macros{ProteinG: 15.5, FatG: 6.5, CarbG: 30.5, Kcal: 240}
	if got != want {
		t.Fatalf("Add() = %#v, want %#v", got, want)
	}
}

func TestMacrosAddRejectsInvalidValues(t *testing.T) {
	for name, value := range map[string]Macros{
		"negative grams": {ProteinG: -1},
		"negative kcal":  {Kcal: -1},
		"not a number":   {FatG: math.NaN()},
		"infinite":       {CarbG: math.Inf(1)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := (Macros{}).Add(value); err == nil {
				t.Fatal("Add() error = nil, want invalid macros error")
			}
		})
	}
}

func TestMacrosScale(t *testing.T) {
	got, err := (Macros{ProteinG: 12.5, FatG: 4.25, CarbG: 20.5, Kcal: 101}).Scale(0.5)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	want := Macros{ProteinG: 6.25, FatG: 2.125, CarbG: 10.25, Kcal: 51}
	if got != want {
		t.Fatalf("Scale() = %#v, want %#v", got, want)
	}
}

func TestMacrosScaleRejectsInvalidFactor(t *testing.T) {
	macros := Macros{ProteinG: 1, FatG: 1, CarbG: 1, Kcal: 1}
	for name, factor := range map[string]float64{
		"negative":     -1,
		"not a number": math.NaN(),
		"infinite":     math.Inf(1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := macros.Scale(factor); err == nil {
				t.Fatal("Scale() error = nil, want invalid factor error")
			}
		})
	}
}
