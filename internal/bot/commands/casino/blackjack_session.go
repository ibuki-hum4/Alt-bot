package casino

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rs/zerolog/log"
)

const (
	blackjackDecks          = 6
	blackjackInitialCards   = 2
	blackjackAceValue       = 11
	blackjackTimeout        = casinoInteractionTimeout
	blackjackImageFileName   = "blackjack-state.png"
	blackjackImageAttachment = "attachment://blackjack-state.png"
	blackjackMaxSplitHands  = 2
	blackjackDealerStandSoft = 17
)

type blackjackCard struct {
	Value int
}

type blackjackHand struct {
	Cards   []blackjackCard
	Bet     int64
	Done    bool
	Busted  bool
	Stand   bool
	Natural bool
}

type blackjackSession struct {
	ID             string
	UserID         string
	GuildID        string
	BaseBet        int64
	Expires        int64
	Deck           []blackjackCard
	Dealer         []blackjackCard
	Hands          []blackjackHand
	ActiveHand     int
	SplitUsed      bool
	Closed         bool
	BalanceAfter   int64
	DealersRevealed bool
	FinalReason    string
}

type blackjackRenderHand struct {
	Title   string
	Cards   []blackjackCard
	Bet     int64
	Total   int
	Soft    bool
	Active  bool
	Done    bool
	Busted  bool
	Stand   bool
	Natural bool
}

type blackjackRenderState struct {
	Title         string
	Subtitle      string
	Dealer        []blackjackCard
	RevealDealer  bool
	Hands         []blackjackRenderHand
	ActiveHand    int
	Balance       int64
	BaseBet       int64
	CurrentAction string
	Footer        string
	SessionID     string
	Closed        bool
}

var (
	blackjackMu       sync.Mutex
	blackjackSessions = map[string]*blackjackSession{}
	blackjackRand     = rand.New(rand.NewSource(time.Now().UnixNano() + 31337))
)

func startBlackjackSession(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	bet := int64(event.SlashCommandInteractionData().Int("amount"))
	if bet <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("amount は 1 以上で指定してください。例: /casino blackjack amount:100").
			SetEphemeral(true).
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	balance, err := economy.BlackjackPlaceBet(ctx, event.User().ID.String(), bet)
	if err != nil {
		var insufficient *service.InsufficientYenError
		if errors.As(err, &insufficient) {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(fmt.Sprintf("Yen不足です。必要: %d %s / 現在: %d %s", insufficient.Need, service.CurrencyYenUnit, insufficient.Have, service.CurrencyYenUnit)).
				SetEphemeral(true).
				Build())
			return
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("blackjack の開始に失敗しました。少し待って再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	session := &blackjackSession{
		ID:           newBlackjackSessionID(),
		UserID:       event.User().ID.String(),
		GuildID:      guildID.String(),
		BaseBet:      bet,
		Expires:      time.Now().Add(blackjackTimeout).Unix(),
		Deck:         buildBlackjackDeck(),
		Dealer:       nil,
		Hands:        []blackjackHand{{Bet: bet}},
		ActiveHand:   0,
		SplitUsed:    false,
		Closed:       false,
		BalanceAfter: balance,
	}
	blackjackDealInitial(session)

	blackjackMu.Lock()
	blackjackSessions[session.ID] = session
	blackjackMu.Unlock()

	if blackjackPlayerHasNaturalBlackjack(session) || blackjackSessionHasDealerBlackjack(session) {
		settled, err := settleBlackjack(ctx, economy, session)
		if err != nil {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("blackjack の精算に失敗しました。少し待って再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		sendBlackjackInitialMessage(event, blackjackSnapshot(settled, true, "start", settled.FinalReason), true)
		return
	}

	sendBlackjackInitialMessage(event, blackjackSnapshot(session, false, "start", blackjackActionMessage(session)), false)
}

func HandleBlackjackComponent(economy *service.EconomyService, event *events.ComponentInteractionCreate) {
	parts := strings.Split(event.Data.CustomID(), ":")
	if len(parts) != 3 || parts[0] != "blackjack" {
		return
	}
	action := parts[1]
	sessionID := parts[2]

	blackjackMu.Lock()
	session, ok := blackjackSessions[sessionID]
	if !ok {
		blackjackMu.Unlock()
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("セッションが見つかりません。再度 /casino blackjack を実行してください。").
			SetEphemeral(true).
			Build())
		return
	}
	if session.UserID != event.User().ID.String() {
		blackjackMu.Unlock()
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このボタンはコマンド実行者のみ操作できます。").
			SetEphemeral(true).
			Build())
		return
	}
	if session.Closed || time.Now().Unix() > session.Expires {
		delete(blackjackSessions, sessionID)
		blackjackMu.Unlock()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(discord.NewEmbedBuilder().
				SetTitle("Blackjack 終了").
				SetDescription("セッションが終了しました。再度 /casino blackjack を実行してください。").
				SetColor(0x95A5A6).
				Build()).
			ClearContainerComponents().
			Build())
		return
	}
	blackjackMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch action {
	case "hit":
		if err := blackjackHit(session); err != nil {
			respondBlackjackError(event, err.Error())
			return
		}
		resolveBlackjackIfNeeded(ctx, economy, event, session, "hit")
	case "stand":
		blackjackMu.Lock()
		if err := blackjackStandLocked(session); err != nil {
			blackjackMu.Unlock()
			respondBlackjackError(event, err.Error())
			return
		}
		blackjackMu.Unlock()
		resolveBlackjackIfNeeded(ctx, economy, event, session, "stand")
	case "double":
		if err := blackjackDouble(ctx, economy, session); err != nil {
			respondBlackjackError(event, err.Error())
			return
		}
		resolveBlackjackIfNeeded(ctx, economy, event, session, "double")
	case "split":
		if err := blackjackSplit(ctx, economy, session); err != nil {
			respondBlackjackError(event, err.Error())
			return
		}
		resolveBlackjackIfNeeded(ctx, economy, event, session, "split")
	default:
		return
	}
}

func respondBlackjackError(event *events.ComponentInteractionCreate, message string) {
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(message).
		SetEphemeral(true).
		Build())
}

func sendBlackjackInitialMessage(event *events.ApplicationCommandInteractionCreate, view blackjackRenderState, closed bool) {
	embed := buildBlackjackEmbed(view, closed)
	components := buildBlackjackComponents(view, closed)
	if err := event.DeferCreateMessage(true); err != nil {
		log.Error().
			Err(err).
			Str("game", "blackjack").
			Str("phase", "defer_create").
			Str("session_id", view.SessionID).
			Msg("failed to defer blackjack response")
		if err := event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(*embed).
			SetContainerComponents(components...).
			SetEphemeral(true).
			Build()); err != nil {
			log.Error().
				Err(err).
				Str("game", "blackjack").
				Str("phase", "send_base").
				Str("session_id", view.SessionID).
				Msg("failed to send blackjack message")
		}
		return
	}

	update := discord.NewMessageUpdateBuilder().
		SetEmbeds(*embed).
		SetContainerComponents(components...)
	if pngBytes, err := renderBlackjackStatePNG(view); err == nil {
		imageEmbed := discord.NewEmbedBuilder().SetImage(blackjackImageAttachment).Build()
		update = update.
			SetEmbeds(*embed, imageEmbed).
			AddFile(blackjackImageFileName, "blackjack state", bytes.NewReader(pngBytes))
	} else {
		log.Error().
			Err(err).
			Str("game", "blackjack").
			Str("phase", "render_png").
			Str("session_id", view.SessionID).
			Msg("failed to render blackjack png")
	}
	if _, err := event.Client().Rest().UpdateInteractionResponse(
		event.ApplicationID(),
		event.Token(),
		update.Build(),
	); err != nil {
		log.Error().
			Err(err).
			Str("game", "blackjack").
			Str("phase", "send_base").
			Str("session_id", view.SessionID).
			Msg("failed to update blackjack response")
	}
}

func resolveBlackjackIfNeeded(ctx context.Context, economy *service.EconomyService, event *events.ComponentInteractionCreate, session *blackjackSession, action string) {
	blackjackMu.Lock()
	if session.Closed {
		blackjackMu.Unlock()
		return
	}

	if blackjackSessionReadyToSettle(session) {
		view := blackjackSnapshot(session, true, action, "ディーラーのターン")
		blackjackMu.Unlock()
		settled, err := settleBlackjack(ctx, economy, session)
		if err != nil {
			respondBlackjackError(event, "blackjack の精算に失敗しました。少し待って再試行してください。")
			return
		}
		view = blackjackSnapshot(settled, true, action, settled.FinalReason)
		updateBlackjackMessage(event, view, settled.Closed)
		return
	}

	view := blackjackSnapshot(session, false, action, blackjackActionMessage(session))
	blackjackMu.Unlock()
	updateBlackjackMessage(event, view, false)
}

func updateBlackjackMessage(event *events.ComponentInteractionCreate, view blackjackRenderState, closed bool) {
	embed := buildBlackjackEmbed(view, closed)
	components := buildBlackjackComponents(view, closed)
	if err := event.DeferUpdateMessage(); err != nil {
		log.Error().
			Err(err).
			Str("game", "blackjack").
			Str("phase", "defer_update").
			Str("session_id", view.SessionID).
			Msg("failed to defer blackjack update")
		if err := event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(*embed).
			SetContainerComponents(components...).
			Build()); err != nil {
			log.Error().
				Err(err).
				Str("game", "blackjack").
				Str("phase", "send_base").
				Str("session_id", view.SessionID).
				Msg("failed to update blackjack message")
		}
		return
	}

	update := discord.NewMessageUpdateBuilder().
		SetEmbeds(*embed).
		SetContainerComponents(components...)
	if pngBytes, err := renderBlackjackStatePNG(view); err == nil {
		imageEmbed := discord.NewEmbedBuilder().SetImage(blackjackImageAttachment).Build()
		update = update.
			SetEmbeds(*embed, imageEmbed).
			AddFile(blackjackImageFileName, "blackjack state", bytes.NewReader(pngBytes))
	} else {
		log.Error().
			Err(err).
			Str("game", "blackjack").
			Str("phase", "render_png").
			Str("session_id", view.SessionID).
			Msg("failed to render blackjack png")
	}
	if _, err := event.Client().Rest().UpdateInteractionResponse(
		event.ApplicationID(),
		event.Token(),
		update.Build(),
	); err != nil {
		log.Error().
			Err(err).
			Str("game", "blackjack").
			Str("phase", "send_base").
			Str("session_id", view.SessionID).
			Msg("failed to update blackjack message")
	}
}

func buildBlackjackEmbed(view blackjackRenderState, closed bool) *discord.Embed {
	color := 0x3498DB
	if closed {
		if strings.Contains(strings.ToLower(view.Footer), "勝") {
			color = 0x2ECC71
		} else if strings.Contains(strings.ToLower(view.Footer), "敗") || strings.Contains(strings.ToLower(view.Footer), "bust") {
			color = 0xE74C3C
		}
	}
	actionLabel := "次の操作"
	if closed {
		actionLabel = "結果"
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(view.Title).
		SetDescription(view.Subtitle).
		SetColor(color).
		AddField("ディーラーの手札", blackjackDealerSummary(view), true).
		AddField("あなたの手札", blackjackHandsSummary(view), true).
		AddField("残高", fmt.Sprintf("%d %s", view.Balance, service.CurrencyYenUnit), true).
		AddField("ベット", fmt.Sprintf("%d %s", view.BaseBet, service.CurrencyYenUnit), true).
		AddField(actionLabel, blackjackActionSummary(view, closed), false).
		SetTimestamp(time.Now()).
		Build()
	return &embed
}

func buildBlackjackComponents(view blackjackRenderState, closed bool) []discord.ContainerComponent {
	if closed {
		return nil
	}
	buttons := []discord.InteractiveComponent{}
	buttons = append(buttons, blackjackActionButton("Hit", "blackjack:hit:"+blackjackSessionID(view), false, discord.NewPrimaryButton))
	buttons = append(buttons, blackjackActionButton("Stand", "blackjack:stand:"+blackjackSessionID(view), false, discord.NewSecondaryButton))
	buttons = append(buttons, blackjackActionButton("Double", "blackjack:double:"+blackjackSessionID(view), !blackjackCanDouble(view), discord.NewSuccessButton))
	buttons = append(buttons, blackjackActionButton("Split", "blackjack:split:"+blackjackSessionID(view), !blackjackCanSplit(view), discord.NewDangerButton))
	return []discord.ContainerComponent{discord.NewActionRow(buttons...)}
}

func blackjackActionButton(label string, customID string, disabled bool, factory func(string, string) discord.ButtonComponent) discord.ButtonComponent {
	btn := factory(label, customID)
	btn.Disabled = disabled
	return btn
}

func blackjackActionSummary(view blackjackRenderState, closed bool) string {
	if closed {
		return view.Footer
	}
	action := strings.TrimSpace(view.CurrentAction)
	if action == "" {
		action = "操作を選んでください"
	}
	doubleLabel := "可"
	if !blackjackCanDouble(view) {
		doubleLabel = "不可"
	}
	splitLabel := "可"
	if !blackjackCanSplit(view) {
		splitLabel = "不可"
	}
	return fmt.Sprintf("%s / Double:%s / Split:%s", action, doubleLabel, splitLabel)
}

func blackjackDealerSummary(view blackjackRenderState) string {
	cards := make([]string, 0, len(view.Dealer))
	for i, card := range view.Dealer {
		if i == 1 && !view.RevealDealer {
			cards = append(cards, "??")
			continue
		}
		cards = append(cards, blackjackCardString(card))
	}
	if len(cards) == 0 {
		return "-"
	}
	if !view.RevealDealer {
		return strings.Join(cards, " | ")
	}
	total, soft := blackjackHandValue(view.Dealer)
	if soft {
		return fmt.Sprintf("%s (%d soft)", strings.Join(cards, " | "), total)
	}
	return fmt.Sprintf("%s (%d)", strings.Join(cards, " | "), total)
}

func blackjackHandsSummary(view blackjackRenderState) string {
	parts := make([]string, 0, len(view.Hands))
	for _, hand := range view.Hands {
		cards := make([]string, 0, len(hand.Cards))
		for _, card := range hand.Cards {
			cards = append(cards, blackjackCardString(card))
		}
		cardsText := strings.Join(cards, " | ")
		if cardsText == "" {
			cardsText = "-"
		}
		total, _ := blackjackHandValue(hand.Cards)
		label := fmt.Sprintf("%s (%d)", cardsText, total)
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "\n")
}

func blackjackSessionID(view blackjackRenderState) string {
	if strings.TrimSpace(view.SessionID) != "" {
		return strings.TrimSpace(view.SessionID)
	}
	return strings.TrimSpace(strings.TrimPrefix(view.Footer, "session:"))
}

func blackjackCanSplit(view blackjackRenderState) bool {
	if len(view.Hands) != 1 || view.ActiveHand != 0 {
		return false
	}
	hand := view.Hands[0]
	if len(hand.Cards) != 2 {
		return false
	}
	if hand.Done || hand.Busted {
		return false
	}
	return blackjackCardRank(hand.Cards[0]) == blackjackCardRank(hand.Cards[1])
}

func blackjackCanDouble(view blackjackRenderState) bool {
	if view.ActiveHand < 0 || view.ActiveHand >= len(view.Hands) {
		return false
	}
	hand := view.Hands[view.ActiveHand]
	return !hand.Done && !hand.Busted && len(hand.Cards) == 2
}

func blackjackSnapshot(session *blackjackSession, revealDealer bool, action string, footer string) blackjackRenderState {
	hands := make([]blackjackRenderHand, 0, len(session.Hands))
	for i, hand := range session.Hands {
		total, soft := blackjackHandValue(hand.Cards)
		hands = append(hands, blackjackRenderHand{
			Title:   fmt.Sprintf("Hand %d", i+1),
			Cards:   append([]blackjackCard(nil), hand.Cards...),
			Bet:     hand.Bet,
			Total:   total,
			Soft:    soft,
			Active:  i == session.ActiveHand && !session.Closed,
			Done:    hand.Done,
			Busted:  hand.Busted,
			Stand:   hand.Stand,
			Natural: hand.Natural,
		})
	}
	currentAction := blackjackActionMessage(session)
	if session.Closed {
		currentAction = session.FinalReason
	}
	return blackjackRenderState{
		Title:         "Blackjack",
		Subtitle:      "ボタンで Hit / Stand / Double / Split を選択。あなたの手札は「あなたの手札」に表示。",
		Dealer:        append([]blackjackCard(nil), session.Dealer...),
		RevealDealer:  revealDealer,
		Hands:         hands,
		ActiveHand:    session.ActiveHand,
		Balance:       session.BalanceAfter,
		BaseBet:       session.BaseBet,
		CurrentAction: currentAction,
		Footer:        footer,
		SessionID:     session.ID,
		Closed:        session.Closed,
	}
}

func blackjackActionMessage(session *blackjackSession) string {
	if session.Closed {
		return session.FinalReason
	}
	if session.ActiveHand >= len(session.Hands) {
		return "ディーラーのターン"
	}
	return fmt.Sprintf("Hand %d を操作中", session.ActiveHand+1)
}

func blackjackSessionReadyToSettle(session *blackjackSession) bool {
	if session.Closed {
		return false
	}
	for _, hand := range session.Hands {
		if !hand.Done && !hand.Busted {
			return false
		}
	}
	return true
}

func settleBlackjack(ctx context.Context, economy *service.EconomyService, session *blackjackSession) (*blackjackSession, error) {
	blackjackMu.Lock()
	if session.Closed {
		blackjackMu.Unlock()
		return session, nil
	}
	blackjackPlayDealer(session)
	totalPayout := blackjackResolvePayout(session)
	for i := range session.Hands {
		session.Hands[i].Done = true
	}
	session.DealersRevealed = true
	blackjackMu.Unlock()

	balance, err := economy.BlackjackCashout(ctx, session.UserID, totalPayout)
	if err != nil {
		return nil, err
	}

	blackjackMu.Lock()
	session.BalanceAfter = balance
	session.Closed = true
	session.FinalReason = blackjackResultReason(session, totalPayout)
	delete(blackjackSessions, session.ID)
	blackjackMu.Unlock()
	return session, nil
}

func blackjackResultReason(session *blackjackSession, totalPayout int64) string {
	if totalPayout == 0 {
		return "Blackjack 残念"
	}
	if totalPayout > session.BaseBet {
		return "Blackjack 勝利"
	}
	return "Blackjack プッシュ"
}

func blackjackResolvePayout(session *blackjackSession) int64 {
	dealerTotal, dealerSoft := blackjackHandValue(session.Dealer)
	dealerBust := dealerTotal > 21
	payout := int64(0)
	for _, hand := range session.Hands {
		handTotal, _ := blackjackHandValue(hand.Cards)
		if hand.Busted || handTotal > 21 {
			continue
		}
		if hand.Natural {
			if blackjackIsNaturalBlackjack(session.Dealer) {
				payout += hand.Bet
				continue
			}
			payout += hand.Bet + hand.Bet + hand.Bet/2
			continue
		}
		if dealerBust {
			payout += hand.Bet * 2
			continue
		}
		if handTotal > dealerTotal {
			payout += hand.Bet * 2
			continue
		}
		if handTotal == dealerTotal {
			payout += hand.Bet
		}
	}
	if dealerSoft && dealerTotal == 17 {
		_ = dealerSoft
	}
	return payout
}

func blackjackPlayDealer(session *blackjackSession) {
	for {
		total, soft := blackjackHandValue(session.Dealer)
		if total > 21 {
			return
		}
		if total < blackjackDealerStandSoft {
			blackjackDrawCard(session, &session.Dealer)
			continue
		}
		if total == blackjackDealerStandSoft && soft {
			return
		}
		return
	}
}

func blackjackHit(session *blackjackSession) error {
	blackjackMu.Lock()
	defer blackjackMu.Unlock()
	if session.Closed {
		return fmt.Errorf("session already closed")
	}
	if session.ActiveHand < 0 || session.ActiveHand >= len(session.Hands) {
		return fmt.Errorf("no active hand")
	}
	hand := &session.Hands[session.ActiveHand]
	if hand.Done || hand.Busted {
		return fmt.Errorf("current hand is already finished")
	}
	blackjackDrawCard(session, &hand.Cards)
	total, _ := blackjackHandValue(hand.Cards)
	if total > 21 {
		hand.Busted = true
		hand.Done = true
		blackjackAdvanceHand(session)
	}
	return nil
}

func blackjackStandLocked(session *blackjackSession) error {
	if session.Closed {
		return fmt.Errorf("session already closed")
	}
	if session.ActiveHand < 0 || session.ActiveHand >= len(session.Hands) {
		return fmt.Errorf("no active hand")
	}
	hand := &session.Hands[session.ActiveHand]
	if hand.Done || hand.Busted {
		return fmt.Errorf("current hand is already finished")
	}
	hand.Stand = true
	hand.Done = true
	blackjackAdvanceHand(session)
	return nil
}

func blackjackDouble(ctx context.Context, economy *service.EconomyService, session *blackjackSession) error {
	blackjackMu.Lock()
	if session.Closed {
		blackjackMu.Unlock()
		return fmt.Errorf("session already closed")
	}
	if session.ActiveHand < 0 || session.ActiveHand >= len(session.Hands) {
		blackjackMu.Unlock()
		return fmt.Errorf("no active hand")
	}
	hand := &session.Hands[session.ActiveHand]
	if len(hand.Cards) != 2 {
		blackjackMu.Unlock()
		return fmt.Errorf("double は最初の2枚でのみ可能です")
	}
	required := hand.Bet
	blackjackMu.Unlock()

	balance, err := economy.BlackjackAddBet(ctx, session.UserID, required)
	if err != nil {
		return err
	}

	blackjackMu.Lock()
	session.BalanceAfter = balance
	if session.Closed {
		blackjackMu.Unlock()
		return fmt.Errorf("session already closed")
	}
	hand = &session.Hands[session.ActiveHand]
	hand.Bet += required
	blackjackDrawCard(session, &hand.Cards)
	total, _ := blackjackHandValue(hand.Cards)
	if total > 21 {
		hand.Busted = true
	}
	hand.Done = true
	blackjackAdvanceHand(session)
	blackjackMu.Unlock()
	return nil
}

func blackjackSplit(ctx context.Context, economy *service.EconomyService, session *blackjackSession) error {
	blackjackMu.Lock()
	if session.Closed {
		blackjackMu.Unlock()
		return fmt.Errorf("session already closed")
	}
	if session.SplitUsed {
		blackjackMu.Unlock()
		return fmt.Errorf("split は1回までです")
	}
	if len(session.Hands) != 1 || session.ActiveHand != 0 {
		blackjackMu.Unlock()
		return fmt.Errorf("split できるのは最初の手札のみです")
	}
	hand := session.Hands[0]
	if len(hand.Cards) != 2 || blackjackCardRank(hand.Cards[0]) != blackjackCardRank(hand.Cards[1]) {
		blackjackMu.Unlock()
		return fmt.Errorf("同じランクの2枚でのみ split できます")
	}
	required := hand.Bet
	blackjackMu.Unlock()

	balance, err := economy.BlackjackAddBet(ctx, session.UserID, required)
	if err != nil {
		return err
	}

	blackjackMu.Lock()
	if session.Closed {
		blackjackMu.Unlock()
		return fmt.Errorf("session already closed")
	}
	if len(session.Hands) != 1 {
		blackjackMu.Unlock()
		return fmt.Errorf("split は1回までです")
	}
	original := session.Hands[0]
	first := blackjackHand{Cards: []blackjackCard{original.Cards[0]}, Bet: original.Bet}
	second := blackjackHand{Cards: []blackjackCard{original.Cards[1]}, Bet: original.Bet}
	blackjackDrawCard(session, &first.Cards)
	blackjackDrawCard(session, &second.Cards)
	session.Hands = []blackjackHand{first, second}
	session.ActiveHand = 0
	session.SplitUsed = true
	session.BalanceAfter = balance
	blackjackMu.Unlock()
	return nil
}

func blackjackAdvanceHand(session *blackjackSession) {
	for i := session.ActiveHand; i < len(session.Hands); i++ {
		if !session.Hands[i].Done && !session.Hands[i].Busted {
			session.ActiveHand = i
			return
		}
	}
	session.ActiveHand = len(session.Hands)
}

func blackjackDrawCard(session *blackjackSession, dest *[]blackjackCard) {
	card := blackjackTakeCard(session)
	if dest != nil {
		*dest = append(*dest, card)
	}
}

func blackjackTakeCard(session *blackjackSession) blackjackCard {
	if len(session.Deck) == 0 {
		session.Deck = buildBlackjackDeck()
	}
	card := session.Deck[0]
	session.Deck = session.Deck[1:]
	return card
}

func buildBlackjackDeck() []blackjackCard {
	deck := make([]blackjackCard, 0, blackjackDecks*52)
	for deckIndex := 0; deckIndex < blackjackDecks; deckIndex++ {
		for value := 0; value < 52; value++ {
			deck = append(deck, blackjackCard{Value: value})
		}
	}
	blackjackRand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	return deck
}

func blackjackCardValue(card blackjackCard) int {
	rank := blackjackCardRank(card)
	switch rank {
	case 0:
		return blackjackAceValue
	case 10, 11, 12:
		return 10
	default:
		return rank + 1
	}
}

func blackjackCardRank(card blackjackCard) int {
	return card.Value % 13
}

func blackjackHandValue(cards []blackjackCard) (int, bool) {
	total := 0
	aces := 0
	for _, card := range cards {
		value := blackjackCardValue(card)
		total += value
		if blackjackCardRank(card) == 0 {
			aces++
		}
	}
	soft := false
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	if aces > 0 && total <= 21 {
		soft = true
	}
	return total, soft
}

func blackjackIsNaturalBlackjack(cards []blackjackCard) bool {
	if len(cards) != 2 {
		return false
	}
	total, _ := blackjackHandValue(cards)
	return total == 21
}

func blackjackCardString(card blackjackCard) string {
	ranks := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
	suits := []string{"♠", "♥", "♦", "♣"}
	rank := ranks[blackjackCardRank(card)]
	suit := suits[(card.Value/13)%4]
	return rank + suit
}

func newBlackjackSessionID() string {
	b := make([]byte, 6)
	if _, err := crand.Read(b); err != nil {
		blackjackRand.Read(b)
	}
	return hex.EncodeToString(b)
}

func blackjackDealInitial(session *blackjackSession) {
	blackjackDrawCard(session, &session.Hands[0].Cards)
	blackjackDrawCard(session, &session.Dealer)
	blackjackDrawCard(session, &session.Hands[0].Cards)
	blackjackDrawCard(session, &session.Dealer)
}

func dealBlackjackCard(session *blackjackSession, hand *blackjackHand) {
	if hand == nil {
		return
	}
	card := blackjackTakeCard(session)
	hand.Cards = append(hand.Cards, card)
}

func blackjackPlayDealerLocked(session *blackjackSession) {
	for {
		total, soft := blackjackHandValue(session.Dealer)
		if total > 21 {
			return
		}
		if total < blackjackDealerStandSoft {
			blackjackDrawCard(session, &session.Dealer)
			continue
		}
		if total == blackjackDealerStandSoft && soft {
			return
		}
		return
	}
}

func blackjackPlayDealerLegacy(session *blackjackSession) {
	blackjackMu.Lock()
	defer blackjackMu.Unlock()
	blackjackPlayDealerLocked(session)
}

func blackjackDealerFinalTotal(session *blackjackSession) int {
	total, _ := blackjackHandValue(session.Dealer)
	return total
}

func blackjackDealerBust(session *blackjackSession) bool {
	return blackjackDealerFinalTotal(session) > 21
}

func blackjackUnlockAndSnapshot(session *blackjackSession, revealDealer bool, action string, footer string) blackjackRenderState {
	return blackjackSnapshot(session, revealDealer, action, footer)
}

func blackjackFinalizeView(session *blackjackSession, reason string) blackjackRenderState {
	return blackjackSnapshot(session, true, "settle", reason)
}

func blackjackOpenSession(session *blackjackSession) blackjackRenderState {
	return blackjackSnapshot(session, false, "start", blackjackActionMessage(session))
}

func blackjackAwaitingDealer(session *blackjackSession) bool {
	return blackjackSessionReadyToSettle(session)
}

func blackjackPlayerHasNaturalBlackjack(session *blackjackSession) bool {
	if len(session.Hands) == 0 {
		return false
	}
	return blackjackIsNaturalBlackjack(session.Hands[0].Cards)
}

func blackjackSessionHasDealerBlackjack(session *blackjackSession) bool {
	return blackjackIsNaturalBlackjack(session.Dealer)
}

func blackjackDisplayCards(cards []blackjackCard) []string {
	labels := make([]string, 0, len(cards))
	for _, card := range cards {
		labels = append(labels, blackjackCardString(card))
	}
	return labels
}

func blackjackSortedHands(session *blackjackSession) []blackjackHand {
	hands := append([]blackjackHand(nil), session.Hands...)
	sort.SliceStable(hands, func(i, j int) bool { return i < j })
	return hands
}

func blackjackCanAdvance(session *blackjackSession) bool {
	return !session.Closed && session.ActiveHand < len(session.Hands)
}

func blackjackSessionSummary(session *blackjackSession) string {
	parts := make([]string, 0, len(session.Hands))
	for i, hand := range session.Hands {
		total, _ := blackjackHandValue(hand.Cards)
		parts = append(parts, fmt.Sprintf("H%d:%d", i+1, total))
	}
	return strings.Join(parts, ", ")
}

func blackjackEnsureInitialDeal(session *blackjackSession) {
	if len(session.Dealer) > 0 || len(session.Hands) == 0 || len(session.Hands[0].Cards) > 0 {
		return
	}
	blackjackDealInitial(session)
}

func blackjackCheckNaturalEnd(session *blackjackSession) bool {
	playerBJ := blackjackPlayerHasNaturalBlackjack(session)
	dealerBJ := blackjackSessionHasDealerBlackjack(session)
	return playerBJ || dealerBJ
}

func blackjackInitialOutcome(session *blackjackSession) int64 {
	playerBJ := blackjackPlayerHasNaturalBlackjack(session)
	dealerBJ := blackjackSessionHasDealerBlackjack(session)
	if playerBJ && dealerBJ {
		return session.BaseBet
	}
	if playerBJ {
		return session.BaseBet + session.BaseBet + session.BaseBet/2
	}
	if dealerBJ {
		return 0
	}
	return -1
}

func blackjackSessionLabel(session *blackjackSession) string {
	if session.Closed {
		return session.FinalReason
	}
	return blackjackActionMessage(session)
}

func blackjackIsDealerVisible(session *blackjackSession) bool {
	return session.DealersRevealed || session.Closed
}

func blackjackSessionStateText(session *blackjackSession) string {
	if session.Closed {
		return session.FinalReason
	}
	return fmt.Sprintf("残高 %d %s", session.BalanceAfter, service.CurrencyYenUnit)
}

func blackjackFormatError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func blackjackSessionFooter(session *blackjackSession) string {
	return fmt.Sprintf("session:%s", session.ID)
}

func blackjackPrepareSession(session *blackjackSession) {
	blackjackEnsureInitialDeal(session)
}
