package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var defaultInterval = 30 * time.Second

type Record struct {
	ID        int
	UpdatedAt time.Time
}

func runJob() {
	start := time.Now()
	today := start.UTC().Format("2006-01-02")

	slog.Info("job started", "date", today)

	records := []Record{
		{ID: 1, UpdatedAt: start},
		{ID: 2, UpdatedAt: start},
		{ID: 3, UpdatedAt: start},
	}

	time.Sleep(200 * time.Millisecond)

	slog.Info("job completed",
		"date", today,
		"records_updated", len(records),
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	interval := defaultInterval
	if val := os.Getenv("WORKER_INTERVAL_SECONDS"); val != "" {
		if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
			interval = time.Duration(secs) * time.Second
		}
	}

	slog.Info("worker starting",
		"env", env,
		"interval_seconds", interval.Seconds(),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runJob()

	for {
		select {
		case <-ticker.C:
			runJob()
		case sig := <-quit:
			slog.Info("worker shutting down", "signal", sig.String())
			return
		}
	}
}
