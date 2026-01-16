package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"consult_app.cedrickewi/internal/payunit"
	"consult_app.cedrickewi/internal/store"
	"consult_app.cedrickewi/internal/twillio"
	"consult_app.cedrickewi/internal/zoom"
)

// ZoomPayload represents the payload for creating a Zoom meeting
type ZoomPayload struct {
	Agenda    string `json:"agenda" example:"Team meeting to discuss project progress"`
	Topic     string `json:"topic" example:"Project Progress Review"`
	StartTime string `json:"start_time" example:"2023-10-01T10:00:00Z"`
	EndTime   string `json:"end_time" example:"2023-10-01T11:00:00Z"`
}

// Handler to create a new booking
func (app *application) createBookingHandler(w http.ResponseWriter, r *http.Request) {
	expertID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()
	user := app.contextGetUser(r)

	payload := ZoomPayload{}
	if err := app.readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(&payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// save everthing in the booking table, this will contain all booked events

	expertInfo, err := app.store.Expert.GetExpertByID(ctx, expertID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	startTime, err := time.Parse(time.RFC3339, payload.StartTime)
	if err != nil {
		app.badRequestResponse(w, r, errors.New("invalid start time format, must be RFC3339"))
		return
	}
	endTime, err := time.Parse(time.RFC3339, payload.EndTime)
	if err != nil {
		app.badRequestResponse(w, r, errors.New("invalid end time format, must be RFC3339"))
		return
	}
	duration := endTime.Sub(startTime).Hours()
	totalAmount := int(expertInfo.FeesPerHr*duration) + payunit.PlatformFees // adding platform fees

	bk := store.Booking{
		UserID:          user.ID,
		ExpertID:        expertID,
		StartTime:       payload.StartTime,
		EndTime:         payload.EndTime,
		Topic:           payload.Topic,
		AdditionalNotes: payload.Agenda,
		TotalAmount:     totalAmount,
	}

	if err = app.store.Booking.Insert(ctx, &bk); err != nil {
		switch err.Error() {
		case "pq: ❌ Booking duration must be at least 30 minutes.":
			app.errorResponse(w, r, http.StatusBadRequest, "❌ Booking duration must be at least 30 minutes.")
			return
		case "booking overlaps with existing user booking":
			app.errorResponse(w, r, http.StatusConflict, "❌ You already have a booking that overlaps this time range.")
			return
		case "booking overlaps with expert's schedule":
			app.errorResponse(w, r, http.StatusConflict, "❌ This expert already has a booking during that time.")
			return
		case "pq: ❌ End time must be after start time.":
			app.errorResponse(w, r, http.StatusBadRequest, "❌ End time must be after start time.")
			return
		case "the selected time is outside the expert’s available hours":
			app.errorResponse(w, r, http.StatusBadRequest, "❌ The selected time is outside the expert’s available hours.")
			return
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"booking_created": bk}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type updateBookingStatusInput struct {
	BKStatus string `json:"bk_status"`
}

// Handler to approve a booking
func (app *application) approveBookingHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get booking ID from URL
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	payload := updateBookingStatusInput{}
	if err := app.readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(&payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	expert, err := app.store.Expert.GetExpertByUserID(ctx, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notPermittedResponse(w, r)
			return
		default:

			app.serverErrorResponse(w, r, err)
			return
		}
	}

	if user.ID != expert.UserID {
		app.notPermittedResponse(w, r)
		return
	}

	// check if expert owns meeting
	isOwner, err := app.store.Booking.IsExpertMeeting(ctx, id, expert.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}

	// Get the booking from database
	booking, err := app.store.Booking.GetByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	// Update the booking status to approved
	booking.BKStatus = payload.BKStatus

	if payload.BKStatus != "pending" && payload.BKStatus != "confirmed" && payload.BKStatus != "cancelled" && payload.BKStatus != "completed" {
		app.badRequestResponse(w, r, errors.New("invalid booking status - must be 'pending', 'confirmed', 'cancelled', or 'completed'"))
		return
	}
	if err = app.store.Booking.UpdateBookingStatus(ctx, booking.ID, booking.BKStatus); err != nil {
		app.internalServerError(w, r, err)
		return
	}

	// Return success response
	if err = app.writeJSON(w, http.StatusOK, envelope{"message": "booking approved successfully"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type initializePaymentInput struct {
	Currency       string `json:"currency" example:"XAF"`
	PaymentCountry string `json:"payment_country" example:"CM"`
}

// Handler to initialize booking payment
func (app *application) initializeBookingPaymentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := app.contextGetUser(r)

	bookingID, err := app.readIDParam(r, "id")
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	payload := initializePaymentInput{}

	if err := app.readJSON(w, r, &payload); err != nil {
		app.internalServerError(w, r, err)
		return
	}
	if err := Validate.Struct(&payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// check if booking belongs to the user
	isOwner, err := app.store.Booking.IsUserMeeting(ctx, user.ID, bookingID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}

	bk, err := app.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	//allow user to make payment using payunit
	paymentInitiale := payunit.PayUnitRequest{
		Currency:       payload.Currency,
		PaymentCountry: payload.PaymentCountry,
	}

	// prevent inititating multiple transactions for the same booking
	if bk.PayunitTransactionInitID != (sql.NullInt64{}) && bk.PayunitTransactionInitID.Valid {
		app.badRequestResponse(w, r, errors.New("payment transaction already initialized for this booking"))
		return
	}

	// if payment is successful, update payment_status in database
	payunitResp, err := app.payunit.InitializePayUnitTransaction(ctx, paymentInitiale, bk.ID, user.ID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"message": payunitResp}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type makePaymentPayload struct {
	Gateway     string `json:"gateway" example:"payunit"`
	PhoneNumber string `json:"phone_number" example:"+237612345678"`
}

// Hanlder to complete the payment and create zoom meeting
func (app *application) makePaymentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := app.contextGetUser(r)
	bookingID, err := app.readIDParam(r, "id")
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	payload := makePaymentPayload{}

	if err := app.readJSON(w, r, &payload); err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err := Validate.Struct(&payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// check if booking belongs to the user
	isOwner, err := app.store.Booking.IsUserMeeting(ctx, user.ID, bookingID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}

	bk, err := app.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	paymentRequest := payunit.PaymentRequest{
		Gateway:     payload.Gateway,
		PhoneNumber: payload.PhoneNumber,
	}

	// make payment
	_, err = app.payunit.MakePayment(ctx, paymentRequest, bk.ID, user.ID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"message": "payment initiated"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) rescheduleBookingHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := app.contextGetUser(r)

	// Get booking ID from URL
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	isOwner, err := app.store.Booking.IsUserMeeting(ctx, user.ID, id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}

	// Get the booking from database
	booking, err := app.store.Booking.GetByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	// Get the Zoom meeting details
	if !booking.ZoomMeetingID.Valid {
		app.notFoundResponse(w, r)
		return
	}
	meeting, err := app.store.ZoomMeeting.GetByID(ctx, booking.ZoomMeetingID.Int64)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	// Parse the update payload
	var input struct {
		Topic    string `json:"topic"`
		Agenda   string `json:"agenda"`
		Duration int64  `json:"duration"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Update the meeting details
	meeting.Topic = &input.Topic
	meeting.Agenda = &input.Agenda
	meeting.Duration = input.Duration

	// Update in Zoom
	err = zoom.UpdateZoomMeeting(meeting.MeetingID, *meeting)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	// Update in database
	err = app.store.ZoomMeeting.Update(ctx, meeting)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	// Return the updated meeting
	err = app.writeJSON(w, http.StatusOK, envelope{"meeting": meeting}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// Get all booking for a user
func (app *application) getAllBookingsForUser(w http.ResponseWriter, r *http.Request) {
	// id, err := app.readIDParam(r, "id")
	user := app.contextGetUser(r)
	ctx := r.Context()
	bookx, err := app.store.Booking.GetAllUserBookings(ctx, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"bookings": bookx}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) getABookingForUser(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")

	// user := app.contextGetUser(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()
	bookx, err := app.store.Booking.GetBookingDetails(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	if bookx.Booking.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	// TODO: check if payment have been made

	if err := app.writeJSON(w, http.StatusOK, envelope{"booking": bookx}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

func (app *application) getABookingForExpert(w http.ResponseWriter, r *http.Request) {
	bookingID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	ctx := r.Context()

	user := app.contextGetUser(r)

	isExpert, err := app.store.Expert.IsExpert(ctx, user.ID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if !isExpert {
		app.notPermittedResponse(w, r)
		return
	}

	expert, err := app.store.Expert.GetExpertByUserID(ctx, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notPermittedResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	isExpertMtg, err := app.store.Booking.IsExpertMeeting(ctx, bookingID, expert.ID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if !isExpertMtg {
		app.notPermittedResponse(w, r)
		return
	}

	bookx, err := app.store.Booking.GetBookingDetails(ctx, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if bookx.Booking.ExpertID != expert.ID {
		app.notPermittedResponse(w, r)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"booking": bookx}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

// PayunitWebhookHandler handles Payunit payment status webhooks
// payunitGetPaymentStatusHandler handles retrieving and updating payment status for a booking.
func (app *application) payunitWebHookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	app.logger.Infof("📥 Received PayUnit webhook from %s", r.RemoteAddr)

	// STEP 1️⃣ — Log request headers
	for name, values := range r.Header {
		app.logger.Infof("Header %s: %v", name, values)
	}

	// STEP 2️⃣ — Read and log raw body for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		app.logger.Errorf("❌ Failed to read webhook body: %v", err)
		app.badRequestResponse(w, r, fmt.Errorf("failed to read webhook body"))
		return
	}
	app.logger.Infof("📦 Raw webhook body: %s", string(bodyBytes))
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // reset for decoding

	// STEP 3️⃣ — Decode JSON payload
	var payload struct {
		TransactionStatus   string  `json:"transaction_status"`
		TransactionGateway  string  `json:"transaction_gateway"`
		TransactionAmount   float64 `json:"transaction_amount"`
		TransactionID       string  `json:"transaction_id"`
		Message             string  `json:"message"`
		TransactionCurrency string  `json:"transaction_currency"`
		NotifyURL           string  `json:"notify_url"`
		CallbackURL         string  `json:"callback_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		app.logger.Errorf("❌ Invalid webhook payload: %v", err)
		app.badRequestResponse(w, r, fmt.Errorf("invalid webhook payload"))
		return
	}

	app.logger.Infof("✅ Decoded PayUnit Payload: %+v", payload)

	// STEP 4️⃣ — Get bookingID from query param
	bookingIDStr := r.URL.Query().Get("bookingID")
	if bookingIDStr == "" {
		app.badRequestResponse(w, r, fmt.Errorf("missing bookingID in webhook URL"))
		return
	}
	_, err = strconv.ParseInt(bookingIDStr, 10, 64)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("invalid bookingID format"))
		return
	}

	// STEP 5️⃣ — Environment-based security
	appMode := os.Getenv("PAYUNIT_MODE") // "sandbox" or "live"

	if strings.ToLower(appMode) == "live" {
		// Verify request IP (PayUnit’s servers usually use DigitalOcean IPs)
		if !strings.HasPrefix(r.RemoteAddr, "138.197.") {
			app.logger.Warnf("🚨 Unauthorized IP: %s (expected PayUnit server)", r.RemoteAddr)
			http.Error(w, "unauthorized webhook source", http.StatusUnauthorized)
			return
		}
		app.logger.Info("✅ Valid webhook source detected (live mode)")
	} else {
		app.logger.Info("⚠️ Sandbox mode — skipping strict verification")
	}

	// STEP 6️⃣ — Verify transaction in DB
	booking, err := app.store.Booking.GetByTransactionID(ctx, payload.TransactionID)
	if err != nil {
		app.logger.Errorf("❌ Booking not found for transaction ID %s: %v", payload.TransactionID, err)
		app.badRequestResponse(w, r, fmt.Errorf("booking not found for transaction ID %s", payload.TransactionID))
		return
	}

	if !booking.TransactionID.Valid || booking.TransactionID.String != payload.TransactionID {
		app.logger.Errorf("🚫 Transaction mismatch for booking %d", booking.ID)
		app.badRequestResponse(w, r, fmt.Errorf("transaction mismatch"))
		return
	}

	app.logger.Infof("✅ Verified booking #%d for transaction %s", booking.ID, payload.TransactionID)

	// STEP 7️⃣ — Trigger status verification (updates DB + creates Zoom)
	result, err := app.payunit.GetPaymentStatus(ctx, booking.ID)
	if err != nil {
		app.logger.Errorf("❌ Failed to verify payment status: %v", err)
		app.serverErrorResponse(w, r, fmt.Errorf("failed to verify payment status: %v", err))
		return
	}

	app.logger.Infof("💰 Payment verification complete for booking #%d — Status: %s",
		booking.ID, result.Data.TransactionStatus)

	// STEP 8️⃣ — Return success response to PayUnit
	app.writeJSON(w, http.StatusOK, envelope{
		"status":  "success",
		"message": fmt.Sprintf("Webhook processed successfully for booking %d", booking.ID),
	}, nil)
}

func (app *application) zoomWebhookHandler(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("X-Zm-Signature")
	timestamp := r.Header.Get("X-Zm-Request-Timestamp")

	app.logger.Infof("Received Headers: %+v", r.Header)

	if signature == "" || timestamp == "" {
		app.logger.Warnln("Missing signature or timestamp header")
		http.Error(w, "invalid or missing authentication token", http.StatusUnauthorized)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	message := fmt.Sprintf("v0:%s:%s", timestamp, string(bodyBytes))
	// app.logger.Infof("Constructed Message for Signature: %s", message) // Added logging for the message

	secret := os.Getenv("ZOOM_WEBHOOK_SECRET_TOKEN")
	// app.logger.Infof("ZOOM_WEBHOOK_SECRET_TOKEN: %s", secret)

	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(message))
	expectedSignature := "v0=" + hex.EncodeToString(hash.Sum(nil))

	if hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			app.logger.Errorf("JSON unmarshal error: %v", err)
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Convert the meeting ID to a string if it's a number
		if object, ok := payload["payload"].(map[string]interface{})["object"].(map[string]interface{}); ok {
			if id, ok := object["id"].(float64); ok {
				object["id"] = fmt.Sprintf("%.0f", id)
			}
		}

		// Marshal back to the ZoomWebhook struct
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			app.logger.Errorf("Error re-marshalling payload: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		var zoomPayload ZoomWebhook
		if err := json.Unmarshal(payloadBytes, &zoomPayload); err != nil {
			app.logger.Errorf("Error unmarshalling into ZoomWebhook struct: %v", err)
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		switch zoomPayload.Event {
		case "endpoint.url_validation":
			validateHash := hmac.New(sha256.New, []byte(secret))
			validateHash.Write([]byte(zoomPayload.Payload.PlainToken))
			encryptedToken := hex.EncodeToString(validateHash.Sum(nil))

			response := map[string]string{
				"plainToken":     zoomPayload.Payload.PlainToken,
				"encryptedToken": encryptedToken,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
			return

		case "meeting.participant_joined":
			joinTimeStr := zoomPayload.Payload.Object.Participant.JoinTime // Use JoinTime
			joinTime, _ := time.Parse(time.RFC3339, joinTimeStr)
			// First get the meeting ID from your database using the Zoom meeting ID
			meetingID, err := strconv.ParseInt(zoomPayload.Payload.Object.ID, 10, 64)
			if err != nil {
				app.logger.Errorf("Error parsing meeting ID: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

			app.logger.Infof("Getting Meeting from DB with ID: %s", meetingID)
			meeting, err := app.store.ZoomMeeting.GetByZoomID(r.Context(), meetingID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					app.logger.Infof("Meeting not found in DB: %d ", meetingID)
					// You can optionally save the meeting or skip processing.
					return
				} else if errors.Is(err, store.ErrRecordNotFound) {
					app.logger.Infof("Meeting not found in DB: %d ", meetingID)
					return
				} else {
					app.logger.Errorf("Error getting meeting by Zoom ID: %v", err)
					app.serverErrorResponse(w, r, err)
					return
				}
			}

			app.logger.Infof("Scheduling end meeting task for meeting ID: %d", meetingID)
			if _, err := app.mtgschelduler.ScheduleEndMeeting(r.Context(), meetingID, meeting.StartTime.Add(time.Duration(meeting.Duration)*time.Minute).UTC()); err != nil {
				app.logger.Errorf("Error scheduling end meeting task: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

			participant := store.MeetingParticipant{
				MeetingID:        meeting.ID, // Use the meeting ID from your database
				ZoomMeetingID:    meetingID,
				MeetingUUID:      zoomPayload.Payload.Object.UUID,
				ParticipantID:    zoomPayload.Payload.Object.Participant.UserID,
				ParticipantName:  zoomPayload.Payload.Object.Participant.UserName,
				ParticipantEmail: zoomPayload.Payload.Object.Participant.Email,
				JoinedAt:         joinTime,
				UserID:           meeting.CreatedBy,
			}

			app.logger.Infof("Inserting participant joined: %+v", participant)
			if err := app.store.MeetingParticipant.InsertParticipantJoined(r.Context(), &participant); err != nil {
				app.logger.Errorf("Error inserting participant joined: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

		case "meeting.participant_left":
			leaveTimeStr := zoomPayload.Payload.Object.Participant.LeaveTime // Use LeaveTime
			leftTime, err := time.Parse(time.RFC3339, leaveTimeStr)
			if err != nil {
				app.logger.Errorf("Error parsing leave time: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}
			zoomMeetingID, err := strconv.ParseInt(zoomPayload.Payload.Object.ID, 10, 64)
			if err != nil {
				app.logger.Errorf("Error parsing meeting ID: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}
			// First get the meeting ID from your database using the Zoom meeting ID
			meeting, err := app.store.ZoomMeeting.GetByZoomID(r.Context(), zoomMeetingID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					app.logger.Info("Meeting not found in DB: ", zoomPayload.Payload.Object.ID)
					// You can optionally save the meeting or skip processing.
					return
				} else if errors.Is(err, store.ErrRecordNotFound) {
					app.logger.Info("Meeting not found in DB: ", zoomPayload.Payload.Object.ID)
					return
				} else {
					app.logger.Errorf("Error getting meeting by Zoom ID: %v", err)
					app.serverErrorResponse(w, r, err)
					return
				}
			}
			participant := store.MeetingParticipant{
				MeetingID:     meeting.ID, // Use the meeting ID from your database
				LeftAt:        leftTime,
				ZoomMeetingID: zoomMeetingID,
				ParticipantID: zoomPayload.Payload.Object.Participant.UserID,
			}

			if err := app.store.MeetingParticipant.UpdateParticipantLeft(r.Context(), &participant); err != nil {
				app.logger.Errorf("Error updating participant left: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

		case "meeting.ended":
			meetingID, err := strconv.ParseInt(zoomPayload.Payload.Object.ID, 10, 64)
			if err != nil {
				app.logger.Errorf("Error parsing meeting ID: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}
			app.logger.Infof("Getting Meeting from DB with ID: %s", meetingID)
			meeting, err := app.store.ZoomMeeting.GetByZoomID(r.Context(), meetingID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					app.logger.Infof("Meeting not found in DB: %d", meetingID)
					// You can optionally save the meeting or skip processing.
					return
				} else if errors.Is(err, store.ErrRecordNotFound) {
					app.logger.Infof("Meeting not found in DB: %d", meetingID)
					return
				} else {
					app.logger.Errorf("Error getting meeting by Zoom ID: %v", err)
					app.serverErrorResponse(w, r, err)
					return
				}
			}

			meeting.Status = "ended"
			meeting.Duration = zoomPayload.Payload.Object.Duration
			if err := app.store.ZoomMeeting.Update(r.Context(), meeting); err != nil {
				app.logger.Errorf("Error updating meeting status: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}
			zoomMtgID, err := strconv.ParseInt(zoomPayload.Payload.Object.ID, 10, 64)
			if err != nil {
				app.logger.Errorf("Error parsing meeting ID: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

			if err := zoom.DeleteZoomMeeting(zoomMtgID); err != nil {
				app.logger.Errorf("Error deleting Zoom meeting: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

		case "meeting.started":
			meetingID, err := strconv.ParseInt(zoomPayload.Payload.Object.ID, 10, 64)
			if err != nil {
				app.logger.Errorf("Error parsing meeting ID: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

			app.logger.Infof("Getting Meeting from DB with ID: %s", meetingID)
			meeting, err := app.store.ZoomMeeting.GetByZoomID(r.Context(), meetingID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					app.logger.Infof("Meeting not found in DB: %d ", meetingID)
					// You can optionally save the meeting or skip processing.
					return
				} else if errors.Is(err, store.ErrRecordNotFound) {
					app.logger.Infof("Meeting not found in DB: %d", meetingID)
					return
				} else {
					app.logger.Errorf("Error getting meeting by Zoom ID: %v", err)
					app.serverErrorResponse(w, r, err)
					return
				}
			}
			

			meeting.Status = "started"
			startTime, err := time.Parse(time.RFC3339, zoomPayload.Payload.Object.StartTime)
			if err != nil {
				app.logger.Errorf("Error parsing start time: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

			meeting.StartTime = startTime
			meeting.Duration = zoomPayload.Payload.Object.Duration
			if err := app.store.ZoomMeeting.Update(r.Context(), meeting); err != nil {
				app.logger.Errorf("Error updating meeting status: %v", err)
				app.serverErrorResponse(w, r, err)
				return
			}

		default:
			app.logger.Warnf("Received unhandled event type: %s", zoomPayload.Event)
			w.WriteHeader(http.StatusOK) // Or handle differently
			return
		}

		// Custom business logic goes here (e.g., Zoom API interaction)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Authorized request to Zoom Webhook sample.",
		})
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Unauthorized request to Zoom Webhook sample.",
		})
	}
}

func (app *application) getSignatureHandler(w http.ResponseWriter, r *http.Request) {
	var req zoom.SignatureRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Validate the request
	if err := Validate.Struct(req); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	clientID := os.Getenv("ZOOM_SDK_CLIENT_ID")
	clientSecret := os.Getenv("ZOOM_SDK_CLIENT_SECRET")

	signature, err := zoom.GenerateZoomMeetingSDKSignature(clientID, clientSecret, req.MeetingNumber, req.Role)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var res zoom.SignatureResponse
	res.Signature = signature
	res.SDKKey = clientID

	// Return the signature
	if err := app.writeJSON(w, http.StatusOK, envelope{"signature": res}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// Handler to send booking reminder to expert
func (app *application) sendBookingReminderToExpertHandler(w http.ResponseWriter, r *http.Request) {
	bookingID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()
	user := app.contextGetUser(r)

	// check if booking belongs to the user
	isOwner, err := app.store.Booking.IsUserMeeting(ctx, user.ID, bookingID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}

	booking, err := app.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
	}

	expert, err := app.store.Expert.GetUserByExpertID(ctx, booking.ExpertID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
	}

	// send sms to user and expert
	smsPayload := twillio.TwillioSendMessagePayload{
		To: expert.Phone,
		Message: fmt.Sprintf(
			"Reminder: %s is waiting for you in the meeting (booking #%d) scheduled from %s to %s. Please join now.",
			user.Name, booking.ID, booking.StartTime, booking.EndTime,
		),
	}

	if _, err := twillio.SendMessage(smsPayload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"message": "Reminder sent successfully"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

// Handler to send booking reminder to user
func (app *application) sendBookingReminderToUserHandler(w http.ResponseWriter, r *http.Request) {
	bookingID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()
	user := app.contextGetUser(r)
	// check if booking belongs to the user
	isOwner, err := app.store.Booking.IsUserMeeting(ctx, user.ID, bookingID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}
	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}
	booking, err := app.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
	}

	expert, err := app.store.Expert.GetUserByExpertID(ctx, booking.ExpertID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.internalServerError(w, r, err)
		}
	}

	// send sms to user
	smsPayload := twillio.TwillioSendMessagePayload{
		To: user.Phone,
		Message: fmt.Sprintf(
			"Reminder: %s is waiting for you in the meeting (booking #%d) scheduled from %s to %s. Please join now.",
			expert.Name, booking.ID, booking.StartTime, booking.EndTime,
		),
	}

	if _, err := twillio.SendMessage(smsPayload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"message": "Reminder sent successfully"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

