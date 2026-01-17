package main

import (
	"flag"
	"log"
	"os"


	"consult_app.cedrickewi/internal/db"
	"consult_app.cedrickewi/internal/env"
	"consult_app.cedrickewi/internal/mailer"
	"consult_app.cedrickewi/internal/mtgschelduler"
	"consult_app.cedrickewi/internal/payunit"
	"consult_app.cedrickewi/internal/store"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
)

func main() {

	// setup configurations
	cfg := config{
		addr: env.GetString("ADDR", ":80"),

		// initialize database settings
		db: dbConfig{
			addr:         os.Getenv("DB_ADDR"),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 30),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 30),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "15m"),
		},
		smtp: smtp{
			host:     env.GetString("SMTP_HOST", "smtp.mailtrap.io"),
			port:     env.GetInt("SMTP_PORT", 2525),
			username: os.Getenv("MAILTRAP_USERNAME"),
			password: os.Getenv("MAILTRAP_PASSWORD"),
			sender:   os.Getenv("MAILTRAP_SENDER"),
		},
		env:         env.GetString("ENV", "development"),
		frontendURL: env.GetString("FRONTEND_URL", "http://localhost:4000"),
		apiURL:      env.GetString("EXTERNAL_URL", "localhost:8080"),
	}
	flag.IntVar(&cfg.port, "port", 8080, "API server port")

	// Logger
	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	// creating new instance of our database by calling the new function in internals, db
	db, err := db.New(
		cfg.db.addr,
		cfg.db.maxOpenConns,
		cfg.db.maxIdleConns,
		cfg.db.maxIdleTime,
	)
	if err != nil {
		logger.Fatal(err)
	}

	defer db.Close()

	logger.Info("database connection pool established")

	store := store.NewStorage(db)
	payunit := payunit.NewPayunit(db)

	// Initialize meeting scheduler with Redis address
	redisAddr := os.Getenv("REDIS_ADDR")
	meetingScheduler := mtgschelduler.NewMeetingScheduler(store, redisAddr, logger)

	app := &application{
		config:        cfg,
		store:         store,
		logger:        logger,
		mailer:        mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
		payunit:       payunit,
		mtgschelduler: meetingScheduler,
	}

	// if err := app.mailer.Send("testmailer@mail.cm", "user_welcome.tmpl", nil); err != nil {
	// 	app.logger.Errorln(err)
	// }

	mux := app.routes()

	log.Fatal(app.run(mux, cfg.port))
}
