package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// DateLayout tarih alanlarının JSON ve veritabanı biçimidir.
const DateLayout = "2006-01-02"

// Date saat ve saat dilimi taşımayan takvim tarihidir. JSON'da "YYYY-MM-DD" olarak
// yazılır ve PostgreSQL'de DATE sütununa eşlenir; böylece saat dilimi kayması olmaz.
type Date struct {
	time.Time
}

// ParseDate "YYYY-MM-DD" biçimindeki metni Date'e çevirir.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, strings.TrimSpace(s))
	if err != nil {
		return Date{}, fmt.Errorf("tarih YYYY-AA-GG biçiminde olmalıdır: %w", err)
	}
	return Date{t}, nil
}

func (d Date) String() string { return d.Format(DateLayout) }

// MarshalJSON tarihi "YYYY-MM-DD" olarak yazar.
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON "YYYY-MM-DD" metnini okur.
func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// GormDataType sütun tipini DATE yapar.
func (Date) GormDataType() string { return "date" }

// Value veritabanına "YYYY-MM-DD" yazar.
func (d Date) Value() (driver.Value, error) { return d.String(), nil }

// Scan veritabanından gelen DATE değerini okur.
func (d *Date) Scan(value any) error {
	switch v := value.(type) {
	case time.Time:
		d.Time = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
		return nil
	case string:
		parsed, err := ParseDate(v)
		*d = parsed
		return err
	case []byte:
		parsed, err := ParseDate(string(v))
		*d = parsed
		return err
	default:
		return fmt.Errorf("Date: desteklenmeyen tip %T", value)
	}
}
