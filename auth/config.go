package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type (
	logging struct {
		Level     int    `mapstructure:"level,omitempty"`
		Path      string `mapstructure:"path,omitempty"`
		MaxSizeMB int    `mapstructure:"max_size_mb,omitempty"`
	}

	connection_pool struct {
		MaxOpenConns    int `mapstructure:"max_open"`
		MaxIdleConns    int `mapstructure:"max_idle"`
		MaxLifetime     int `mapstructure:"max_lifetime"`
		MaxIdleLifetime int `mapstructure:"max_idle_lifetime"`
	}

	rate_limiting struct {
		GlobalRPS   int `mapstructure:"global_rps"`
		GlobalBurst int `mapstructure:"global_burst"`
		ClientRPS   int `mapstructure:"client_rps"`
		ClientBurst int `mapstructure:"client_burst"`
	}

	database struct {
		Host           string          `mapstructure:"host"`
		Port           int             `mapstructure:"port"`
		Service        string          `mapstructure:"service"`
		User           string          `mapstructure:"user"`
		Password       string          `mapstructure:"password"`
		ConnTimeout    string          `mapstructure:"connection_timeout"`
		ConnectionPool connection_pool `mapstructure:"connection_pool"`
	}

	configuration struct {
		Version         string        `mapstructure:"version,omitempty"`
		Logging         logging       `mapstructure:"logging"`
		ServerPort      string        `mapstructure:"server_port"`
		HTTPSServerPort string        `mapstructure:"https_server_port"`
		HTTPSEnabled    bool          `mapstructure:"https_enabled"`
		CertFile        string        `mapstructure:"cert_file"`
		KeyFile         string        `mapstructure:"key_file"`
		MetricPort      int           `mapstructure:"metric_port"`
		RateLimiting    rate_limiting `mapstructure:"rate_limiting"`
		Database        database      `mapstructure:"database"`
	}
)

var (
	AppConfig configuration
)

func ReadConfiguration() error {
	viper.SetConfigName("auth-server-config")
	viper.SetConfigType("json")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")

	err := viper.ReadInConfig()
	if err != nil {
		log.Warn().Err(err).Msg("configuration file not found, using defaults")
		setDefaults()
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("config unmarshal failed: %w", err)
	}

	// Load sensitive data from environment variables (override config file)
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		AppConfig.Database.Password = dbPassword
	}
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		viper.Set("jwt_secret", jwtSecret)
	}

	// Validate required fields
	if err := validateConfiguration(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

func setDefaults() {
	viper.SetDefault("version", "1.0.0")
	viper.SetDefault("server_port", 8080)
	viper.SetDefault("metric_port", 7071)
	viper.SetDefault("jwt_secret", "")
	viper.SetDefault("database.password", "")
	viper.SetDefault("logging.level", 2)
	viper.SetDefault("logging.path", "./logs/auth-server.log")
	viper.SetDefault("logging.max_size_mb", 100)
	viper.SetDefault("rate_limiting.global_rps", 100)
	viper.SetDefault("rate_limiting.global_burst", 10)
	viper.SetDefault("rate_limiting.client_rps", 10)
	viper.SetDefault("rate_limiting.client_burst", 2)
}

func validateConfiguration() error {
	if AppConfig.ServerPort == "" {
		return errors.New("server_port is required in configuration")
	}

	if AppConfig.Logging.Path == "" {
		return errors.New("logging.path is required in configuration")
	}

	if AppConfig.Logging.MaxSizeMB <= 0 {
		return errors.New("logging.max_size_mb must be greater than 0")
	}

	return nil
}
