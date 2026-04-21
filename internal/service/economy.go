package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"text/template"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/marketstate"
	"alt-bot/ent/transactionlog"
	"alt-bot/ent/user"
	"alt-bot/internal/config"

	"github.com/disgoorg/disgo/discord"
	"github.com/rs/zerolog"
)

type WorkDifficulty string

const (
	WorkDifficultyEasy WorkDifficulty = "easy"
	WorkDifficultyHard WorkDifficulty = "hard"
)

type CooldownError struct {
	Until time.Time
}

func (e *CooldownError) Error() string {
	return fmt.Sprintf("work is on cooldown until %s", e.Until.UTC().Format(time.RFC3339))
}

func (e *CooldownError) Remaining(now time.Time) time.Duration {
	if !e.Until.After(now) {
		return 0
	}
	return e.Until.Sub(now)
}

type WorkResult struct {
	Difficulty WorkDifficulty
	YenReward  int64
	XP         int64
	YenBalance int64
	AltBalance int64
	NextWorkAt time.Time
}

type BuyCryptoResult struct {
	Amount       int64
	SettledPrice float64
	TotalCostYen int64
	YenBalance   int64
	AltBalance   int64
	CurrentPrice float64
}

type SellCryptoResult struct {
	Amount          int64
	SettledPrice    float64
	TotalRevenueYen int64
	YenBalance      int64
	AltBalance      int64
	CurrentPrice    float64
}

type InsufficientYenError struct {
	Need int64
	Have int64
}

func (e *InsufficientYenError) Error() string {
	return fmt.Sprintf("insufficient yen: need=%d have=%d", e.Need, e.Have)
}

type InsufficientALTError struct {
	Need int64
	Have int64
}

func (e *InsufficientALTError) Error() string {
	return fmt.Sprintf("insufficient alt: need=%d have=%d", e.Need, e.Have)
}

type workRule struct {
	minReward int64
	maxReward int64
	xpGain    int64
	cooldown  time.Duration
}

type EconomyService struct {
	client *ent.Client
	logger zerolog.Logger
	rules  map[WorkDifficulty]workRule
	rand   *rand.Rand
	news   *NewsEngine

	mu        sync.Mutex
	altPrice  float64
	impactK   float64
	prevHash  string
	publicKey ed25519.PublicKey
	signer    ed25519.PrivateKey

	marketLocation       *time.Location
	gbmMuPerTick         float64
	gbmSigmaPerTick      float64
	passiveTickMin       float64
	passiveTickMax       float64
	passiveMeanReversion float64
}

const serviceTimeout = 5 * time.Second

const (
	CurrencyYenName = "Yen"
	CurrencyYenUnit = "¥"
	CurrencyALTName = "ALToken"
	CurrencyALTUnit = "ALT"

	defaultALTPrice = 100.0
	defaultImpactK  = 2.5
	minALTPrice     = 1.0
)

func withServiceTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, serviceTimeout)
}

func NewEconomyService(client *ent.Client, cfg config.Config, logger zerolog.Logger) (*EconomyService, error) {
	pub, priv, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 keypair: %w", err)
	}

	prevHash := ""
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	last, qerr := client.TransactionLog.Query().
		Order(ent.Desc(transactionlog.FieldCreatedAt)).
		First(ctx)
	if qerr == nil {
		prevHash = last.Hash
	} else if !ent.IsNotFound(qerr) {
		logger.Warn().Err(qerr).Msg("failed to load latest transaction hash; starting new chain")
	}

	loc := loadTimeLocation(cfg.TimeZone)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	newsRng := rand.New(rand.NewSource(time.Now().UnixNano() + 991))
	gbmMu := clampFloat64(cfg.MarketGBMMu, 0.00002, -0.01, 0.01)
	gbmSigma := clampFloat64(cfg.MarketGBMSigma, 0.003, 0.0001, 0.05)
	passiveMin := clampFloat64(cfg.MarketPassiveMin, 0.996, 0.90, 1.0)
	passiveMax := clampFloat64(cfg.MarketPassiveMax, 1.004, 1.0, 1.10)
	if passiveMin > passiveMax {
		passiveMin = 0.996
		passiveMax = 1.004
	}
	meanReversion := clampFloat64(cfg.MarketMeanReversion, 0.18, 0.0, 1.0)

	svc := &EconomyService{
		client: client,
		logger: logger,
		rules: map[WorkDifficulty]workRule{
			WorkDifficultyEasy: {
				minReward: 30,
				maxReward: 80,
				xpGain:    10,
				cooldown:  45 * time.Second,
			},
			WorkDifficultyHard: {
				minReward: 120,
				maxReward: 260,
				xpGain:    25,
				cooldown:  2 * time.Minute,
			},
		},
		rand:                 rng,
		news:                 newNewsEngine(newsRng, loc),
		altPrice:             defaultALTPrice,
		impactK:              defaultImpactK,
		prevHash:             prevHash,
		publicKey:            pub,
		signer:               priv,
		marketLocation:       loc,
		gbmMuPerTick:         gbmMu,
		gbmSigmaPerTick:      gbmSigma,
		passiveTickMin:       passiveMin,
		passiveTickMax:       passiveMax,
		passiveMeanReversion: meanReversion,
	}

	marketCtx, marketCancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer marketCancel()
	state, stateErr := svc.ensureMarketState(marketCtx)
	if stateErr != nil {
		logger.Warn().Err(stateErr).Msg("failed to initialize market state; using defaults")
	} else {
		if state.LastPrice >= minALTPrice {
			svc.altPrice = state.LastPrice
		}
		if state.CurrentEvent != "" {
			logger.Info().
				Str("current_event", state.CurrentEvent).
				Int("remaining_ticks", state.EventRemainingTicks).
				Int("pity_counter", state.PityCounter).
				Msg("market state restored")
		}
	}

	return svc, nil
}

func loadTimeLocation(name string) *time.Location {
	zone := name
	if strings.TrimSpace(zone) == "" {
		zone = "Asia/Tokyo"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}

func clampFloat64(v float64, fallback float64, min float64, max float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (s *EconomyService) CurrentALTPrice() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.altPrice
}

func (s *EconomyService) EnsureUser(ctx context.Context, discordID string) (*ent.User, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	found, err := s.client.User.Query().
		Where(user.DiscordID(discordID)).
		Only(ctx)
	if err == nil {
		return found, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	created, createErr := s.client.User.Create().
		SetDiscordID(discordID).
		SetBalance(0).
		SetCryptoBalance(0).
		SetXp(0).
		SetWorkEndAt(time.Unix(0, 0).UTC()).
		Save(ctx)
	if createErr != nil {
		var constraintErr interface{ ConstraintName() string }
		if errors.As(createErr, &constraintErr) {
			return s.client.User.Query().Where(user.DiscordID(discordID)).Only(ctx)
		}
		return nil, fmt.Errorf("failed to create user: %w", createErr)
	}
	return created, nil
}

func (s *EconomyService) Work(ctx context.Context, discordID string, difficulty WorkDifficulty) (WorkResult, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, ok := s.rules[difficulty]
	if !ok {
		return WorkResult{}, fmt.Errorf("invalid difficulty: %s", difficulty)
	}

	now := time.Now().UTC()

	var result WorkResult
	var nextHash string
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

		u, err := tx.User.Query().
			Where(user.DiscordID(discordID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				u, err = tx.User.Create().
					SetDiscordID(discordID).
					SetBalance(0).
					SetCryptoBalance(0).
					SetXp(0).
					SetWorkEndAt(time.Unix(0, 0).UTC()).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to auto-create user in work: %w", err)
				}
			} else {
				return fmt.Errorf("failed to read user in work: %w", err)
			}
		}

		if u.WorkEndAt.After(now) {
			return &CooldownError{Until: u.WorkEndAt}
		}

		reward := rule.minReward
		if rule.maxReward > rule.minReward {
			reward += s.rand.Int63n(rule.maxReward - rule.minReward + 1)
		}
		tax := (reward * workTaxBPS) / 10000
		netReward := reward - tax
		dailyDate, dailyIssued, dailyCap = normalizeDailyIssuanceFields(dailyDate, dailyIssued, dailyCap, now, activeEvent, s.marketLocation)
		if dailyIssued+netReward > dailyCap {
			return &DailyIssuanceCapError{Cap: dailyCap, Issued: dailyIssued, Want: netReward}
		}
		fees := splitFee(tax)
		reserveBalance += fees.Reserve
		revenueBalance += fees.Revenue
		burnedTotal += fees.Burn
		dailyIssued += netReward
		s.logger.Info().Int64("total_fee", tax).Int64("burn", fees.Burn).Int64("reserve", fees.Reserve).Int64("revenue", fees.Revenue).Str("kind", "work_tax").Msg("FEE_ALLOCATED")

		nextEnd := now.Add(rule.cooldown)
		newBalance := u.Balance + netReward
		newXP := u.Xp + rule.xpGain

		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			SetXp(newXP).
			SetWorkEndAt(nextEnd).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in work: %w", err)
		}

		nextPrice, gbm, evtMul, impact := s.modelPrice(state.LastPrice, activeEvent, 0, 0)
		nextCircuit, nextBreach, breached := evolveCircuitState(circuitLevel, state.LastPrice, nextPrice, lastBreachAt, now)
		if breached {
			s.logger.Warn().Str("level", string(nextCircuit)).Msg("CIRCUIT_ESCALATED")
		} else if circuitLevel == CircuitHalt && nextCircuit == CircuitNormal {
			s.logger.Info().Msg("CIRCUIT_RECOVERED")
		}
		nowTick := time.Now().UTC()
		if err = s.insertPriceHistoryTx(ctx, tx, nextPrice, nowTick); err != nil {
			return err
		}
		if err = s.setMarketStateTx(ctx, tx, state, activeEvent, remaining, pity, nextPrice, nextCircuit, nextBreach, reserveBalance, revenueBalance, burnedTotal, dailyDate, dailyIssued, dailyCap, nowTick); err != nil {
			return err
		}
		s.logger.Info().
			Float64("price_before", state.LastPrice).
			Float64("gbm", gbm).
			Float64("event_multiplier", evtMul).
			Float64("impact", impact).
			Float64("price_after", nextPrice).
			Str("kind", "work").
			Msg("PRICE_UPDATED")

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "work",
			YenDelta:     netReward,
			ALTDelta:     0,
			XPDelta:      rule.xpGain,
			Amount:       0,
			SettledPrice: state.LastPrice,
			PriceAfter:   nextPrice,
			BalanceAfter: newBalance,
			ALTAfter:     u.CryptoBalance,
		})
		if err != nil {
			return err
		}

		result = WorkResult{
			Difficulty: difficulty,
			YenReward:  netReward,
			XP:         rule.xpGain,
			YenBalance: newBalance,
			AltBalance: u.CryptoBalance,
			NextWorkAt: nextEnd,
		}
		s.altPrice = nextPrice
		return nil
	})
	if err != nil {
		return WorkResult{}, err
	}
	s.prevHash = nextHash
	return result, nil
}

func (s *EconomyService) BuyCrypto(ctx context.Context, discordID string, amount int64) (BuyCryptoResult, error) {
	if amount <= 0 {
		return BuyCryptoResult{}, fmt.Errorf("amount must be positive")
	}
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	var result BuyCryptoResult
	var nextHash string
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

		if circuitLevel == CircuitHalt {
			return &MarketHaltedError{}
		}
		maxQty := tradeLimitForLevel(circuitLevel)
		if amount > maxQty {
			return &CircuitLimitError{Level: circuitLevel, MaxQty: maxQty}
		}

		settledPrice := state.LastPrice
		grossCost := int64(math.Ceil(settledPrice * float64(amount)))
		fee := (grossCost * tradeFeeBPS) / 10000
		totalCost := grossCost + fee
		fees := splitFee(fee)
		priceAfter, gbm, evtMul, impact := s.modelPrice(settledPrice, EventType(state.CurrentEvent), amount, 1)

		u, err := tx.User.Query().
			Where(user.DiscordID(discordID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				u, err = tx.User.Create().
					SetDiscordID(discordID).
					SetBalance(0).
					SetCryptoBalance(0).
					SetXp(0).
					SetWorkEndAt(time.Unix(0, 0).UTC()).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create user in buy: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load user in buy: %w", err)
			}
		}

		if u.Balance < totalCost {
			return &InsufficientYenError{Need: totalCost, Have: u.Balance}
		}

		newBalance := u.Balance - totalCost
		newALT := u.CryptoBalance + amount

		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			SetCryptoBalance(newALT).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in buy: %w", err)
		}
		reserveBalance += fees.Reserve
		revenueBalance += fees.Revenue
		burnedTotal += fees.Burn
		s.logger.Info().Int64("total_fee", fee).Int64("burn", fees.Burn).Int64("reserve", fees.Reserve).Int64("revenue", fees.Revenue).Str("kind", "trade_buy").Msg("FEE_ALLOCATED")

		nowTick := time.Now().UTC()
		dailyDate, dailyIssued, dailyCap = normalizeDailyIssuanceFields(dailyDate, dailyIssued, dailyCap, nowTick, activeEvent, s.marketLocation)
		nextCircuit, nextBreach, breached := evolveCircuitState(circuitLevel, settledPrice, priceAfter, lastBreachAt, nowTick)
		if breached {
			s.logger.Warn().Str("level", string(nextCircuit)).Msg("CIRCUIT_ESCALATED")
		} else if circuitLevel == CircuitHalt && nextCircuit == CircuitNormal {
			s.logger.Info().Msg("CIRCUIT_RECOVERED")
		}
		if err = s.insertPriceHistoryTx(ctx, tx, priceAfter, nowTick); err != nil {
			return err
		}
		if err = s.setMarketStateTx(ctx, tx, state, activeEvent, remaining, pity, priceAfter, nextCircuit, nextBreach, reserveBalance, revenueBalance, burnedTotal, dailyDate, dailyIssued, dailyCap, nowTick); err != nil {
			return err
		}
		s.logger.Info().
			Float64("price_before", settledPrice).
			Float64("gbm", gbm).
			Float64("event_multiplier", evtMul).
			Float64("impact", impact).
			Float64("price_after", priceAfter).
			Str("kind", "buy").
			Int64("volume", amount).
			Msg("PRICE_UPDATED")

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "buy",
			YenDelta:     -totalCost,
			ALTDelta:     amount,
			XPDelta:      0,
			Amount:       amount,
			SettledPrice: settledPrice,
			PriceAfter:   priceAfter,
			BalanceAfter: newBalance,
			ALTAfter:     newALT,
		})
		if err != nil {
			return err
		}

		result = BuyCryptoResult{
			Amount:       amount,
			SettledPrice: settledPrice,
			TotalCostYen: totalCost,
			YenBalance:   newBalance,
			AltBalance:   newALT,
			CurrentPrice: priceAfter,
		}
		s.logger.Info().
			Str("kind", "buy").
			Str("user_id", discordID).
			Int64("amount", amount).
			Int64("fee", fee).
			Float64("settled_price", settledPrice).
			Float64("price_after", priceAfter).
			Msg("TRADE_EXECUTED")
		s.altPrice = priceAfter
		return nil
	})
	if err != nil {
		return BuyCryptoResult{}, err
	}

	s.prevHash = nextHash
	return result, nil
}

func (s *EconomyService) SellCrypto(ctx context.Context, discordID string, amount int64) (SellCryptoResult, error) {
	if amount <= 0 {
		return SellCryptoResult{}, fmt.Errorf("amount must be positive")
	}
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	var result SellCryptoResult
	var nextHash string
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

		if circuitLevel == CircuitHalt {
			return &MarketHaltedError{}
		}
		maxQty := tradeLimitForLevel(circuitLevel)
		if amount > maxQty {
			return &CircuitLimitError{Level: circuitLevel, MaxQty: maxQty}
		}

		settledPrice := state.LastPrice
		grossRevenue := int64(math.Floor(settledPrice * float64(amount)))
		fee := (grossRevenue * tradeFeeBPS) / 10000
		totalRevenue := grossRevenue - fee
		if totalRevenue < 0 {
			totalRevenue = 0
		}
		fees := splitFee(fee)
		priceAfter, gbm, evtMul, impact := s.modelPrice(settledPrice, EventType(state.CurrentEvent), amount, -1)

		u, err := tx.User.Query().
			Where(user.DiscordID(discordID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return &InsufficientALTError{Need: amount, Have: 0}
			}
			return fmt.Errorf("failed to load user in sell: %w", err)
		}

		if u.CryptoBalance < amount {
			return &InsufficientALTError{Need: amount, Have: u.CryptoBalance}
		}

		newBalance := u.Balance + totalRevenue
		newALT := u.CryptoBalance - amount

		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			SetCryptoBalance(newALT).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in sell: %w", err)
		}
		reserveBalance += fees.Reserve
		revenueBalance += fees.Revenue
		burnedTotal += fees.Burn
		s.logger.Info().Int64("total_fee", fee).Int64("burn", fees.Burn).Int64("reserve", fees.Reserve).Int64("revenue", fees.Revenue).Str("kind", "trade_sell").Msg("FEE_ALLOCATED")

		nowTick := time.Now().UTC()
		dailyDate, dailyIssued, dailyCap = normalizeDailyIssuanceFields(dailyDate, dailyIssued, dailyCap, nowTick, activeEvent, s.marketLocation)
		nextCircuit, nextBreach, breached := evolveCircuitState(circuitLevel, settledPrice, priceAfter, lastBreachAt, nowTick)
		if breached {
			s.logger.Warn().Str("level", string(nextCircuit)).Msg("CIRCUIT_ESCALATED")
		} else if circuitLevel == CircuitHalt && nextCircuit == CircuitNormal {
			s.logger.Info().Msg("CIRCUIT_RECOVERED")
		}
		if err = s.insertPriceHistoryTx(ctx, tx, priceAfter, nowTick); err != nil {
			return err
		}
		if err = s.setMarketStateTx(ctx, tx, state, activeEvent, remaining, pity, priceAfter, nextCircuit, nextBreach, reserveBalance, revenueBalance, burnedTotal, dailyDate, dailyIssued, dailyCap, nowTick); err != nil {
			return err
		}
		s.logger.Info().
			Float64("price_before", settledPrice).
			Float64("gbm", gbm).
			Float64("event_multiplier", evtMul).
			Float64("impact", impact).
			Float64("price_after", priceAfter).
			Str("kind", "sell").
			Int64("volume", amount).
			Msg("PRICE_UPDATED")

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "sell",
			YenDelta:     totalRevenue,
			ALTDelta:     -amount,
			XPDelta:      0,
			Amount:       amount,
			SettledPrice: settledPrice,
			PriceAfter:   priceAfter,
			BalanceAfter: newBalance,
			ALTAfter:     newALT,
		})
		if err != nil {
			return err
		}

		result = SellCryptoResult{
			Amount:          amount,
			SettledPrice:    settledPrice,
			TotalRevenueYen: totalRevenue,
			YenBalance:      newBalance,
			AltBalance:      newALT,
			CurrentPrice:    priceAfter,
		}
		s.logger.Info().
			Str("kind", "sell").
			Str("user_id", discordID).
			Int64("amount", amount).
			Int64("fee", fee).
			Float64("settled_price", settledPrice).
			Float64("price_after", priceAfter).
			Msg("TRADE_EXECUTED")
		s.altPrice = priceAfter
		return nil
	})
	if err != nil {
		return SellCryptoResult{}, err
	}

	s.prevHash = nextHash
	return result, nil
}

type txLogInput struct {
	DiscordID    string
	Kind         string
	YenDelta     int64
	ALTDelta     int64
	XPDelta      int64
	Amount       int64
	SettledPrice float64
	PriceAfter   float64
	BalanceAfter int64
	ALTAfter     int64
}

func (s *EconomyService) appendSignedLog(ctx context.Context, tx *ent.Tx, in txLogInput) (string, error) {
	txID, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate tx id: %w", err)
	}

	payload := fmt.Sprintf(
		"%s|%s|%s|%d|%d|%d|%d|%.8f|%.8f|%d|%d",
		s.prevHash,
		txID,
		in.Kind,
		in.YenDelta,
		in.ALTDelta,
		in.XPDelta,
		in.Amount,
		in.SettledPrice,
		in.PriceAfter,
		in.BalanceAfter,
		in.ALTAfter,
	)

	hash := sha256.Sum256([]byte(payload))
	hashHex := hex.EncodeToString(hash[:])
	signature := ed25519.Sign(s.signer, hash[:])

	if _, err = tx.TransactionLog.Create().
		SetTxID(txID).
		SetUserDiscordID(in.DiscordID).
		SetKind(in.Kind).
		SetYenDelta(in.YenDelta).
		SetAltDelta(in.ALTDelta).
		SetXpDelta(in.XPDelta).
		SetAmount(in.Amount).
		SetSettledPrice(in.SettledPrice).
		SetPriceAfter(in.PriceAfter).
		SetBalanceAfter(in.BalanceAfter).
		SetCryptoAfter(in.ALTAfter).
		SetPrevHash(s.prevHash).
		SetHash(hashHex).
		SetSignature(hex.EncodeToString(signature)).
		SetPublicKey(hex.EncodeToString(s.publicKey)).
		Save(ctx); err != nil {
		return "", fmt.Errorf("failed to persist transaction log: %w", err)
	}

	return hashHex, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type NewsTemplateData struct {
	Amount int64
	Price  float64
}

func (s *EconomyService) RandomEventType() EventType {
	return s.news.pickWeightedEvent(s.news.now())
}

func (s *EconomyService) RollNewsEvent(now time.Time) (EventType, float64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	state, err := s.client.MarketState.Query().Where(marketstate.IDEQ(marketStateSingletonID)).Only(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to load market state for roll")
		return s.news.RollEvent(now, 0)
	}
	return s.news.RollEvent(now, state.PityCounter)
}

func (s *EconomyService) CurrentNewsProbability(now time.Time) ProbabilityDetails {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()
	state, err := s.client.MarketState.Query().Where(marketstate.IDEQ(marketStateSingletonID)).Only(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to load market state for probability")
		return s.news.CurrentProbabilityDetails(now, 0)
	}
	return s.news.CurrentProbabilityDetails(now, state.PityCounter)
}

func (s *EconomyService) BuildNewsEmbed(eventType EventType, data NewsTemplateData) (discord.Embed, error) {
	story, ok := NewsStories[eventType]
	if !ok {
		return discord.Embed{}, fmt.Errorf("unknown event type: %s", eventType)
	}

	text, err := renderNewsTemplate(story.Description, data)
	if err != nil {
		return discord.Embed{}, fmt.Errorf("failed to render story template: %w", err)
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(story.Title).
		SetDescription(text).
		SetColor(story.Color).
		AddField("イベントID", string(eventType), true).
		AddField("影響度", story.ImpactLevel, true).
		SetTimestamp(time.Now()).
		Build()

	return embed, nil
}

func renderNewsTemplate(tmpl string, data NewsTemplateData) (string, error) {
	t, err := template.New("news_story").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}

	return b.String(), nil
}
