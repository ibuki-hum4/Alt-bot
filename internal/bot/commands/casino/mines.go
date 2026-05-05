package casino

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

const (
	minesRows         = 5
	minesCols         = 5
	minesTotalCells   = minesRows * minesCols
	minesCashoutIndex = minesTotalCells - 1
	minesDefaultCount = 3
)

type minesSession struct {
	ID              string
	UserID          string
	GuildID         string
	Bet             int64
	Expires         int64
	MinesMask       uint32
	OpenedMask      uint32
	MinesCount      int
	SafeCount       int
	BalanceAfterBet int64
	Closed          bool
}

var (
	minesMu       sync.Mutex
	minesSessions = map[string]*minesSession{}
	minesRand     = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func handleMines(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	bet := int64(event.SlashCommandInteractionData().Int("amount"))
	if bet <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("amount は 1 以上で指定してください。例: /casino mines amount:100").
			SetEphemeral(true).
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	balance, err := economy.MinesPlaceBet(ctx, event.User().ID.String(), bet)
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
			SetContent("mines の開始に失敗しました。少し待って再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	sessionID := newMinesSessionID()
	s := &minesSession{
		ID:              sessionID,
		UserID:          event.User().ID.String(),
		GuildID:         guildID.String(),
		Bet:             bet,
		Expires:         time.Now().Add(casinoInteractionTimeout).Unix(),
		MinesMask:       generateMinesMask(minesDefaultCount),
		OpenedMask:      0,
		MinesCount:      minesDefaultCount,
		SafeCount:       0,
		BalanceAfterBet: balance,
		Closed:          false,
	}

	minesMu.Lock()
	minesSessions[sessionID] = s
	minesMu.Unlock()

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(*buildMinesEmbed(*s, "Mines", "マスを開いて安全地帯を探します。")).
		SetContainerComponents(buildMinesComponents(*s, false)...).
		SetEphemeral(true).
		Build())
}

func HandleMinesComponent(economy *service.EconomyService, event *events.ComponentInteractionCreate) {
	parts := strings.Split(event.Data.CustomID(), ":")
	if len(parts) < 3 || parts[0] != "mines" {
		return
	}
	action := parts[1]
	sessionID := parts[2]

	minesMu.Lock()
	s, ok := minesSessions[sessionID]
	if !ok {
		minesMu.Unlock()
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("セッションが見つかりません。再度 /casino mines を実行してください。").
			SetEphemeral(true).
			Build())
		return
	}
	if s.UserID != event.User().ID.String() {
		minesMu.Unlock()
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このボタンはコマンド実行者のみ操作できます。").
			SetEphemeral(true).
			Build())
		return
	}
	if s.Closed || time.Now().Unix() > s.Expires {
		delete(minesSessions, sessionID)
		minesMu.Unlock()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(discord.NewEmbedBuilder().
				SetTitle("Mines 終了").
				SetDescription("セッションが終了しました。再度 /casino mines を実行してください。").
				SetColor(0x95A5A6).
				Build()).
			ClearContainerComponents().
			Build())
		return
	}

	if action == "cash" {
		payout := minesPayout(s.Bet, s.SafeCount)
		s.Closed = true
		delete(minesSessions, sessionID)
		minesMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		balance, err := economy.MinesCashout(ctx, s.UserID, payout)
		if err != nil {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("cashout の処理に失敗しました。少し待って再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		s.BalanceAfterBet = balance
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(*buildMinesEmbed(*s, "Mines Cash Out", "安全な時点で終了しました。")).
			SetContainerComponents(buildMinesComponents(*s, true)...).
			Build())
		return
	}

	if action != "open" || len(parts) != 4 {
		minesMu.Unlock()
		return
	}
	pos, err := strconv.Atoi(parts[3])
	if err != nil || pos < 0 || pos >= minesPlayableCells() {
		minesMu.Unlock()
		return
	}
	bit := minesBit(pos)
	if s.OpenedMask&bit != 0 {
		minesMu.Unlock()
		return
	}

	s.OpenedMask |= bit
	if s.MinesMask&bit != 0 {
		s.Closed = true
		delete(minesSessions, sessionID)
		minesMu.Unlock()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(*buildMinesEmbed(*s, "Mines BOOM", "地雷を踏みました。" )).
			SetContainerComponents(buildMinesComponents(*s, true)...).
			Build())
		return
	}

	s.SafeCount++
	updated := *s
	minesMu.Unlock()

	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(*buildMinesEmbed(updated, "Mines", "安全マスを開きました。" )).
		SetContainerComponents(buildMinesComponents(updated, false)...).
		Build())
}

func buildMinesEmbed(s minesSession, title string, desc string) *discord.Embed {
	multiplier := minesMultiplier(s.SafeCount)
	payout := minesPayout(s.Bet, s.SafeCount)
	color := 0x3498DB
	if strings.Contains(strings.ToLower(title), "boom") {
		color = 0xE74C3C
	} else if strings.Contains(strings.ToLower(title), "cash") {
		color = 0x2ECC71
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(title).
		SetDescription(desc).
		SetColor(color).
		AddField("ベット", fmt.Sprintf("%d %s", s.Bet, service.CurrencyYenUnit), true).
		AddField("安全数", fmt.Sprintf("%d", s.SafeCount), true).
		AddField("地雷数", fmt.Sprintf("%d", s.MinesCount), true).
		AddField("倍率", fmt.Sprintf("%.2fx", multiplier), true).
		AddField("払戻見込", fmt.Sprintf("%d %s", payout, service.CurrencyYenUnit), true).
		AddField("残高", fmt.Sprintf("%d %s", s.BalanceAfterBet, service.CurrencyYenUnit), true).
		SetTimestamp(time.Now()).
		Build()
	return &embed
}

func buildMinesComponents(s minesSession, reveal bool) []discord.ContainerComponent {
	rows := make([]discord.ContainerComponent, 0, minesRows)
	for r := 0; r < minesRows; r++ {
		buttons := make([]discord.InteractiveComponent, 0, minesCols)
		for c := 0; c < minesCols; c++ {
			idx := r*minesCols + c
			if idx == minesCashoutIndex {
				cash := discord.NewSuccessButton("CASH", fmt.Sprintf("mines:cash:%s", s.ID))
				if s.Closed {
					cash.Disabled = true
				}
				buttons = append(buttons, cash)
				continue
			}
			buttons = append(buttons, buildMinesCellButton(s, idx, reveal))
		}
		rows = append(rows, discord.NewActionRow(buttons...))
	}
	return rows
}

func buildMinesCellButton(s minesSession, idx int, reveal bool) discord.ButtonComponent {
	customID := fmt.Sprintf("mines:open:%s:%d", s.ID, idx)
	opened := s.OpenedMask&minesBit(idx) != 0
	isMine := s.MinesMask&minesBit(idx) != 0
	label := "[]"
	btn := discord.NewSecondaryButton(label, customID)

	if opened || reveal || s.Closed {
		btn.Disabled = true
		if isMine {
			btn = discord.NewDangerButton("X", customID)
		} else {
			count := minesAdjacentCount(s, idx)
			btn = discord.NewSecondaryButton(fmt.Sprintf("%d", count), customID)
		}
		btn.Disabled = true
		return btn
	}

	return btn
}

func minesAdjacentCount(s minesSession, idx int) int {
	row := idx / minesCols
	col := idx % minesCols
	count := 0
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			r := row + dr
			c := col + dc
			if r < 0 || r >= minesRows || c < 0 || c >= minesCols {
				continue
			}
			pos := r*minesCols + c
			if pos == minesCashoutIndex {
				continue
			}
			if s.MinesMask&minesBit(pos) != 0 {
				count++
			}
		}
	}
	return count
}

func minesMultiplier(safeCount int) float64 {
	if safeCount <= 0 {
		return 1.0
	}
	m := 1.0 + float64(safeCount)*0.25
	return math.Min(m, 5.0)
}

func minesPayout(bet int64, safeCount int) int64 {
	return int64(math.Floor(float64(bet) * minesMultiplier(safeCount)))
}

func generateMinesMask(count int) uint32 {
	positions := make([]int, 0, minesPlayableCells())
	for i := 0; i < minesPlayableCells(); i++ {
		positions = append(positions, i)
	}
	minesRand.Shuffle(len(positions), func(i, j int) {
		positions[i], positions[j] = positions[j], positions[i]
	})
	mask := uint32(0)
	for i := 0; i < count && i < len(positions); i++ {
		mask |= minesBit(positions[i])
	}
	return mask
}

func minesBit(pos int) uint32 {
	return 1 << pos
}

func minesPlayableCells() int {
	return minesTotalCells - 1
}

func newMinesSessionID() string {
	b := make([]byte, 6)
	if _, err := crand.Read(b); err != nil {
		minesRand.Read(b)
	}
	return hex.EncodeToString(b)
}
