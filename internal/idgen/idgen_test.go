package idgen

import (
	"sync"
	"testing"
)

func TestNextID_Timestamp_BigEndian(t *testing.T) {
	fixedTS := int64(1717500204)
	g := NewGenerator(func() int64 { return fixedTS })

	id, err := g.NextID()
	if err != nil {
		t.Fatal(err)
	}

	ts := int64(0)
	for i := 0; i < 6; i++ {
		ts = (ts << 8) | int64(id[i])
	}
	if ts != fixedTS {
		t.Fatalf("timestamp mismatch: got %d, want %d", ts, fixedTS)
	}

	if id[6]>>4 != 0x7 {
		t.Fatalf("version nibble mismatch: got 0x%x, want 0x7", id[6]>>4)
	}
	if id[8]>>6 != 0x2 {
		t.Fatalf("variant bits mismatch: got 0b%02b, want 0b10", id[8]>>6)
	}
}

func TestNextID_ErrorOnTimestampOutOfRange(t *testing.T) {
	tests := []struct {
		name string
		ts   int64
	}{
		{"negative", -1},
		{"overflow", 1 << 48},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(func() int64 { return tt.ts })
			_, err := g.NextID()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestNextIDs_Batch(t *testing.T) {
	g := NewGenerator(nil)

	ids, err := g.NextIDs(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 5 {
		t.Fatalf("expected 5 ids, got %d", len(ids))
	}

	seen := make(map[[16]byte]struct{})
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id in batch: %x", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNextIDs_ErrorOnInvalidBatchSize(t *testing.T) {
	g := NewGenerator(nil)

	tests := []struct {
		name string
		n    int
	}{
		{"zero", 0},
		{"negative", -1},
		{"over max", MaxBatchSize + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := g.NextIDs(tt.n)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestString(t *testing.T) {
	id := [16]byte{
		0x01, 0x8f, 0x3a, 0x2c, 0x9e, 0x5b, 0x70, 0x00,
		0x80, 0x00, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc,
	}
	expected := "018f3a2c-9e5b-7000-8000-123456789abc"
	got := String(id)
	if got != expected {
		t.Fatalf("String: got %q, want %q", got, expected)
	}
}

func TestNextID_ConcurrencyNoDuplicates(t *testing.T) {
	g := NewGenerator(nil)
	const goroutines = 100
	const idsPerGoroutine = 100

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[[16]byte]struct{})

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range idsPerGoroutine {
				id, err := g.NextID()
				if err != nil {
					t.Errorf("NextID: %v", err)
					return
				}
				mu.Lock()
				if _, ok := seen[id]; ok {
					t.Errorf("duplicate id detected: %x", id)
				}
				seen[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	expected := goroutines * idsPerGoroutine
	if len(seen) != expected {
		t.Fatalf("expected %d unique ids, got %d", expected, len(seen))
	}
}

func TestParse_Valid(t *testing.T) {
	id, err := Parse("018f3a2c-9e5b-7000-8000-123456789abc")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := [16]byte{
		0x01, 0x8f, 0x3a, 0x2c, 0x9e, 0x5b, 0x70, 0x00,
		0x80, 0x00, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc,
	}
	if id != want {
		t.Fatalf("Parse: got %x, want %x", id, want)
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too short", "018f3a2c-9e5b-7000-8000-123456789ab"},
		{"too long", "018f3a2c-9e5b-7000-8000-123456789abcd"},
		{"missing hyphen", "018f3a2c9e5b-7000-8000-123456789abc"},
		{"hyphen wrong position", "018f3a2c-9e5b-700-08000-123456789abc"},
		{"non-hex", "018f3a2c-9e5b-7000-8000-123456789abz"},
		{"uppercase", "018F3A2C-9E5B-7000-8000-123456789ABC"},
		{"version 4", "018f3a2c-9e5b-4000-8000-123456789abc"},
		{"variant 00", "018f3a2c-9e5b-7000-0000-123456789abc"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.input); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestDecode_TestVector(t *testing.T) {
	id := [16]byte{
		0x01, 0x8f, 0x3a, 0x2c, 0x9e, 0x5b, 0x70, 0x00,
		0x80, 0x00, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc,
	}
	d := Decode(id)

	if d.TimestampMs != 1714667953755 {
		t.Fatalf("timestamp: got %d, want %d", d.TimestampMs, 1714667953755)
	}
	if d.Version != 7 {
		t.Fatalf("version: got %d, want 7", d.Version)
	}
	if d.Variant != "10xx" {
		t.Fatalf("variant: got %q, want %q", d.Variant, "10xx")
	}
	if d.RandomPayload != "00000000123456789abc" {
		t.Fatalf("random_payload: got %q, want %q", d.RandomPayload, "00000000123456789abc")
	}
}

func TestParseDecode_Roundtrip(t *testing.T) {
	g := NewGenerator(nil)
	for range 50 {
		id, err := g.NextID()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(String(id))
		if err != nil {
			t.Fatalf("Parse(%s): %v", String(id), err)
		}
		if parsed != id {
			t.Fatalf("roundtrip mismatch: parsed %x, want %x", parsed, id)
		}
		d := Decode(parsed)
		if d.Version != 7 {
			t.Fatalf("version: got %d, want 7", d.Version)
		}
	}
}
