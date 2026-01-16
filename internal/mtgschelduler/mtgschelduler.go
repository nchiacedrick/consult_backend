package mtgschelduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"consult_app.cedrickewi/internal/store"
	"consult_app.cedrickewi/internal/zoom"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

const (
	// Task type for ending Zoom meetings
	TaskEndZoomMeeting = "end:zoom:meeting"
	// Queue name for meeting-related tasks
	QueueMeetings = "meetings"
)

// EndMeetingPayload represents the payload for ending a Zoom meeting task
type EndMeetingPayload struct {
	MeetingID int64 `json:"meeting_id"`
}

type MeetingScheduler struct {
	store  store.Storage
	client *asynq.Client
	logger *zap.SugaredLogger
}

// NewMeetingScheduler creates a new MeetingScheduler instance
// redisAddr should be in the format "host:port" or "redis://host:port"
func NewMeetingScheduler(store store.Storage, redisAddr string, logger *zap.SugaredLogger) *MeetingScheduler {
	redisOpt, err := asynq.ParseRedisURI(os.Getenv("REDIS_ADDR"))
	if err != nil {
		panic(err)
	}

	client := asynq.NewClient(redisOpt)

	return &MeetingScheduler{
		store:  store,
		client: client,
		logger: logger,
	}
}

// ScheduleEndMeeting schedules a task to end a Zoom meeting at a specific time
// endTime is the time when the meeting should be ended
func (ms *MeetingScheduler) ScheduleEndMeeting(ctx context.Context, meetingID int64, endTime time.Time) (string, error) {
	payload := EndMeetingPayload{
		MeetingID: meetingID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Calculate delay until end time
	delay := time.Until(endTime)
	if delay < 0 {
		return "", fmt.Errorf("end time is in the past: %v", endTime)
	}

	task := asynq.NewTask(
		TaskEndZoomMeeting,
		payloadBytes,
		asynq.Queue(QueueMeetings),
		asynq.ProcessAt(endTime.UTC()),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)

	info, err := ms.client.Enqueue(task)
	if err != nil {
		return "", fmt.Errorf("failed to enqueue task: %w", err)
	}

	ms.logger.Infof("Scheduled end meeting task for meeting ID %d at %v (task ID: %s)",
		meetingID, endTime, info.ID)

	return info.ID, nil
}

// handleEndZoomMeeting is the worker handler that processes the end meeting
func (ms *MeetingScheduler) handleEndZoomMeeting(ctx context.Context, t *asynq.Task) error {
	var payload EndMeetingPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	ms.logger.Infof("Processing end meeting task for meeting ID: %d", payload.MeetingID)

	// Call the EndZoomMeeting function from the zoom package
	if err := zoom.EndZoomMeeting(payload.MeetingID); err != nil {
		ms.logger.Errorf("Failed to end Zoom meeting %d: %v", payload.MeetingID, err)
		return fmt.Errorf("failed to end Zoom meeting: %w", err)
	}

	ms.logger.Infof("Successfully ended Zoom meeting %d", payload.MeetingID)
	return nil
}
