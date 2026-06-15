package config

import (
	"github.com/spf13/viper"
	"strconv"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	MySQL      MySQLConfig      `mapstructure:"mysql"`
	Memcached  MemcachedConfig  `mapstructure:"memcached"`
	Rosdomofon RosdomofonConfig `mapstructure:"rosdomofon"`
	Sections   SectionsConfig   `mapstructure:"sections"`
	JWTSecret  string           `mapstructure:"jwt_secret"`
	LogLevel   string           `mapstructure:"log_level"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

func (c MySQLConfig) DSN() string {
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(c.Port) + ")/" + c.Database + "?parseTime=true"
}

type MemcachedConfig struct {
	Address string `mapstructure:"address"`
}

type RosdomofonConfig struct {
	Email               string `mapstructure:"email"`
	Password            string `mapstructure:"password"`
	ServiceID           int    `mapstructure:"service_id"`
	SyncIntervalMinutes int    `mapstructure:"sync_interval_minutes"`
}

type SectionsConfig struct {
	Enabled []string `mapstructure:"enabled"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/app")

	// Устанавливаем значения по умолчанию
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("sections.enabled", []string{"cars", "keys"})
	viper.SetDefault("rosdomofon.sync_interval_minutes", 60)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
