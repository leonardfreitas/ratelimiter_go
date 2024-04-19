package configs

import "github.com/spf13/viper"

var cfg *conf

type conf struct {
	ApiPort             int    `mapstructure:"API_PORT"`
	MaxRequestsByIP     int    `mapstructure:"MAX_REQUESTS_BY_IP"`
	BlockUserForByIP    int    `mapstructure:"BLOCK_USER_FOR_BY_IP"`
	MaxRequestsByToken  int    `mapstructure:"MAX_REQUESTS_BY_TOKEN"`
	BlockUserForByToken int    `mapstructure:"BLOCK_USER_FOR_BY_TOKEN"`
	RedisHost           string `mapstructure:"REDIS_HOST"`
	RedisPassword       string `mapstructure:"REDIS_PASSWORD"`
	RedisDB             int    `mapstructure:"REDIS_DB"`
}

func LoadConfig(path string) (*conf, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("env")
	viper.AddConfigPath(path)
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
