package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

var seq uint64

// newID generates a unique identifier with a descriptive prefix.
// It combines a random component with a monotonically increasing sequence
// to guarantee uniqueness even under heavy concurrency.
func newID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	n := atomic.AddUint64(&seq, 1)
	return fmt.Sprintf("%s_%s_%d", prefix, hex.EncodeToString(b), n)
}
