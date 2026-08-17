package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	DiscordToken string `mapstructure:"discord_token" validate:"required"`
	DatabaseURL  string `mapstructure:"database_url" validate:"required"`
	LogLevel     string `mapstructure:"log_level"`
	TimeZone     string `mapstructure:"time_zone"`

	MarketGBMMu         float64 `mapstructure:"market_gbm_mu"`
	MarketGBMSigma      float64 `mapstructure:"market_gbm_sigma"`
	MarketPassiveMin    float64 `mapstructure:"market_passive_min"`
	MarketPassiveMax    float64 `mapstructure:"market_passive_max"`
	MarketMeanReversion float64 `mapstructure:"market_mean_reversion"`

	MinSameCommandIntervalMS int `mapstructure:"min_same_command_interval_ms"`
	SlashWindowSeconds       int `mapstructure:"slash_window_seconds"`
	MaxSlashPerWindow        int `mapstructure:"max_slash_per_window"`
	ComponentWindowSeconds   int `mapstructure:"component_window_seconds"`
	MaxComponentPerWindow    int `mapstructure:"max_component_per_window"`
	ChartMaxConcurrent       int `mapstructure:"chart_max_concurrent"`

	CasinoRTPBlackjack float64 `mapstructure:"casino_rtp_blackjack"`
	CasinoRTPChinchiro float64 `mapstructure:"casino_rtp_chinchiro"`
	CasinoRTPPoker     float64 `mapstructure:"casino_rtp_poker"`
	CasinoRTPMines     float64 `mapstructure:"casino_rtp_mines"`

	MinesBombCount     int     `mapstructure:"mines_bomb_count"`
	MinesInitialSafePC float64 `mapstructure:"mines_initial_safe_pc"`

	// StickyEnabled also controls whether the bot requests the guild message
	// gateway intent at all, so leaving it off keeps the bot from receiving
	// every message in every guild.
	StickyEnabled         bool `mapstructure:"sticky_enabled"`
	StickyDebounceSeconds int  `mapstructure:"sticky_debounce_seconds"`

	EconomyEnabled    bool            `mapstructure:"economy_enabled"`
	CasinoEnabled     bool            `mapstructure:"casino_enabled"`
	CryptoEnabled     bool            `mapstructure:"crypto_enabled"`
	PokerEnabled      bool            `mapstructure:"poker_enabled"`
	RolePanelEnabled  bool            `mapstructure:"role_panel_enabled"`
	ModEnabled        bool            `mapstructure:"mod_enabled"`
	DailyProfitCap    int64           `mapstructure:"daily_profit_cap"`
	WeeklyProfitCap   int64           `mapstructure:"weekly_profit_cap"`
	MaxBetAmount      int64           `mapstructure:"max_bet_amount"`
	MaxBetPercent     float64         `mapstructure:"max_bet_percent"`
	CashoutFeePercent float64         `mapstructure:"cashout_fee_percent"`
	CasinoFeePercent  float64         `mapstructure:"casino_fee_percent"`
	HighValueTaxBase  int64           `mapstructure:"high_value_tax_base"`
	HighValueTaxRate  float64         `mapstructure:"high_value_tax_rate"`
	RolePanelGuildIDs []string        `mapstructure:"role_panel_guild_ids"`
	RolePanelRoles    []RolePanelRole `mapstructure:"role_panel_roles"`

	OwnerIDs []string
}

type RolePanelRole struct {
	RoleID      string `mapstructure:"role_id"`
	Label       string `mapstructure:"label"`
	Description string `mapstructure:"description"`
}

func bindEnv(v *viper.Viper, key string, envNames ...string) error {
	args := append([]string{key}, envNames...)
	if err := v.BindEnv(args...); err != nil {
		return fmt.Errorf("failed to bind env %s: %w", key, err)
	}
	return nil
}

func setDefaults(v *viper.Viper, defaults map[string]any) {
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("ALTBOT")
	v.AutomaticEnv()
	bindings := []struct {
		key  string
		envs []string
	}{
		{key: "discord_token"},
		{key: "database_url"},
		{key: "log_level"},
		{key: "time_zone"},
		{key: "owner_id", envs: []string{"OWNER_ID", "ALTBOT_OWNER_ID", "ALTBOT_OWNER_IDS"}},
		{key: "market_gbm_mu"},
		{key: "market_gbm_sigma"},
		{key: "market_passive_min"},
		{key: "market_passive_max"},
		{key: "market_mean_reversion"},
		{key: "min_same_command_interval_ms"},
		{key: "slash_window_seconds"},
		{key: "max_slash_per_window"},
		{key: "component_window_seconds"},
		{key: "max_component_per_window"},
		{key: "chart_max_concurrent"},
		{key: "casino_rtp_blackjack"},
		{key: "casino_rtp_chinchiro"},
		{key: "casino_rtp_poker"},
		{key: "casino_rtp_mines"},
		{key: "casino_enabled"},
		{key: "crypto_enabled"},
		{key: "poker_enabled"},
		{key: "mines_bomb_count"},
		{key: "mines_initial_safe_pc"},
		{key: "role_panel_enabled"},
		{key: "sticky_enabled"},
		{key: "sticky_debounce_seconds"},
		{key: "economy_enabled"},
		{key: "mod_enabled"},
		{key: "daily_profit_cap"},
		{key: "weekly_profit_cap"},
		{key: "max_bet_amount"},
		{key: "max_bet_percent"},
		{key: "cashout_fee_percent"},
		{key: "casino_fee_percent"},
		{key: "high_value_tax_base"},
		{key: "high_value_tax_rate"},
		{key: "role_panel_guild_ids"},
		{key: "role_panel_roles"},
	}

	for _, b := range bindings {
		if err := bindEnv(v, b.key, b.envs...); err != nil {
			return Config{}, err
		}
	}

	setDefaults(v, map[string]any{
		"log_level":                   "info",
		"time_zone":                   "Asia/Tokyo",
		"market_gbm_mu":               0.00002,
		"market_gbm_sigma":            0.003,
		"market_passive_min":          0.996,
		"market_passive_max":          1.004,
		"market_mean_reversion":       0.18,
		"min_same_command_interval_ms": 800,
		"slash_window_seconds":         5,
		"max_slash_per_window":         8,
		"component_window_seconds":     4,
		"max_component_per_window":     12,
		"chart_max_concurrent":         2,
		"casino_rtp_blackjack":         0.945,
		"casino_rtp_chinchiro":         0.940,
		"casino_rtp_poker":             0.935,
		"casino_rtp_mines":             0.885,
		"casino_enabled":               false,
		"crypto_enabled":               true,
		"poker_enabled":                false,
		"mines_bomb_count":             4,
		"mines_initial_safe_pc":        0.88,
		"mod_enabled":                  false,
		"role_panel_enabled":           false,
		"sticky_enabled":               false,
		"sticky_debounce_seconds":      5,
		"economy_enabled":              true,
		"daily_profit_cap":             int64(3500000),
		"weekly_profit_cap":            int64(15000000),
		"max_bet_amount":               int64(1000000),
		"max_bet_percent":              0.1,
		"cashout_fee_percent":          0.05,
		"casino_fee_percent":           0.01,
		"high_value_tax_base":          int64(10000),
		"high_value_tax_rate":          0.15,
	})

	if err := v.ReadInConfig(); err != nil {
		_, _ = err.(viper.ConfigFileNotFoundError)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	rawOwnerIDs := v.GetString("owner_id")
	if rawOwnerIDs != "" {
		parts := strings.Split(rawOwnerIDs, ",")
		cfg.OwnerIDs = make([]string, 0, len(parts))
		for _, p := range parts {
			id := strings.TrimSpace(p)
			if id == "" {
				continue
			}
			cfg.OwnerIDs = append(cfg.OwnerIDs, id)
		}
	}

	rawRolePanelGuildIDs := v.GetString("role_panel_guild_ids")
	if rawRolePanelGuildIDs != "" {
		cfg.RolePanelGuildIDs = splitCommaValues(rawRolePanelGuildIDs)
	}

	rawRolePanelRoles := v.GetString("role_panel_roles")
	if rawRolePanelRoles != "" {
		roles, err := parseRolePanelRoles(rawRolePanelRoles)
		if err != nil {
			return Config{}, fmt.Errorf("failed to parse role_panel_roles: %w", err)
		}
		cfg.RolePanelRoles = roles
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func splitCommaValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		value := strings.TrimSpace(p)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func parseRolePanelRoles(raw string) ([]RolePanelRole, error) {
	items := strings.Split(raw, ";")
	roles := make([]RolePanelRole, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.Split(item, "|")
		roleID := strings.TrimSpace(parts[0])
		if roleID == "" {
			return nil, fmt.Errorf("role_id is empty in %q", item)
		}
		role := RolePanelRole{RoleID: roleID}
		if len(parts) > 1 {
			role.Label = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			role.Description = strings.TrimSpace(parts[2])
		}
		roles = append(roles, role)
	}
	return roles, nil
}
