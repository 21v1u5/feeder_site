package config

import "os"

type Config struct {
	Port              string
	RiotAPIKey        string
	RiotRateLimitSpec string
	PostgresURL       string
	RedisURL          string
	RabbitMQURL       string
}

func Load() Config {
	return Config{
		Port:              getEnv("PORT", "8080"),
		RiotAPIKey:        getEnv("RIOT_API_KEY", ""),
		RiotRateLimitSpec: getEnv("RIOT_RATE_LIMIT_WINDOWS", "20:1s,100:120s"),
		PostgresURL:       getEnv("POSTGRES_URL", ""),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
		RabbitMQURL:       getEnv("RABBITMQ_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
