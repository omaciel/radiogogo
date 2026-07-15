// Package station provides the built-in catalog of radio stations.
package station

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// Station is a named radio stream.
type Station struct {
	Name string
	URL  string
}

// catalog holds the built-in stations. Every URL must use https.
var catalog = []Station{
	{Name: "Radio Paradise", URL: "https://stream.radioparadise.com/mp3-192"},
	{Name: "Radio Swiss Classic", URL: "https://stream.srg-ssr.ch/m/rsc_de/mp3_128"},
	{Name: "WUNC", URL: "https://edg-iad-wunc-ice.streamguys1.com/wunc-128-mp3.m3u"},
}

// All returns the catalog ordered by name. The result is a copy; callers may
// modify it freely.
func All() []Station {
	out := slices.Clone(catalog)
	slices.SortFunc(out, func(a, b Station) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// Lookup returns the station with the given name, ignoring case.
func Lookup(name string) (Station, error) {
	for _, s := range catalog {
		if strings.EqualFold(s.Name, name) {
			return s, nil
		}
	}
	return Station{}, fmt.Errorf("unknown station %q", name)
}

// Random returns a station chosen at random.
func Random() Station {
	return RandomFrom(rand.IntN)
}

// RandomFrom returns a station chosen using intn, which must behave like
// [math/rand/v2.IntN]. It exists so tests can supply a deterministic source.
func RandomFrom(intn func(int) int) Station {
	all := All()
	return all[intn(len(all))]
}
