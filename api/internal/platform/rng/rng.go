// Package rng provides the seeded randomness the rules engine is given; every seed is logged by its caller.
package rng

import (
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand/v2"
)

// Seed draws a fresh seed from the operating system.
func Seed() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:]) // never fails since Go 1.24
	return binary.LittleEndian.Uint64(b[:])
}

// Source is a deterministic generator: the same seed always rolls the same faces.
type Source struct {
	r *mrand.Rand
}

// New returns the generator for a seed.
func New(seed uint64) *Source {
	return &Source{r: mrand.New(mrand.NewPCG(seed, seed^0x9e3779b97f4a7c15))} //nolint:gosec // G404: replayable game dice, not secrets
}

// IntN returns a uniform integer in [0, n).
func (s *Source) IntN(n int) int {
	return s.r.IntN(n)
}
