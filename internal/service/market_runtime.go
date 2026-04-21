package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/guild"
	"alt-bot/ent/marketstate"
	"alt-bot/ent/pricehistory"
)

const (
	marketStateSingletonID = 1
	priceHistoryRetention  = 24 * time.Hour
	impactScale            = 0.003

	tradeFeeBPS = int64(80)
	workTaxBPS  = int64(1000)

	feeBurnBPS    = int64(7000)
	feeReserveBPS = int64(2000)
	feeRevenueBPS = int64(1000)

	baseDailyIssuanceCap int64 = 20000

	circuitCautionThreshold = 0.05
	circuitAlertThreshold   = 0.10
	circuitHaltThreshold    = 0.18
	circuitRecoverWindow    = 30 * time.Minute
)

type CircuitLevel string

const (
	CircuitNormal  CircuitLevel = "NORMAL"
	CircuitCaution CircuitLevel = "CAUTION"
	CircuitAlert   CircuitLevel = "ALERT"
	CircuitHalt    CircuitLevel = "HALT"
)

type NewsTickResult struct {
	Hit         bool
	EventType   EventType
	Probability float64
	Price       float64
	Amount      int64
}

type RateSnapshot struct {
	CurrentPrice float64
	Change24h    float64
	Has24h       bool
	CurrentEvent EventType
	CircuitLevel CircuitLevel
	ReserveYen   int64
	RevenueYen   int64
	BurnedTotal  int64
	UpdatedAt    time.Time
	Comment      string
}

type PricePoint struct {
	Price     float64
	CreatedAt time.Time
}

type MarketHaltedError struct{}

func (e *MarketHaltedError) Error() string {
	return "market is halted"
}

type CircuitLimitError struct {
	Level  CircuitLevel
	MaxQty int64
}

func (e *CircuitLimitError) Error() string {
	return fmt.Sprintf("order size exceeds circuit limit: level=%s max=%d", e.Level, e.MaxQty)
}

type DailyIssuanceCapError struct {
	Cap    int64
	Issued int64
	Want   int64
}

func (e *DailyIssuanceCapError) Error() string {
	return fmt.Sprintf("daily issuance cap exceeded: cap=%d issued=%d want=%d", e.Cap, e.Issued, e.Want)
}

type feeSplit struct {
	Burn    int64
	Reserve int64
	Revenue int64
}

func splitFee(total int64) feeSplit {
	if total <= 0 {
		return feeSplit{}
	}
	burn := (total * feeBurnBPS) / 10000
	reserve := (total * feeReserveBPS) / 10000
	revenue := (total * feeRevenueBPS) / 10000
	remainder := total - burn - reserve - revenue
	burn += remainder
	return feeSplit{Burn: burn, Reserve: reserve, Revenue: revenue}
}

func normalizeCircuitLevel(raw string) CircuitLevel {
	lvl := CircuitLevel(raw)
	switch lvl {
	case CircuitNormal, CircuitCaution, CircuitAlert, CircuitHalt:
		return lvl
	default:
		return CircuitNormal
	}
}

func tradeLimitForLevel(level CircuitLevel) int64 {
	switch level {
	case CircuitCaution:
		return 5000
	case CircuitAlert:
		return 2500
	case CircuitHalt:
		return 0
	default:
		return 10000
	}
}

func effectiveDailyCap(eventType EventType) int64 {
	cap := baseDailyIssuanceCap
	switch eventType {
	case EventMoon, EventFOMO, EventGoldenCross:
		cap = int64(math.Round(float64(cap) * 1.2))
	case EventCrash, EventRegulation, EventDeflationShock:
		cap = int64(math.Round(float64(cap) * 0.9))
	}
	if cap < 5000 {
		cap = 5000
	}
	return cap
}

func normalizeDailyIssuanceFields(date string, issued int64, cap int64, now time.Time, eventType EventType, loc *time.Location) (string, int64, int64) {
	today := now.In(loc).Format("2006-01-02")
	nextCap := effectiveDailyCap(eventType)
	if date != today {
		return today, 0, nextCap
	}
	if cap <= 0 {
		cap = nextCap
	}
	return date, issued, cap
}

func evolveCircuitState(level CircuitLevel, oldPrice float64, newPrice float64, lastBreachAt *time.Time, now time.Time) (CircuitLevel, *time.Time, bool) {
	if oldPrice <= 0 {
		oldPrice = newPrice
	}
	change := 0.0
	if oldPrice > 0 {
		change = math.Abs((newPrice - oldPrice) / oldPrice)
	}

	next := level
	breach := false
	if change >= circuitHaltThreshold {
		next = CircuitHalt
		breach = true
	} else if change >= circuitAlertThreshold {
		next = CircuitAlert
		breach = true
	} else if change >= circuitCautionThreshold {
		next = CircuitCaution
		breach = true
	} else {
		next = CircuitNormal
	}

	if level == CircuitHalt && !breach {
		if lastBreachAt == nil {
			t := now
			return CircuitHalt, &t, false
		}
		if now.Sub(*lastBreachAt) >= circuitRecoverWindow {
			return CircuitNormal, nil, false
		}
		return CircuitHalt, lastBreachAt, false
	}

	if breach {
		t := now
		return next, &t, true
	}
	return next, nil, false
}

func (s *EconomyService) ensureMarketState(ctx context.Context) (*ent.MarketState, error) {
	state, err := s.client.MarketState.Get(ctx, marketStateSingletonID)
	if err == nil {
		return state, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to load market_state: %w", err)
	}

	now := time.Now().UTC()
	created, createErr := s.client.MarketState.Create().
		SetID(marketStateSingletonID).
		SetCurrentEvent("").
		SetEventRemainingTicks(0).
		SetPityCounter(0).
		SetCircuitLevel(string(CircuitNormal)).
		SetLastPrice(defaultALTPrice).
		SetReserveBalance(0).
		SetRevenueBalance(0).
		SetBurnedTotal(0).
		SetDailyIssuanceDate(now.In(s.marketLocation).Format("2006-01-02")).
		SetDailyIssued(0).
		SetDailyCap(baseDailyIssuanceCap).
		Save(ctx)
	if createErr != nil {
		if ent.IsConstraintError(createErr) {
			return s.client.MarketState.Get(ctx, marketStateSingletonID)
		}
		return nil, fmt.Errorf("failed to create market_state: %w", createErr)
	}

	if err := s.insertPriceHistory(ctx, created.LastPrice, now); err != nil {
		s.logger.Warn().Err(err).Msg("failed to seed price history")
	}
	return created, nil
}

func (s *EconomyService) lockMarketStateTx(ctx context.Context, tx *ent.Tx) (*ent.MarketState, error) {
	state, err := tx.MarketState.Query().
		Where(marketstate.IDEQ(marketStateSingletonID)).
		ForUpdate().
		Only(ctx)
	if err == nil {
		return state, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to lock market_state: %w", err)
	}

	now := time.Now().UTC()
	if _, createErr := tx.MarketState.Create().
		SetID(marketStateSingletonID).
		SetCurrentEvent("").
		SetEventRemainingTicks(0).
		SetPityCounter(0).
		SetCircuitLevel(string(CircuitNormal)).
		SetLastPrice(defaultALTPrice).
		SetReserveBalance(0).
		SetRevenueBalance(0).
		SetBurnedTotal(0).
		SetDailyIssuanceDate(now.In(s.marketLocation).Format("2006-01-02")).
		SetDailyIssued(0).
		SetDailyCap(baseDailyIssuanceCap).
		Save(ctx); createErr != nil && !ent.IsConstraintError(createErr) {
		return nil, fmt.Errorf("failed to create market_state in tx: %w", createErr)
	}

	state, err = tx.MarketState.Query().
		Where(marketstate.IDEQ(marketStateSingletonID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to re-lock market_state: %w", err)
	}
	return state, nil
}

func (s *EconomyService) setMarketStateTx(
	ctx context.Context,
	tx *ent.Tx,
	state *ent.MarketState,
	eventType EventType,
	remaining int,
	pity int,
	price float64,
	circuitLevel CircuitLevel,
	lastBreachAt *time.Time,
	reserveBalance int64,
	revenueBalance int64,
	burnedTotal int64,
	dailyDate string,
	dailyIssued int64,
	dailyCap int64,
	now time.Time,
) error {
	upd := tx.MarketState.UpdateOneID(state.ID).
		SetCurrentEvent(string(eventType)).
		SetEventRemainingTicks(remaining).
		SetPityCounter(pity).
		SetCircuitLevel(string(circuitLevel)).
		SetLastPrice(price).
		SetReserveBalance(reserveBalance).
		SetRevenueBalance(revenueBalance).
		SetBurnedTotal(burnedTotal).
		SetDailyIssuanceDate(dailyDate).
		SetDailyIssued(dailyIssued).
		SetDailyCap(dailyCap).
		SetUpdatedAt(now)
	if lastBreachAt != nil {
		upd.SetLastBreachAt(*lastBreachAt)
	} else {
		upd.ClearLastBreachAt()
	}
	if _, err := upd.Save(ctx); err != nil {
		return fmt.Errorf("failed to save market_state: %w", err)
	}

	s.logger.Info().
		Str("event", string(eventType)).
		Str("circuit_level", string(circuitLevel)).
		Int("remaining_ticks", remaining).
		Int("pity_counter", pity).
		Int64("reserve_balance", reserveBalance).
		Int64("revenue_balance", revenueBalance).
		Int64("burned_total", burnedTotal).
		Float64("price", price).
		Msg("MARKET_STATE_SAVED")
	return nil
}

func (s *EconomyService) insertPriceHistory(ctx context.Context, price float64, now time.Time) error {
	if _, err := s.client.PriceHistory.Create().SetPrice(price).SetCreatedAt(now).Save(ctx); err != nil {
		return fmt.Errorf("failed to insert price history: %w", err)
	}
	_, err := s.client.PriceHistory.Delete().Where(pricehistory.CreatedAtLT(now.Add(-priceHistoryRetention))).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to prune old price history: %w", err)
	}
	return nil
}

func (s *EconomyService) insertPriceHistoryTx(ctx context.Context, tx *ent.Tx, price float64, now time.Time) error {
	if _, err := tx.PriceHistory.Create().SetPrice(price).SetCreatedAt(now).Save(ctx); err != nil {
		return fmt.Errorf("failed to insert price history: %w", err)
	}
	if _, err := tx.PriceHistory.Delete().Where(pricehistory.CreatedAtLT(now.Add(-priceHistoryRetention))).Exec(ctx); err != nil {
		return fmt.Errorf("failed to prune old price history: %w", err)
	}
	return nil
}

func (s *EconomyService) eventMultiplier(eventType EventType) float64 {
	switch eventType {
	case EventCrash:
		return 0.82
	case EventMoon:
		return 1.20
	case EventHoliday:
		return 1.00
	case EventStagnation:
		return 0.995
	case EventWhaleAlert:
		return 1.08
	case EventBurnEvent:
		return 1.05
	case EventFOMO:
		return 1.10
	case EventInsiderLeak:
		return 1.06
	case EventBubble:
		return 1.12
	case EventShortSqueeze:
		return 1.09
	case EventRegulation:
		return 0.93
	case EventDeflationShock:
		return 0.96
	case EventGoldenCross:
		return 1.04
	default:
		return 1.0
	}
}

func (s *EconomyService) impactMultiplier(volume int64, side int) float64 {
	if volume <= 0 || side == 0 {
		return 1.0
	}
	impact := impactScale * math.Sqrt(float64(volume))
	if side < 0 {
		impact = -impact
	}
	factor := 1.0 + impact
	if factor < 0.2 {
		return 0.2
	}
	return factor
}

func (s *EconomyService) gbmMultiplier() float64 {
	z := s.rand.NormFloat64()
	mu := s.gbmMuPerTick
	sigma := s.gbmSigmaPerTick
	return math.Exp((mu - 0.5*sigma*sigma) + sigma*z)
}

func (s *EconomyService) modelPrice(base float64, eventType EventType, volume int64, side int) (float64, float64, float64, float64) {
	if base < minALTPrice {
		base = minALTPrice
	}
	gbm := s.gbmMultiplier()
	if eventType == "" {
		if gbm < s.passiveTickMin {
			gbm = s.passiveTickMin
		}
		if gbm > s.passiveTickMax {
			gbm = s.passiveTickMax
		}
	}
	eventMul := s.eventMultiplier(eventType)
	impact := s.impactMultiplier(volume, side)
	next := base * gbm * eventMul * impact
	if eventType == "" {
		reversion := 1.0 + s.passiveMeanReversion*((defaultALTPrice-base)/defaultALTPrice)
		if reversion < 0.97 {
			reversion = 0.97
		}
		if reversion > 1.03 {
			reversion = 1.03
		}
		next *= reversion
	}
	if next < minALTPrice {
		next = minALTPrice
	}
	return next, gbm, eventMul, impact
}

func (s *EconomyService) SetNewsChannel(ctx context.Context, guildID string, channelID string) error {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	row, err := s.client.Guild.Query().Where(guild.GuildID(guildID)).Only(ctx)
	if err == nil {
		if _, updateErr := s.client.Guild.UpdateOneID(row.ID).SetNewsChannelID(channelID).Save(ctx); updateErr != nil {
			return fmt.Errorf("failed to update guild channel: %w", updateErr)
		}
		return nil
	}
	if !ent.IsNotFound(err) {
		return fmt.Errorf("failed to query guild channel: %w", err)
	}

	if createErr := s.client.Guild.Create().SetGuildID(guildID).SetNewsChannelID(channelID).Exec(ctx); createErr != nil {
		return fmt.Errorf("failed to create guild channel: %w", createErr)
	}
	return nil
}

func (s *EconomyService) DisableNewsChannel(ctx context.Context, guildID string) error {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	if _, err := s.client.Guild.Delete().Where(guild.GuildID(guildID)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete guild channel: %w", err)
	}
	return nil
}

func (s *EconomyService) GetNewsChannel(ctx context.Context, guildID string) (string, bool, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	row, err := s.client.Guild.Query().Where(guild.GuildID(guildID)).Only(ctx)
	if err == nil {
		return row.NewsChannelID, true, nil
	}
	if ent.IsNotFound(err) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("failed to query guild channel: %w", err)
}

func (s *EconomyService) LoadNewsChannels(ctx context.Context) (map[string]string, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	rows, err := s.client.Guild.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load guild channels: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.NewsChannelID == "" {
			continue
		}
		out[row.GuildID] = row.NewsChannelID
	}
	return out, nil
}

func (s *EconomyService) ProcessNewsTick(ctx context.Context, now time.Time) (NewsTickResult, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	result := NewsTickResult{Hit: false, Amount: int64((now.UnixNano()%90001 + 10000))}
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		state, err := s.lockMarketStateTx(ctx, tx)
		if err != nil {
			return err
		}

		activeEvent := EventType(state.CurrentEvent)
		remaining := state.EventRemainingTicks
		pity := state.PityCounter
		reserveBalance := state.ReserveBalance
		revenueBalance := state.RevenueBalance
		burnedTotal := state.BurnedTotal
		dailyDate := state.DailyIssuanceDate
		dailyIssued := state.DailyIssued
		dailyCap := state.DailyCap
		circuitLevel := normalizeCircuitLevel(state.CircuitLevel)
		var lastBreachAt *time.Time
		if state.LastBreachAt != nil {
			t := *state.LastBreachAt
			lastBreachAt = &t
		}

		basePrice := state.LastPrice
		if basePrice < minALTPrice {
			basePrice = minALTPrice
		}
		probability := s.news.CurrentProbabilityDetails(now, pity).Final
		result.Probability = probability

		if activeEvent != "" && remaining > 0 {
			remaining--
			if remaining == 0 {
				s.logger.Info().Str("event", string(activeEvent)).Msg("EVENT_ENDED")
				if activeEvent == EventBubble {
					activeEvent = EventCrash
					remaining = eventDurationTicks(activeEvent)
					s.logger.Info().Str("event", string(activeEvent)).Msg("EVENT_TRIGGERED")
				} else {
					activeEvent = ""
				}
			}
		} else {
			eventType, p, hit := s.news.RollEvent(now, pity)
			result.Probability = p
			if hit {
				activeEvent = eventType
				remaining = eventDurationTicks(eventType)
				pity = 0
				result.Hit = true
				result.EventType = eventType
				s.logger.Info().Str("event", string(eventType)).Msg("EVENT_TRIGGERED")
			} else {
				pity++
			}
		}

		nextPrice, gbm, evtMul, impact := s.modelPrice(basePrice, activeEvent, 1, 0)
		nextCircuit, nextBreach, breached := evolveCircuitState(circuitLevel, basePrice, nextPrice, lastBreachAt, now)
		if breached {
			s.logger.Warn().Str("level", string(nextCircuit)).Float64("change", math.Abs((nextPrice-basePrice)/basePrice)).Msg("CIRCUIT_ESCALATED")
		} else if circuitLevel == CircuitHalt && nextCircuit == CircuitNormal {
			s.logger.Info().Msg("CIRCUIT_RECOVERED")
		}

		result.Price = nextPrice
		if result.EventType == "" {
			result.EventType = activeEvent
		}
		s.logger.Info().Float64("price_before", basePrice).Float64("gbm", gbm).Float64("event_multiplier", evtMul).Float64("impact", impact).Float64("price_after", nextPrice).Msg("PRICE_UPDATED")

		if err := s.insertPriceHistoryTx(ctx, tx, nextPrice, now); err != nil {
			return err
		}

		dailyDate, dailyIssued, dailyCap = normalizeDailyIssuanceFields(dailyDate, dailyIssued, dailyCap, now, activeEvent, s.marketLocation)
		if err := s.setMarketStateTx(ctx, tx, state, activeEvent, remaining, pity, nextPrice, nextCircuit, nextBreach, reserveBalance, revenueBalance, burnedTotal, dailyDate, dailyIssued, dailyCap, now); err != nil {
			return err
		}
		s.altPrice = nextPrice
		if result.EventType != "" {
			result.Hit = true
		}
		return nil
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("news tick tx failed")
		return NewsTickResult{}, err
	}
	return result, nil
}

func (s *EconomyService) GetRateSnapshot(ctx context.Context) (RateSnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	state, err := s.ensureMarketState(ctx)
	if err != nil {
		return RateSnapshot{}, err
	}

	since := time.Now().Add(-24 * time.Hour)
	base, err := s.client.PriceHistory.Query().Where(pricehistory.CreatedAtGTE(since)).Order(ent.Asc(pricehistory.FieldCreatedAt)).First(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return RateSnapshot{}, fmt.Errorf("failed to load 24h base price: %w", err)
		}
		base = nil
	}

	snap := RateSnapshot{
		CurrentPrice: state.LastPrice,
		CurrentEvent: EventType(state.CurrentEvent),
		CircuitLevel: normalizeCircuitLevel(state.CircuitLevel),
		ReserveYen:   state.ReserveBalance,
		RevenueYen:   state.RevenueBalance,
		BurnedTotal:  state.BurnedTotal,
		UpdatedAt:    state.UpdatedAt,
		Comment:      marketComment(EventType(state.CurrentEvent), 0, false, normalizeCircuitLevel(state.CircuitLevel)),
	}
	if base != nil && base.Price > 0 {
		snap.Has24h = true
		snap.Change24h = ((state.LastPrice - base.Price) / base.Price) * 100
		snap.Comment = marketComment(EventType(state.CurrentEvent), snap.Change24h, true, normalizeCircuitLevel(state.CircuitLevel))
	}
	return snap, nil
}

func (s *EconomyService) GetPriceHistory(ctx context.Context, limit int) ([]PricePoint, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	rows, err := s.client.PriceHistory.Query().Order(ent.Desc(pricehistory.FieldCreatedAt)).Limit(limit).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load price history: %w", err)
	}
	out := make([]PricePoint, len(rows))
	for i := range rows {
		out[i] = PricePoint{Price: rows[i].Price, CreatedAt: rows[i].CreatedAt}
	}
	return out, nil
}

func eventDurationTicks(eventType EventType) int {
	switch eventType {
	case EventBubble:
		return 6
	case EventShortSqueeze:
		return 3
	case EventStagnation:
		return 5
	default:
		return 1
	}
}

func marketComment(eventType EventType, change24h float64, has24h bool, circuit CircuitLevel) string {
	if circuit == CircuitHalt {
		return "市場は緊急停止中です。30分安定後に自動解除されます。"
	}
	if circuit == CircuitAlert {
		return "市場は警戒レベルです。注文上限が強く制限されています。"
	}
	if circuit == CircuitCaution {
		return "市場は注意レベルです。注文上限が制限されています。"
	}
	if eventType != "" {
		switch eventType {
		case EventCrash:
			return "弱気トレンド。防御的なポジションが推奨です。"
		case EventMoon:
			return "強気トレンド。ボラティリティ上昇に注意してください。"
		case EventBubble:
			return "過熱相場。急反転リスクが高まっています。"
		default:
			return "イベント主導相場。短期変動に注意してください。"
		}
	}
	if !has24h {
		return "24h履歴が不足しているため評価保留です。"
	}
	if change24h >= 5 {
		return "上昇基調。利益確定ラインの管理が重要です。"
	}
	if change24h <= -5 {
		return "下落基調。リスク管理を優先してください。"
	}
	return "レンジ相場。ニュース起因の急変に注意してください。"
}
