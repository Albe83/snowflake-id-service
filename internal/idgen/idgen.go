package idgen

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

const maxBatchSize = 1000

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
	if n <= 0 || n > maxBatchSize {
		return nil, fmt.Errorf("batch size %d must be between 1 and %d", n, maxBatchSize)
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
