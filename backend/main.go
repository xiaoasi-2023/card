package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load("../.env")
	}
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	app, err := newApp(config)
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "seed-catalog" {
		replace := len(os.Args) > 2 && os.Args[2] == "--replace"
		result, err := seedDefaultCatalog(app.db, replace)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf(
			"catalog seeded: platforms=%d products=%d skus=%d purged_platforms=%d purged_products=%d purged_skus=%d purged_batches=%d purged_cards=%d purged_orders=%d",
			result.PlatformsUpserted,
			result.ProductsUpserted,
			result.SKUsUpserted,
			result.PlatformsPurged,
			result.ProductsPurged,
			result.SKUsPurged,
			result.CardBatchesPurged,
			result.CardsPurged,
			result.OrdersPurged,
		)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go app.runExpiryWorker(ctx)

	server := &http.Server{Addr: config.Addr, Handler: app.router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("card service listening on %s", config.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
