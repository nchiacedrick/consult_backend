package zoom

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"consult_app.cedrickewi/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

type ZoomTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type ZoomValidationPayload struct {
	PlainToken string `json:"plainToken"`
}

type ZoomEventRequest struct {
	Event   string `json:"event"`
	Payload struct {
		PlainToken string `json:"plainToken"`
	} `json:"payload"`
}

type SignatureRequest struct {
	MeetingNumber int64 `json:"meetingNumber"`
	Role          int64 `json:"role"`
}

type SignatureResponse struct {
	Signature string `json:"signature"`
	SDKKey    string `json:"sdkKey"`
}

// GenerateZoomAccessToken fetches the Zoom access token
func GenerateZoomAccessToken() (*ZoomTokenResponse, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, proceeding with existing environment variables")
	}

	ZOOM_CLIENT_SECRET := os.Getenv("ZOOM_CLIENT_SECRET")
	ZOOM_ACCOUNT_ID := os.Getenv("ZOOM_ACCOUNT_ID")
	ZOOM_CLIENT_ID := os.Getenv("ZOOM_CLIENT_ID")

	url := fmt.Sprintf("https://zoom.us/oauth/token?grant_type=account_credentials&account_id=%s", ZOOM_ACCOUNT_ID)

	encodedValue := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", ZOOM_CLIENT_ID, ZOOM_CLIENT_SECRET)))

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedValue))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error: received status code %d, response: %s", resp.StatusCode, string(body))
	}

	var ztRes ZoomTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&ztRes); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &ztRes, nil
}

// GenerateZoomMeeting creates a new Zoom meeting
func CreateZoomMeeting(booking *store.Booking) (*store.ZoomMeeting, error) {
	tokenResponse, err := GenerateZoomAccessToken()
	if err != nil {
		return nil, fmt.Errorf("error generating Zoom access token: %w", err)
	}

	// 2. Prepare the meeting time
	startTime, err := time.Parse(time.RFC3339, booking.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid time format: %w", err)
	}

	endTime, err := time.Parse(time.RFC3339, booking.EndTime)
	if err != nil {
		return nil, fmt.Errorf("invalid time format: %w", err)
	}

	// Zoom expects RFC3339 format in UTC
	zoomTimeFormatStart := startTime.UTC().Format(time.RFC3339)

	// Calculate minutes between start and end time
	minutes := endTime.Sub(startTime).Minutes()
	// Round to nearest minute and set duration

	// 3. Prepare meeting payload
	meetingPayload := map[string]interface{}{
		"topic":      booking.Topic,
		"agenda":     booking.AdditionalNotes,
		"type":       2, // Scheduled meeting
		"start_time": zoomTimeFormatStart,
		"duration":   minutes, // Duration in minutes
		"timezone":   "UTC",   // Or detect from user preferences
		"settings": map[string]interface{}{
			"host_video":         true,
			"participant_video":  true,
			"join_before_host":   true,
			"auto_start_meeting": true,  // Auto-start when first participant joins
			"waiting_room":       false, // More secure
			"mute_upon_entry":    false,
			"audio_setting":      "both", // Better for larger meetings
			"approval_type":      0,      // Auto-approve
			"auto_recording":     "none", // Or "cloud"/"local" if recording needed
			"alternative_hosts":  "",     // Could add alternative hosts here
		},
	}

	payloadBytes, err := json.Marshal(meetingPayload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling meeting payload: %w", err)
	}

	url := "https://api.zoom.us/v2/users/me/meetings"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating meeting request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokenResponse.AccessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making meeting request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error: received status code %d, response: %s", resp.StatusCode, string(body))
	}

	var meetingResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&meetingResponse); err != nil {
		return nil, fmt.Errorf("error decoding meeting response: %w", err)
	}

	// Convert meeting ID to int64 if it's a number or string
	var idInt int64
	if id, ok := meetingResponse["id"].(float64); ok {
		idInt = int64(id)
		meetingResponse["id"] = idInt
	} else if idStr, ok := meetingResponse["id"].(string); ok {
		_, err := fmt.Sscanf(idStr, "%d", &idInt)
		if err != nil || idInt == 0 {
			return nil, fmt.Errorf("invalid or missing meeting id in Zoom response")
		}
		meetingResponse["id"] = idInt
	} else {
		return nil, fmt.Errorf("missing meeting id in Zoom response")
	}

	// Marshal back to store.ZoomMeeting struct
	meetingBytes, err := json.Marshal(meetingResponse)
	if err != nil {
		return nil, fmt.Errorf("error re-marshalling meeting response: %w", err)
	}

	var zoomMeeting store.ZoomMeeting
	if err := json.Unmarshal(meetingBytes, &zoomMeeting); err != nil {
		return nil, fmt.Errorf("error unmarshalling into ZoomMeeting struct: %w", err)
	}

	// Defensive: ensure ID is not zero
	if zoomMeeting.MeetingID == 0 {
		return nil, fmt.Errorf("zoom meeting ID is zero after unmarshalling: %d", zoomMeeting.MeetingID)
	}

	return &zoomMeeting, nil
}

// DeleteZoomMeeting deletes a Zoom meeting by its ID
func DeleteZoomMeeting(meetingID int64) error {
	tokenResponse, err := GenerateZoomAccessToken()
	if err != nil {
		return fmt.Errorf("error generating Zoom access token: %w", err)
	}

	url := fmt.Sprintf("https://api.zoom.us/v2/meetings/%d", meetingID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("error creating delete request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokenResponse.AccessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error: received status code %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UpdateZoomMeeting updates an existing Zoom meeting
func UpdateZoomMeeting(meetingID int64, zm store.ZoomMeeting) error {
	tokenResponse, err := GenerateZoomAccessToken()
	if err != nil {
		return fmt.Errorf("error generating Zoom access token: %w", err)
	}

	// Prepare meeting payload
	meetingPayload := map[string]interface{}{
		"topic":    zm.Topic,
		"agenda":   zm.Agenda,
		"duration": zm.Duration,
		"settings": map[string]interface{}{
			"host_video":        true,
			"participant_video": true,
			"join_before_host":  false,
			"mute_upon_entry":   true,
			"approval_type":     0,
			"auto_recording":    "none",
			"alternative_hosts": "",
		},
	}

	payloadBytes, err := json.Marshal(meetingPayload)
	if err != nil {
		return fmt.Errorf("error marshalling meeting payload: %w", err)
	}

	url := fmt.Sprintf("https://api.zoom.us/v2/meetings/%d", meetingID)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating update request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokenResponse.AccessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error: received status code %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

func EndZoomMeeting(meetingID int64) error {
	tokenResponse, err := GenerateZoomAccessToken()
	if err != nil {
		return fmt.Errorf("error generating Zoom access token: %w", err)
	}

	url := fmt.Sprintf("https://api.zoom.us/v2/meetings/%d/status", meetingID)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("error creating end meeting request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokenResponse.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	// Set request body to end the meeting
	req.Body = io.NopCloser(strings.NewReader(`{"action":"end"}`))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making end meeting request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error: received status code %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}


func generateZoomSignature(apiKey, apiSecret, meetingNumber string, role int) string {
	timestamp := time.Now().UnixNano()/1e6 - 30000
	msg := fmt.Sprintf("%s%s%d%d", apiKey, meetingNumber, timestamp, role)

	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(msg))
	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	sig := fmt.Sprintf("%s.%s.%d.%d.%s", apiKey, meetingNumber, timestamp, role, hash)
	return base64.StdEncoding.EncodeToString([]byte(sig))
}

func GenerateZoomMeetingSDKSignature(clientID, clientSecret string, meetingNumber int64, role int64) (string, error) {
	// Generate iat and exp
	iat := time.Now().Unix() - 30
	exp := iat + 60*60*2 // 2 hours
	payload := jwt.MapClaims{
		"appKey":   clientID,
		"mn":       meetingNumber,
		"role":     role,
		"iat":      iat,
		"exp":      exp,
		"tokenExp": exp,
		// "video_webrtc_mode": 0,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	tokenString, err := token.SignedString([]byte(clientSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
