package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const MaxBatchSize = 1000

type Generator struct {
	now func() int64
}

func NewGenerator(now func() int64) *Generator {
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &Generator{now: now}
}

var ErrTimestampRange = errors.New("timestamp out of 48-bit range")

func (g *Generator) NextID() ([16]byte, error) {
	ts := g.now()
	if ts < 0 || ts >= 1<<48 {
		return [16]byte{}, fmt.Errorf("%w: %d", ErrTimestampRange, ts)
	}

	var id [16]byte

	id[0] = byte(ts >> 40)
	id[1] = byte(ts >> 32)
	id[2] = byte(ts >> 24)
	id[3] = byte(ts >> 16)
	id[4] = byte(ts >> 8)
	id[5] = byte(ts)

	if _, err := rand.Read(id[6:16]); err != nil {
		return [16]byte{}, fmt.Errorf("crypto/rand read: %w", err)
	}

	id[6] = (id[6] & 0x0F) | 0x70
	id[8] = (id[8] & 0x3F) | 0x80

	return id, nil
}

func (g *Generator) NextIDs(n int) ([][16]byte, error) {
	if n <= 0 || n > MaxBatchSize {
		return nil, fmt.Errorf("batch size %d must be between 1 and %d", n, MaxBatchSize)
	}

	ts := g.now()
	if ts < 0 || ts >= 1<<48 {
		return nil, fmt.Errorf("%w: %d", ErrTimestampRange, ts)
	}

	buf := make([]byte, n*10)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("crypto/rand read batch: %w", err)
	}

	ids := make([][16]byte, n)
	for i := 0; i < n; i++ {
		var id [16]byte
		id[0] = byte(ts >> 40)
		id[1] = byte(ts >> 32)
		id[2] = byte(ts >> 24)
		id[3] = byte(ts >> 16)
		id[4] = byte(ts >> 8)
		id[5] = byte(ts)

		copy(id[6:16], buf[i*10:(i+1)*10])
		id[6] = (id[6] & 0x0F) | 0x70
		id[8] = (id[8] & 0x3F) | 0x80
		ids[i] = id
	}
	return ids, nil
}

func String(id [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		id[0:4],
		id[4:6],
		id[6:8],
		id[8:10],
		id[10:16],
	)
}

func Parse(s string) ([16]byte, error) {
	var id [16]byte

	if len(s) != 36 {
		return id, fmt.Errorf("invalid UUIDv7: length %d, want 36", len(s))
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return id, errors.New("invalid UUIDv7: hyphens at wrong positions")
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'F' {
			return id, errors.New("invalid UUIDv7: uppercase hex not allowed")
		}
	}

	groups := [5]string{s[0:8], s[9:13], s[14:18], s[19:23], s[24:36]}
	offsets := [5]int{0, 4, 6, 8, 10}
	for i, g := range groups {
		b, err := hex.DecodeString(g)
		if err != nil {
			return id, fmt.Errorf("invalid UUIDv7: group %q not hex: %w", g, err)
		}
		copy(id[offsets[i]:], b)
	}

	if id[6]>>4 != 0x7 {
		return id, errors.New("invalid UUIDv7: version is not 7")
	}
	if id[8]>>6 != 0x2 {
		return id, errors.New("invalid UUIDv7: variant is not 10xx")
	}
	return id, nil
}

type Decoded struct {
	TimestampMs   int64
	Version       int
	Variant       string
	RandomPayload string
}

func Decode(id [16]byte) Decoded {
	var ts int64
	for i := 0; i < 6; i++ {
		ts = (ts << 8) | int64(id[i])
	}

	payload := make([]byte, 10)
	copy(payload, id[6:16])
	payload[0] &= 0x0F
	payload[2] &= 0x3F

	return Decoded{
		TimestampMs:   ts,
		Version:       int(id[6] >> 4),
		Variant:       "10xx",
		RandomPayload: hex.EncodeToString(payload),
	}
}
