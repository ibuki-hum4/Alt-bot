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

	OwnerIDs []string
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

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}
