package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateJSONAndScan(t *testing.T) {
	var g struct {
		D *Date `json:"d,omitempty"`
	}
	if err := json.Unmarshal([]byte(`{"d":"2026-12-31"}`), &g); err != nil || g.D.String() != "2026-12-31" {
		t.Fatalf("unmarshal: %v %v", err, g.D)
	}
	out, _ := json.Marshal(g)
	if string(out) != `{"d":"2026-12-31"}` {
		t.Fatalf("marshal: %s", out)
	}
	if err := json.Unmarshal([]byte(`{"d":"31.12.2026"}`), &g); err == nil {
		t.Fatal("geçersiz biçim kabul edildi")
	}
	var d Date
	// Sürücü DATE'i saat dilimli time.Time olarak döndürebilir; takvim günü korunmalı.
	if err := d.Scan(time.Date(2026, 3, 1, 0, 0, 0, 0, time.FixedZone("TRT", 3*3600))); err != nil || d.String() != "2026-03-01" {
		t.Fatalf("scan: %v %s", err, d)
	}
	if v, _ := d.Value(); v != "2026-03-01" {
		t.Fatalf("value: %v", v)
	}
}
