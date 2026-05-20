package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type SessionConfig struct {
	AccessTTL        time.Duration // 15m
	RefreshTTL       time.Duration // sliding: 30d
	AbsoluteTTL      time.Duration // cap: 90d
	RefreshTokenSize int           // bytes: 32
}

func SessionConfigFromEnv() (SessionConfig, error) {
	accessTTL, err := envDuration("ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return SessionConfig{}, err
	}
	refreshTTL, err := envDuration("REFRESH_TTL", 30*24*time.Hour)
	if err != nil {
		return SessionConfig{}, err
	}
	absoluteTTL, err := envDuration("ABSOLUTE_TTL", 90*24*time.Hour)
	if err != nil {
		return SessionConfig{}, err
	}

	refreshTokenSize, err := envInt("REFRESH_TOKEN_SIZE", 32)
	if err != nil {
		return SessionConfig{}, err
	}

	return SessionConfig{
		AccessTTL:        accessTTL,
		RefreshTTL:       refreshTTL,
		AbsoluteTTL:      absoluteTTL,
		RefreshTokenSize: refreshTokenSize,
	}, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", key, v, err)
	}
	return d, nil
}

func envInt(key string, def int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid int %q: %w", key, v, err)
	}
	return n, nil
}
