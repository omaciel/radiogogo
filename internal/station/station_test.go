package station

import (
	"strings"
	"testing"
)

func TestAllIsSortedByName(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("All() returned no stations")
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Errorf("All() is not sorted: %q precedes %q", all[i-1].Name, all[i].Name)
		}
	}
}

func TestAllReturnsACopy(t *testing.T) {
	All()[0].Name = "mutated"
	if All()[0].Name == "mutated" {
		t.Error("All() exposed the underlying catalog; a caller can corrupt it")
	}
}

func TestEveryCatalogURLIsHTTPS(t *testing.T) {
	for _, s := range catalog {
		if !strings.HasPrefix(s.URL, "https://") {
			t.Errorf("station %q uses %q; the catalog must be https only", s.Name, s.URL)
		}
	}
}

func TestLookup(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"exact match", "WUNC", "WUNC", false},
		{"lowercase", "wunc", "WUNC", false},
		{"mixed case with space", "radio paradise", "Radio Paradise", false},
		{"unknown station", "KEXP", "", true},
		{"empty name", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Lookup(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Lookup(%q) = %v, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lookup(%q) returned %v, want nil", tt.input, err)
			}
			if got.Name != tt.want {
				t.Errorf("Lookup(%q) = %q, want %q", tt.input, got.Name, tt.want)
			}
		})
	}
}

func TestRandomFromSelectsByIndex(t *testing.T) {
	all := All()
	for i := range all {
		got := RandomFrom(func(int) int { return i })
		if got != all[i] {
			t.Errorf("RandomFrom returning %d = %v, want %v", i, got, all[i])
		}
	}
}

func TestRandomFromIsGivenTheCatalogLength(t *testing.T) {
	var gotN int
	RandomFrom(func(n int) int {
		gotN = n
		return 0
	})
	if want := len(All()); gotN != want {
		t.Errorf("generator called with n=%d, want %d; it must be able to pick any station", gotN, want)
	}
}
