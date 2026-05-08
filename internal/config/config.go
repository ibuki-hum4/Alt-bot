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

	EconomyEnabled    bool            `mapstructure:"economy_enabled"`
	CasinoEnabled     bool            `mapstructure:"casino_enabled"`
	CryptoEnabled     bool            `mapstructure:"crypto_enabled"`
	PokerEnabled      bool            `mapstructure:"poker_enabled"`
	RolePanelEnabled  bool            `mapstructure:"role_panel_enabled"`
	ModEnabled        bool            `mapstructure:"mod_enabled"`
	DailyProfitCap    int64           `mapstructure:"daily_profit_cap"`
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

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("ALTBOT")
	v.AutomaticEnv()
	if err := v.BindEnv("discord_token"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env discord_token: %w", err)
	}
	if err := v.BindEnv("database_url"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env database_url: %w", err)
	}
	if err := v.BindEnv("log_level"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env log_level: %w", err)
	}
	if err := v.BindEnv("time_zone"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env time_zone: %w", err)
	}
	if err := v.BindEnv("owner_id", "OWNER_ID", "ALTBOT_OWNER_ID", "ALTBOT_OWNER_IDS"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env owner_id: %w", err)
	}
	if err := v.BindEnv("market_gbm_mu"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env market_gbm_mu: %w", err)
	}
	if err := v.BindEnv("market_gbm_sigma"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env market_gbm_sigma: %w", err)
	}
	if err := v.BindEnv("market_passive_min"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env market_passive_min: %w", err)
	}
	if err := v.BindEnv("market_passive_max"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env market_passive_max: %w", err)
	}
	if err := v.BindEnv("market_mean_reversion"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env market_mean_reversion: %w", err)
	}
	if err := v.BindEnv("min_same_command_interval_ms"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env min_same_command_interval_ms: %w", err)
	}
	if err := v.BindEnv("slash_window_seconds"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env slash_window_seconds: %w", err)
	}
	if err := v.BindEnv("max_slash_per_window"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env max_slash_per_window: %w", err)
	}
	if err := v.BindEnv("component_window_seconds"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env component_window_seconds: %w", err)
	}
	if err := v.BindEnv("max_component_per_window"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env max_component_per_window: %w", err)
	}
	if err := v.BindEnv("chart_max_concurrent"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env chart_max_concurrent: %w", err)
	}
	if err := v.BindEnv("casino_rtp_blackjack"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env casino_rtp_blackjack: %w", err)
	}
	if err := v.BindEnv("casino_rtp_chinchiro"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env casino_rtp_chinchiro: %w", err)
	}
	if err := v.BindEnv("casino_rtp_poker"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env casino_rtp_poker: %w", err)
	}
	if err := v.BindEnv("casino_rtp_mines"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env casino_rtp_mines: %w", err)
	}
	if err := v.BindEnv("casino_enabled"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env casino_enabled: %w", err)
	}
	if err := v.BindEnv("crypto_enabled"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env crypto_enabled: %w", err)
	}
	if err := v.BindEnv("poker_enabled"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env poker_enabled: %w", err)
	}
	if err := v.BindEnv("mines_bomb_count"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env mines_bomb_count: %w", err)
	}
	if err := v.BindEnv("mines_initial_safe_pc"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env mines_initial_safe_pc: %w", err)
	}
	if err := v.BindEnv("role_panel_enabled"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env role_panel_enabled: %w", err)
	}
	if err := v.BindEnv("economy_enabled"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env economy_enabled: %w", err)
	}
	if err := v.BindEnv("mod_enabled"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env mod_enabled: %w", err)
	}
	if err := v.BindEnv("daily_profit_cap"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env daily_profit_cap: %w", err)
	}
	if err := v.BindEnv("max_bet_amount"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env max_bet_amount: %w", err)
	}
	if err := v.BindEnv("max_bet_percent"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env max_bet_percent: %w", err)
	}
	if err := v.BindEnv("cashout_fee_percent"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env cashout_fee_percent: %w", err)
	}
	if err := v.BindEnv("casino_fee_percent"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env casino_fee_percent: %w", err)
	}
	if err := v.BindEnv("high_value_tax_base"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env high_value_tax_base: %w", err)
	}
	if err := v.BindEnv("high_value_tax_rate"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env high_value_tax_rate: %w", err)
	}
	if err := v.BindEnv("role_panel_guild_ids"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env role_panel_guild_ids: %w", err)
	}
	if err := v.BindEnv("role_panel_roles"); err != nil {
		return Config{}, fmt.Errorf("failed to bind env role_panel_roles: %w", err)
	}

	v.SetDefault("log_level", "info")
	v.SetDefault("time_zone", "Asia/Tokyo")
	v.SetDefault("market_gbm_mu", 0.00002)
	v.SetDefault("market_gbm_sigma", 0.003)
	v.SetDefault("market_passive_min", 0.996)
	v.SetDefault("market_passive_max", 1.004)
	v.SetDefault("market_mean_reversion", 0.18)
	v.SetDefault("min_same_command_interval_ms", 800)
	v.SetDefault("slash_window_seconds", 5)
	v.SetDefault("max_slash_per_window", 8)
	v.SetDefault("component_window_seconds", 4)
	v.SetDefault("max_component_per_window", 12)
	v.SetDefault("chart_max_concurrent", 2)
	v.SetDefault("casino_rtp_blackjack", 0.945)
	v.SetDefault("casino_rtp_chinchiro", 0.940)
	v.SetDefault("casino_rtp_poker", 0.935)
	v.SetDefault("casino_rtp_mines", 0.885)
	v.SetDefault("casino_enabled", false)
	v.SetDefault("crypto_enabled", true)
	v.SetDefault("poker_enabled", false)
	v.SetDefault("mines_bomb_count", 4)         // 爆弾数を3→4に増加
	v.SetDefault("mines_initial_safe_pc", 0.88) // 初期安全率88%
	v.SetDefault("mod_enabled", false)
	v.SetDefault("role_panel_enabled", false)
	v.SetDefault("economy_enabled", false)
	v.SetDefault("daily_profit_cap", int64(5000000))     // 500万円（5M）
	v.SetDefault("max_bet_amount", int64(1000000))       // 100万円（1M）
	v.SetDefault("max_bet_percent", 0.1)                 // 10%
	v.SetDefault("cashout_fee_percent", 0.05)            // 5% (キャッシュアウト手数料)
	v.SetDefault("casino_fee_percent", 0.01)             // 1% (カジノ手数料)
	v.SetDefault("high_value_tax_base", int64(10000))    // 1万円以上で課税
	v.SetDefault("high_value_tax_rate", 0.15)            // 15% (基本税率)

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
