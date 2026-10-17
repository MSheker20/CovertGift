package models

import (
	"reflect"
	"testing"
)

func TestParseInterests(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"", []string{}},
		{" , ; \n", []string{}},
		{" Kahve, Kamp;Müzik\r\nFotoğrafçılık ", []string{"Kahve", "Kamp", "Müzik", "Fotoğrafçılık"}},
		{"Kahve, kahve\nKamp\nKamp", []string{"Kahve", "Kamp"}},
	} {
		if got := ParseInterests(tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseInterests(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
