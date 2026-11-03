package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"pier/internal/config"
	"pier/internal/engine/projectlock"
	"pier/internal/runner"
	"pier/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		var in runner.Input
		if e := json.NewDecoder(os.Stdin).Decode(&in); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		if e := runner.Run(context.Background(), in); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	path := flag.String("config", "", "YAML configuration file")
	flag.Parse()
	cfg, e := config.Load(*path)
	if e != nil {
		log.Fatal(e)
	}
	if e = os.MkdirAll(cfg.DataDir, 0700); e != nil {
		log.Fatal(e)
	}
	lock, e := projectlock.Acquire(cfg.DataDir, "Pier controller")
	if e != nil {
		log.Fatal(e)
	}
	defer lock.Release()
	pidPath := filepath.Join(cfg.DataDir, "pier.pid")
	if e = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); e != nil {
		log.Fatal(e)
	}
	defer os.Remove(pidPath)
	app, e := server.New(cfg)
	if e != nil {
		log.Fatal(e)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Print("停止接收新任务，等待正在执行的任务完成…")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.HTTP.Shutdown(ctx)
	}()
	log.Printf("Pier: http://%s | data: %s", cfg.Listen, cfg.DataDir)
	e = app.Listen()
	app.Tasks.Shutdown()
	if e != nil {
		log.Fatal(e)
	}
}
