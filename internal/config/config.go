package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv           string
	Port             string
	DatabaseURL      string
	RedisURL         string
	JWTSecret        string
	WhatsappToken    string
	WhatsappPhoneID  string
	AllowedOrigins   []string
	ResendAPIKey     string
	ResendFromEmail  string
	FrontendURL      string
	TelegramBotToken string
	TelegramChatID   string
}

func LoadConfig() *Config {

	viper.SetConfigFile(".env")

	err := viper.ReadInConfig()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	viper.AutomaticEnv()

	config := &Config{
		AppEnv:          viper.GetString("APP_ENV"),
		Port:            viper.GetString("PORT"),
		DatabaseURL:     viper.GetString("DATABASE_URL"),
		RedisURL:        viper.GetString("REDIS_URL"),
		JWTSecret:       viper.GetString("JWT_SECRET"),
		WhatsappToken:   viper.GetString("WHATSAPP_TOKEN"),
		WhatsappPhoneID: viper.GetString("WHATSAPP_PHONE_ID"),
		AllowedOrigins:  viper.GetStringSlice("ALLOWED_ORIGINS"),
		ResendAPIKey:    viper.GetString("RESEND_API_KEY"),
		ResendFromEmail: viper.GetString("RESEND_FROM_EMAIL"),
		FrontendURL:     viper.GetString("FRONTEND_URL"),

		TelegramBotToken: viper.GetString("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   viper.GetString("TELEGRAM_CHAT_ID"),
	}
	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if config.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	if config.WhatsappToken == "" {
		log.Fatal("WHATSAPP_TOKEN is required")
	}
	if config.WhatsappPhoneID == "" {
		log.Fatal("WHATSAPP_PHONE_ID is required")
	}
	return config
}
