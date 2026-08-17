package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"private-chat/internal/config"
	"private-chat/internal/logger"
	"private-chat/internal/server"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	logger.SetDefault(log)
	logger.Info("private-chat starting", map[string]interface{}{
		"version": "1.0.0",
		"addr":    cfg.Addr(),
	})

	app, err := server.New(cfg)
	if err != nil {
		logger.Error("init app failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	app.StartBackground()

	srv := &http.Server{
		Addr:    cfg.Addr(),
		Handler: app.Router(),
	}

	go func() {
		logger.Info("http server listening", map[string]interface{}{"addr": cfg.Addr()})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	app.Stop()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", map[string]interface{}{"error": err.Error()})
	}
	logger.Info("stopped", nil)
}
