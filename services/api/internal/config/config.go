package config

import "os"

type Config struct {
	Port        string
	RiotAPIKey  string
	PostgresURL string
	RedisURL    string
	RabbitMQURL string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		RiotAPIKey:  getEnv("RIOT_API_KEY", ""),
		PostgresURL: getEnv("POSTGRES_URL", ""),
		RedisURL:    getEnv("REDIS_URL", ""),
		RabbitMQURL: getEnv("RABBITMQ_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
