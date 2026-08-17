package service

import (
	"math/rand"
	"sync"
)

// lockedRand wraps a math/rand.Rand with a mutex. EconomyService no longer
// serializes whole requests behind a process-wide mutex, but a *rand.Rand
// returned by rand.New is not safe for concurrent use, and this single
// generator is shared by Work, RobUser, playWeightedCasino and
// gbmMultiplier (reached from Work/BuyCrypto/SellCrypto/ProcessNewsTick) —
// all of which can now run at the same time.
//
// Only the four methods actually called on EconomyService.rand are exposed,
// and they keep the names of the underlying rand.Rand methods so call sites
// read exactly as before.
type lockedRand struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func newLockedRand(seed int64) *lockedRand {
	return &lockedRand{rng: rand.New(rand.NewSource(seed))}
}

func (r *lockedRand) Int63n(n int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Int63n(n)
}

func (r *lockedRand) Float64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Float64()
}

func (r *lockedRand) Intn(n int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Intn(n)
}

func (r *lockedRand) NormFloat64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.NormFloat64()
}
