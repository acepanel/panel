package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"

	"github.com/acepanel/panel/v3/pkg/clientip"
	"github.com/acepanel/panel/v3/pkg/config"
)

// Throttle 按客户端来源限流
func Throttle(conf *config.Config, tokens uint64, interval time.Duration) func(next http.Handler) http.Handler {
	store, err := memorystore.New(&memorystore.Config{
		Tokens:   tokens,
		Interval: interval,
	})
	if err != nil {
		log.Fatalf("failed to create throttle memorystore: %v", err)
	}

	limiter, err := httplimit.NewMiddleware(store, func(r *http.Request) (string, error) {
		return clientip.Bucket(clientip.FromRequest(r, conf.HTTP.IPHeader, conf.HTTP.TrustedProxies)), nil
	})
	if err != nil {
		log.Fatalf("failed to initialize throttle middleware: %v", err)
	}

	return limiter.Handle
}
