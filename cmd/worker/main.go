package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"consult_app.cedrickewi/internal/mtgschelduler"
	"consult_app.cedrickewi/internal/zoom"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	logg := logger.Sugar()


	opt, err := asynq.ParseRedisURI(os.Getenv("REDIS_ADDR"))
	if err != nil {
		log.Fatal(err)
	}

	server := asynq.NewServer(
		opt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				mtgschelduler.QueueMeetings: 10,
			},
		},
	)

	mux := asynq.NewServeMux()

	mux.HandleFunc(mtgschelduler.TaskEndZoomMeeting, func(ctx context.Context, t *asynq.Task) error {
		var payload mtgschelduler.EndMeetingPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return err
		}
		return zoom.EndZoomMeeting(payload.MeetingID)
	})

	logg.Info("Starting Asynq worker...")
	if err := server.Run(mux); err != nil {
		log.Fatal(err)
	}
}
