package service

import (
	"math/rand"
	"time"
)

const (
	baseNewsProbability        = 0.002  // 0.2%
	ceilingStepProbability     = 0.0005 // 0.05% / miss
	ceilingMaxBonusProbability = 0.02   // +2.0% max
)

type ProbabilityDetails struct {
	Now               time.Time
	LocationName      string
	Base              float64
	CeilingMultiplier float64
	TimeMultiplier    float64
	WeekendMultiplier float64
	Final             float64
	PityCounter       int
}

type NewsEngine struct {
	rng      *rand.Rand
	location *time.Location
}

func newNewsEngine(rng *rand.Rand, location *time.Location) *NewsEngine {
	return &NewsEngine{
		rng:      rng,
		location: location,
	}
}

func (e *NewsEngine) now() time.Time {
	return time.Now().In(e.location)
}

func (e *NewsEngine) CurrentProbabilityDetails(now time.Time, pityCounter int) ProbabilityDetails {
	return e.currentProbabilityDetails(now.In(e.location), pityCounter)
}

func (e *NewsEngine) RollEvent(now time.Time, pityCounter int) (EventType, float64, bool) {
	n := now.In(e.location)
	probability := e.getCurrentProbability(n, pityCounter)
	if e.rng.Float64() >= probability {
		return "", probability, false
	}
	return e.pickWeightedEvent(n), probability, true
}

func (e *NewsEngine) pickWeightedEvent(now time.Time) EventType {
	return e.pickWeightedEventInternal(now.In(e.location))
}

func (e *NewsEngine) getCurrentProbability(now time.Time, pityCounter int) float64 {
	d := e.currentProbabilityDetails(now, pityCounter)
	if d.Final < 0 {
		return 0
	}
	if d.Final > 1 {
		return 1
	}
	return d.Final
}

func (e *NewsEngine) currentProbabilityDetails(now time.Time, pityCounter int) ProbabilityDetails {
	ceilingBonus := float64(pityCounter) * ceilingStepProbability
	if ceilingBonus > ceilingMaxBonusProbability {
		ceilingBonus = ceilingMaxBonusProbability
	}
	ceilingMultiplier := (baseNewsProbability + ceilingBonus) / baseNewsProbability
	timeMultiplier := timeBasedMultiplier(now)
	weekendMultiplier := weekendHitMultiplier(now)
	final := baseNewsProbability * ceilingMultiplier * timeMultiplier * weekendMultiplier

	return ProbabilityDetails{
		Now:               now,
		LocationName:      e.location.String(),
		Base:              baseNewsProbability,
		CeilingMultiplier: ceilingMultiplier,
		TimeMultiplier:    timeMultiplier,
		WeekendMultiplier: weekendMultiplier,
		Final:             final,
		PityCounter:       pityCounter,
	}
}

func timeBasedMultiplier(now time.Time) float64 {
	h := now.Hour()
	if h >= 20 && h < 24 {
		return 2.0
	}
	if h >= 2 && h < 7 {
		return 0.5
	}
	return 1.0
}

func weekendHitMultiplier(now time.Time) float64 {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return 1.15
	}
	return 1.0
}

func (e *NewsEngine) pickWeightedEventInternal(now time.Time) EventType {
	weights := make(map[EventType]float64, len(NewsStories))
	total := 0.0
	for eventType := range NewsStories {
		w := 1.0
		if (eventType == EventMoon || eventType == EventCrash) && isWeekend(now) {
			w *= 1.5
		}
		weights[eventType] = w
		total += w
	}

	if total <= 0 {
		return EventHoliday
	}

	roll := e.rng.Float64() * total
	acc := 0.0
	for eventType, w := range weights {
		acc += w
		if roll <= acc {
			return eventType
		}
	}

	return EventHoliday
}

func isWeekend(now time.Time) bool {
	return now.Weekday() == time.Saturday || now.Weekday() == time.Sunday
}
