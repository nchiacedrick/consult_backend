package main

import "fmt"

type ZoomWebhook struct {
	Event   string `json:"event"`
	EventTS int64  `json:"event_ts"`
	Payload struct {
		AccountID string `json:"account_id"`
		Object    struct {
			UUID      string `json:"uuid"`
			ID        string  `json:"id"`
			Type      int    `json:"type"`
			Topic     string `json:"topic"`
			HostID    string `json:"host_id"`
			Duration  int64  `json:"duration"`
			StartTime string `json:"start_time"`
			Timezone  string `json:"timezone"`

			Participant struct {
				PublicIP          string `json:"public_ip"`
				UserID            string `json:"user_id"`
				UserName          string `json:"user_name"`
				ParticipantUserID string `json:"participant_user_id"`
				ID                string `json:"id"`
				JoinTime          string `json:"join_time"`
				LeaveTime         string `json:"leave_time,omitempty"` // Optional
				Email             string `json:"email"`
				ParticipantUUID   string `json:"participant_uuid"`
			} `json:"participant"`
		} `json:"object"`

		PlainToken string `json:"plainToken,omitempty"` // for url_validation
	} `json:"payload"`
}

func (app *application) handleParticipantJoined(data map[string]interface{}) {
	object := data["object"].(map[string]interface{})
	participant := object["participant"].(map[string]interface{})

	// Extract fields
	meetingID := object["id"].(float64)
	userName := participant["user_name"].(string)
	joinTime := participant["join_time"].(string)

	// Save to DB (you can convert time and parse meetingID)
	fmt.Println("User joined:", userName, "at", joinTime, "in meeting", meetingID)
	// Save to PostgreSQL here
}
