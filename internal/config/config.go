package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig      `mapstructure:"server"`
	MySQL      MySQLConfig       `mapstructure:"mysql"`
	Memcached  MemcachedConfig   `mapstructure:"memcached"`
	Rosdomofon RosdomofonConfig  `mapstructure:"rosdomofon"`
	MQTT       MQTTConfig        `mapstructure:"mqtt"`
	Sections   SectionsConfig    `mapstructure:"sections"`
	Doors      map[string]Door   `mapstructure:"doors"`
	Auth       map[string]string `mapstructure:"auth"`
	JWTSecret  string            `mapstructure:"jwt_secret"`
	LogLevel   string            `mapstructure:"log_level"`
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
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + string(rune(c.Port)) + ")/" + c.Database + "?parseTime=true"
}

type MemcachedConfig struct {
	Address string `mapstructure:"address"`
}

type RosdomofonConfig struct {
	Email                  string   `mapstructure:"email"`
	Password               string   `mapstructure:"password"`
	SyncIntervalMinutes    int      `mapstructure:"sync_interval_minutes"`
	ServiceTypes           []string `mapstructure:"service_types"`
	ExpiryNotificationDays int      `mapstructure:"expiry_notification_days"`
}

type MQTTConfig struct {
	Broker   string `mapstructure:"broker"` // e.g. "tcp://localhost:1883"
	ClientID string `mapstructure:"client_id"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Topic    string `mapstructure:"topic"` // топик для подписки, можно с wildcard
	QOS      byte   `mapstructure:"qos"`   // 0, 1, 2
}

type SectionsConfig struct {
	Enabled []string `mapstructure:"enabled"`
}

type Door struct {
	URL         string `mapstructure:"url"`
	Method      string `mapstructure:"method"`
	AuthType    string `mapstructure:"auth_type"`
	Username    string `mapstructure:"username,omitempty"`
	Password    string `mapstructure:"password,omitempty"`
	Body        string `mapstructure:"body,omitempty"`
	ContentType string `mapstructure:"content_type,omitempty"`
	InsecureTLS bool   `mapstructure:"insecure_tls,omitempty"`
	MqttTopic   string `mapstructure:"mqtt_topic"` // топик, по которому слушаем события для этой двери
	FreeOut     bool   `mapstructure:"free_out"`   // свободный выезд: открывать любому номеру при детекте
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/app")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("sections.enabled", []string{"cars", "keys"})
	viper.SetDefault("rosdomofon.sync_interval_minutes", 60)
	viper.SetDefault("rosdomofon.service_types", []string{"VideoSurveillance", "Gate", "HardwareIntercom", "SoftwareIntercom"})
	viper.SetDefault("mqtt.qos", 1)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
