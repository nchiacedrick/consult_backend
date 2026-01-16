package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"consult_app.cedrickewi/docs"
	"consult_app.cedrickewi/internal/mailer"
	"consult_app.cedrickewi/internal/mtgschelduler"
	"consult_app.cedrickewi/internal/payunit"
	"consult_app.cedrickewi/internal/store"
	"go.uber.org/zap"
)

const version = "0.0.1"

type application struct {
	config config
	store  store.Storage
	logger *zap.SugaredLogger
	mailer mailer.Mailer
	wg     sync.WaitGroup
	payunit payunit.Payunit  
	mtgschelduler *mtgschelduler.MeetingScheduler
}

type config struct {
	port        int
	addr        string
	db          dbConfig
	env         string
	smtp        smtp
	frontendURL string
	apiURL      string
}

type dbConfig struct {
	addr         string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

type smtp struct {
	host     string
	port     int
	username string
	password string
	sender   string
}


// setup server configuration and run server
func (app *application) run(mux http.Handler, port int) error {
	// Docs
	docs.SwaggerInfo.Version = version
	docs.SwaggerInfo.Host = app.config.apiURL
	docs.SwaggerInfo.BasePath = "/v1"

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	shutdown := make(chan error)
 
	// Start the meeting scheduler worker in a goroutine

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		app.logger.Infow("Signal caught", "signal", s.String())


		shutdown <- srv.Shutdown(ctx)
	}()

	app.logger.Infow("Server has started", "addr", app.config.addr, "env", app.config.env)
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	if err := <-shutdown; err != nil {
		return err
	}

	// Wait for all background goroutines to finish
	app.logger.Info("Waiting for background tasks to complete...")
	app.wg.Wait()

	app.logger.Infow("server has stopped", "addr", app.config.addr, "env", app.config.env)
	return nil
}
