package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	rootcommands "alt-bot/internal/bot/commands"
	cmdcasino "alt-bot/internal/bot/commands/casino"
	cmdcrypto "alt-bot/internal/bot/commands/crypto"
	cmdmod "alt-bot/internal/bot/commands/mod"
	cmdutil "alt-bot/internal/bot/commands/util"
	"alt-bot/internal/config"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rs/zerolog"
)

const newsTickInterval = 10 * time.Minute

const (
	defaultMinSameCommandInterval   = 800 * time.Millisecond
	defaultSlashWindowDuration      = 5 * time.Second
	defaultMaxSlashPerWindowPerUser = 8
	defaultComponentWindowDuration  = 4 * time.Second
	defaultMaxComponentPerWindow    = 12
	defaultChartMaxConcurrent       = 2
	rateLimiterMaxEntries           = 2000
	rateLimiterTTL                  = 15 * time.Minute
)

type userBurstCounter struct {
	windowStart time.Time
	count       int
}

type Handlers struct {
	economy  *service.EconomyService
	ownerIDs map[string]struct{}
	logger   zerolog.Logger

	newsMu       sync.RWMutex
	newsChannels map[string]snowflake.ID
	newsCancel   context.CancelFunc

	rateMu               sync.Mutex
	lastCommandAt        map[string]time.Time
	slashBurstByUser     map[string]userBurstCounter
	componentBurstByUser map[string]userBurstCounter
	chartSem             chan struct{}

	minSameCommandInterval  time.Duration
	slashWindowDuration     time.Duration
	maxSlashPerWindow       int
	componentWindowDuration time.Duration
	maxComponentPerWindow   int
}

func NewHandlers(economy *service.EconomyService, cfg config.Config, ownerIDs []string, logger zerolog.Logger) *Handlers {
	set := make(map[string]struct{}, len(ownerIDs))
	for _, id := range ownerIDs {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}

	minSame := time.Duration(cfg.MinSameCommandIntervalMS) * time.Millisecond
	if minSame <= 0 {
		minSame = defaultMinSameCommandInterval
	}
	slashWindow := time.Duration(cfg.SlashWindowSeconds) * time.Second
	if slashWindow <= 0 {
		slashWindow = defaultSlashWindowDuration
	}
	maxSlash := cfg.MaxSlashPerWindow
	if maxSlash <= 0 {
		maxSlash = defaultMaxSlashPerWindowPerUser
	}
	componentWindow := time.Duration(cfg.ComponentWindowSeconds) * time.Second
	if componentWindow <= 0 {
		componentWindow = defaultComponentWindowDuration
	}
	maxComponent := cfg.MaxComponentPerWindow
	if maxComponent <= 0 {
		maxComponent = defaultMaxComponentPerWindow
	}
	maxCharts := cfg.ChartMaxConcurrent
	if maxCharts <= 0 {
		maxCharts = defaultChartMaxConcurrent
	}

	return &Handlers{
		economy:                 economy,
		ownerIDs:                set,
		logger:                  logger,
		newsChannels:            make(map[string]snowflake.ID),
		lastCommandAt:           make(map[string]time.Time),
		slashBurstByUser:        make(map[string]userBurstCounter),
		componentBurstByUser:    make(map[string]userBurstCounter),
		chartSem:                make(chan struct{}, maxCharts),
		minSameCommandInterval:  minSame,
		slashWindowDuration:     slashWindow,
		maxSlashPerWindow:       maxSlash,
		componentWindowDuration: componentWindow,
		maxComponentPerWindow:   maxComponent,
	}
}

func (h *Handlers) allowSlash(userID string, command string, now time.Time) (bool, string) {
	if _, ok := h.ownerIDs[userID]; ok {
		return true, ""
	}

	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	h.cleanupRateLimiterState(now)

	key := userID + ":" + command
	if last, ok := h.lastCommandAt[key]; ok {
		if now.Sub(last) < h.minSameCommandInterval {
			return false, "同じコマンドの連続実行が速すぎます。少し待ってから再実行してください。"
		}
	}
	h.lastCommandAt[key] = now

	counter := h.slashBurstByUser[userID]
	if counter.windowStart.IsZero() || now.Sub(counter.windowStart) >= h.slashWindowDuration {
		h.slashBurstByUser[userID] = userBurstCounter{windowStart: now, count: 1}
		return true, ""
	}
	counter.count++
	h.slashBurstByUser[userID] = counter
	if counter.count > h.maxSlashPerWindow {
		return false, "コマンド送信が多すぎます。数秒待ってから再試行してください。"
	}
	return true, ""
}

func (h *Handlers) allowComponent(userID string, now time.Time) (bool, string) {
	if _, ok := h.ownerIDs[userID]; ok {
		return true, ""
	}

	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	h.cleanupRateLimiterState(now)

	counter := h.componentBurstByUser[userID]
	if counter.windowStart.IsZero() || now.Sub(counter.windowStart) >= h.componentWindowDuration {
		h.componentBurstByUser[userID] = userBurstCounter{windowStart: now, count: 1}
		return true, ""
	}
	counter.count++
	h.componentBurstByUser[userID] = counter
	if counter.count > h.maxComponentPerWindow {
		return false, "操作が多すぎます。少し待ってから再試行してください。"
	}
	return true, ""
}

func (h *Handlers) cleanupRateLimiterState(now time.Time) {
	if len(h.lastCommandAt) <= rateLimiterMaxEntries &&
		len(h.slashBurstByUser) <= rateLimiterMaxEntries &&
		len(h.componentBurstByUser) <= rateLimiterMaxEntries {
		return
	}

	for key, t := range h.lastCommandAt {
		if now.Sub(t) > rateLimiterTTL {
			delete(h.lastCommandAt, key)
		}
	}
	for userID, c := range h.slashBurstByUser {
		if c.windowStart.IsZero() || now.Sub(c.windowStart) > rateLimiterTTL {
			delete(h.slashBurstByUser, userID)
		}
	}
	for userID, c := range h.componentBurstByUser {
		if c.windowStart.IsZero() || now.Sub(c.windowStart) > rateLimiterTTL {
			delete(h.componentBurstByUser, userID)
		}
	}
}

func (h *Handlers) SetNewsChannel(guildID snowflake.ID, channelID snowflake.ID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.economy.SetNewsChannel(ctx, guildID.String(), channelID.String()); err != nil {
		return err
	}

	h.newsMu.Lock()
	defer h.newsMu.Unlock()
	h.newsChannels[guildID.String()] = channelID
	return nil
}

func (h *Handlers) DisableNewsChannel(guildID snowflake.ID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.economy.DisableNewsChannel(ctx, guildID.String()); err != nil {
		return err
	}

	h.newsMu.Lock()
	defer h.newsMu.Unlock()
	delete(h.newsChannels, guildID.String())
	return nil
}

func (h *Handlers) NewsChannelStatus(guildID snowflake.ID) (snowflake.ID, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	channelID, ok, err := h.economy.GetNewsChannel(ctx, guildID.String())
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	parsedID, parseErr := snowflake.Parse(channelID)
	if parseErr != nil {
		return 0, false, fmt.Errorf("invalid stored channel id: %w", parseErr)
	}
	return parsedID, true, nil
}

func (h *Handlers) StartNewsLoop(client bot.Client) {
	h.newsMu.Lock()
	if h.newsCancel != nil {
		h.newsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.newsCancel = cancel
	h.newsMu.Unlock()

	loadCtx, loadCancel := context.WithTimeout(context.Background(), 5*time.Second)
	channels, loadErr := h.economy.LoadNewsChannels(loadCtx)
	loadCancel()
	if loadErr != nil {
		h.logger.Error().Err(loadErr).Msg("failed to load news channels from db")
	} else {
		h.newsMu.Lock()
		for guildID, channelID := range channels {
			parsedID, parseErr := snowflake.Parse(channelID)
			if parseErr != nil {
				h.logger.Warn().Err(parseErr).Str("guild_id", guildID).Str("channel_id", channelID).Msg("invalid stored channel id")
				continue
			}
			h.newsChannels[guildID] = parsedID
		}
		h.newsMu.Unlock()
	}

	go func() {
		ticker := time.NewTicker(newsTickInterval)
		defer ticker.Stop()

		h.logger.Info().Dur("interval", newsTickInterval).Msg("auto news loop started")
		for {
			select {
			case <-ctx.Done():
				h.logger.Info().Msg("auto news loop stopped")
				return
			case now := <-ticker.C:
				h.dispatchAutoNews(client, now)
			}
		}
	}()
}

func (h *Handlers) StopNewsLoop() {
	h.newsMu.Lock()
	defer h.newsMu.Unlock()
	if h.newsCancel != nil {
		h.newsCancel()
		h.newsCancel = nil
	}
}

func (h *Handlers) dispatchAutoNews(client bot.Client, now time.Time) {
	h.newsMu.RLock()
	channels := make([]snowflake.ID, 0, len(h.newsChannels))
	for _, ch := range h.newsChannels {
		channels = append(channels, ch)
	}
	h.newsMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, tickErr := h.economy.ProcessNewsTick(ctx, now)
	if tickErr != nil {
		h.logger.Error().Err(tickErr).Msg("failed to process news tick")
		return
	}

	if !res.Hit {
		return
	}

	if len(channels) == 0 {
		return
	}

	embed, err := h.economy.BuildNewsEmbed(res.EventType, service.NewsTemplateData{
		Amount: res.Amount,
		Price:  res.Price,
	})
	if err != nil {
		h.logger.Error().Err(err).Str("event_type", string(res.EventType)).Msg("failed to build auto news embed")
		return
	}

	for _, channelID := range channels {
		_, sendErr := client.Rest().CreateMessage(channelID, discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("自動ニュース発生 (抽選確率 %.3f%%)", res.Probability*100)).
			SetEmbeds(embed).
			Build())
		if sendErr != nil {
			h.logger.Warn().Err(sendErr).Str("channel_id", channelID.String()).Msg("failed to send auto news")
		}
	}
}

func (h *Handlers) RegisterCommands(ctx context.Context, client bot.Client) error {
	existing, err := client.Rest().GetGlobalCommands(client.ApplicationID(), false)
	if err != nil {
		var restErr rest.Error
		if ok := errorAs(err, &restErr); ok {
			h.logger.Warn().Err(err).Any("rest_error", restErr).Msg("failed to fetch existing commands before register")
		} else {
			h.logger.Warn().Err(err).Msg("failed to fetch existing commands before register")
		}
	}

	expected := rootcommands.Definitions()
	if err == nil && sameCommandSet(existing, expected) {
		h.logger.Info().Msg("global commands unchanged; skip register")
		return nil
	}

	_, err = client.Rest().SetGlobalCommands(client.ApplicationID(), expected)
	if err != nil {
		return err
	}
	h.logger.Info().Msg("global commands registered")
	return nil
}

func (h *Handlers) OnApplicationCommandInteraction(event *events.ApplicationCommandInteractionCreate) {
	userID := event.User().ID.String()
	name := event.SlashCommandInteractionData().CommandName()
	if ok, message := h.allowSlash(userID, name, time.Now()); !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	switch name {
	case "ping":
		cmdutil.HandlePing(h.logger, event)
	case "help":
		cmdutil.HandleHelp(event)
	case "work":
		cmdutil.HandleWorkSlash(h.logger, event)
	case "crypto":
		cmdcrypto.HandleCryptoSlash(h.logger, h.economy, event)
	case "casino":
		cmdcasino.HandleCasino(event)
	case "commands":
		cmdutil.HandleCommands(h.logger, event, h.ownerIDs)
	case "news":
		cmdutil.HandleNews(h.logger, h.economy, h.SetNewsChannel, h.DisableNewsChannel, h.NewsChannelStatus, event)
	case "rate":
		cmdutil.HandleRate(event, h.economy)
	case "chart":
		select {
		case h.chartSem <- struct{}{}:
			defer func() { <-h.chartSem }()
		default:
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("現在チャート生成が混み合っています。数秒後に再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		cmdutil.HandleChart(h.logger, h.economy, event)
	case "mod":
		cmdmod.HandleModeration(event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のコマンドです").
			SetEphemeral(true).
			Build())
	}
}

func (h *Handlers) OnComponentInteraction(event *events.ComponentInteractionCreate) {
	if ok, message := h.allowComponent(event.User().ID.String(), time.Now()); !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	customID := event.Data.CustomID()
	switch {
	case strings.HasPrefix(customID, "work:"):
		cmdutil.HandleWorkComponent(h.logger, h.economy, event)
	case strings.HasPrefix(customID, "crypto:"):
		cmdcrypto.HandleCryptoComponent(h.logger, h.economy, event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のボタンです").
			SetEphemeral(true).
			Build())
	}
}

func sameCommandSet(existing []discord.ApplicationCommand, expected []discord.ApplicationCommandCreate) bool {
	if len(existing) != len(expected) {
		return false
	}

	existingNames := make([]string, 0, len(existing))
	for _, cmd := range existing {
		existingNames = append(existingNames, cmd.Name())
	}
	expectedNames := make([]string, 0, len(expected))
	for _, cmd := range expected {
		expectedNames = append(expectedNames, commandCreateName(cmd))
	}

	sort.Strings(existingNames)
	sort.Strings(expectedNames)
	for i := range existingNames {
		if existingNames[i] != expectedNames[i] {
			return false
		}
	}
	return true
}

func commandCreateName(cmd discord.ApplicationCommandCreate) string {
	switch c := cmd.(type) {
	case discord.SlashCommandCreate:
		return c.Name
	case discord.UserCommandCreate:
		return c.Name
	case discord.MessageCommandCreate:
		return c.Name
	default:
		return ""
	}
}

func errorAs(err error, target interface{}) bool {
	switch t := target.(type) {
	case *rest.Error:
		re, ok := err.(rest.Error)
		if !ok {
			return false
		}
		*t = re
		return true
	default:
		return false
	}
}
