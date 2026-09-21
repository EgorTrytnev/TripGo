package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr               string
	LogLevel               string
	ShutdownTimeout        time.Duration
	DatabaseURL            string
	DatabaseMaxConns       int
	DatabaseMinConns       int
	DatabaseMaxConnLife    time.Duration
	DatabaseConnectTimeout time.Duration
	DatabaseQueryTimeout   time.Duration
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found")
	}

	return Config{
		HTTPAddr:               getEnv("HTTP_ADDR", ":8080"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:        getDuration("SHUTDOWN_TIMEOUT", "10s"),
		DatabaseURL:            getEnv("DATABASE_URL", " "),
		DatabaseMaxConns:       getInt("DATABASE_MAX_CONNS", 10),
		DatabaseMinConns:       getInt("DATABASE_MIN_CONNS", 2),
		DatabaseMaxConnLife:    getDuration("DATABASE_MAX_CONN_LIFETIME", "30m"),
		DatabaseConnectTimeout: getDuration("DATABASE_CONNECT_TIMEOUT", "5s"),
		DatabaseQueryTimeout:   getDuration("DATABASE_QUERY_TIMEOUT", "3s"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getDuration(key, fallback string) time.Duration {
	value := getEnv(key, fallback)
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return duration
}
