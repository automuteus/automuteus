package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/automuteus/automuteus/v8/internal/broker"
	"github.com/automuteus/automuteus/v8/pkg/logging"
)

// version and commit are overridden via -ldflags at build time (see Dockerfile.galactus).
var (
	version = "dev"
	commit  = "none"
)

const DefaultBrokerPort = "8123"

// DefaultDrainDelay leaves room for endpoint removal to propagate before the shutdown notice, while staying far
// below Kubernetes' default 30s termination grace period together with the 10s announcement budget.
const DefaultDrainDelay = 5 * time.Second

func main() {
	logging.Setup(os.Stdout)
	log.Println("galactus " + version + "-" + commit)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("No REDIS_ADDR specified. Exiting.")
	}

	brokerPort := os.Getenv("BROKER_PORT")
	if brokerPort == "" {
		log.Println("No BROKER_PORT provided. Defaulting to " + DefaultBrokerPort)
		brokerPort = DefaultBrokerPort
	}

	redisUser := os.Getenv("REDIS_USER")
	redisPass := os.Getenv("REDIS_PASS")
	if redisUser != "" {
		log.Println("Using REDIS_USER=" + redisUser)
	} else {
		log.Println("No REDIS_USER specified.")
	}

	if redisPass != "" {
		log.Println("Using REDIS_PASS=<redacted>")
	} else {
		log.Println("No REDIS_PASS specified.")
	}

	msgBroker := broker.NewBroker(redisAddr, redisUser, redisPass)
	// DRAIN_SECONDS: how long to keep running (refusing new clients, failing readiness) after SIGTERM before telling
	// the bots to end this replica's games. Must be well under the orchestrator's termination grace period.
	msgBroker.DrainDelay = DefaultDrainDelay
	if secs, err := strconv.Atoi(os.Getenv("DRAIN_SECONDS")); err == nil && secs >= 0 {
		msgBroker.DrainDelay = time.Duration(secs) * time.Second
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	go msgBroker.Start(brokerPort)
	<-sc

	// Kubernetes (and docker stop) send SIGTERM and wait for the grace period before SIGKILL; tell the bots now so
	// running games are ended and players unmuted before capture connections drop.
	log.Println("shutdown signal received; notifying bots")
	ctx, cancel := context.WithTimeout(context.Background(), msgBroker.DrainDelay+10*time.Second)
	defer cancel()
	if err := msgBroker.Shutdown(ctx); err != nil {
		log.Println("failed to announce shutdown:", err)
		os.Exit(1)
	}
	log.Println("shutdown announced; exiting")
}
