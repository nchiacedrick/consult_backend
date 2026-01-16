package payunit

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"consult_app.cedrickewi/internal/store"
	"consult_app.cedrickewi/internal/zoom"
)

const (
	CmOrange     = "CM_ORANGE"
	CmMtn        = "CM_MTN"
	PlatformFees = 1000
)

type Payunit struct {
	store store.Storage
}

// PayUnitRequest defines the payload structure
type PayUnitRequest struct {
	TotalAmount    int    `json:"total_amount"`
	Currency       string `json:"currency"`
	TransactionID  string `json:"transaction_id"`
	ReturnURL      string `json:"return_url"`
	NotifyURL      string `json:"notify_url"`
	PaymentCountry string `json:"payment_country"`
}

type PaymentRequest struct {
	Gateway       string `json:"gateway"`
	Amount        int    `json:"amount"`
	TransactionID string `json:"transaction_id"`
	ReturnURL     string `json:"return_url"`
	PhoneNumber   string `json:"phone_number"`
	Currency      string `json:"currency"`
	PaymentType   string `json:"paymentType"`
	NotifyURL     string `json:"notify_url"`
}

// InitializePayUnitTransaction sends a POST request to PayUnit
func (p *Payunit) InitializePayUnitTransaction(ctx context.Context, requestBody PayUnitRequest, bookingID int64, userID int64) (*store.PayUnitResponse, error) {
	PAYUNIT_API_USERNAME := os.Getenv("PAYUNIT_API_USERNAME")
	PAYUNIT_API_PASSWORD := os.Getenv("PAYUNIT_API_PASSWORD")
	PAYUNIT_API_TOKEN_SANDBOX := os.Getenv("PAYUNIT_API_TOKEN_SANDBOX")
	// PAYUNIT_API_TOKEN_LIVE := os.Getenv("PAYUNIT_API_TOKEN_LIVE")
	// PAYUNIT_BASE_URL := os.Getenv("PAYUNIT_BASE_URL")
	// PAYUNIT_MODE := os.Getenv("PAYUNIT_MODE")
	BackendURL := os.Getenv("BACKEND_URL")
	NotifyURL := fmt.Sprintf("%s/api/v1/payunit/notify?bookingID=%d", BackendURL, bookingID)

	url := "https://gateway.payunit.net/api/gateway/initialize"

	booking, err := p.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	transactionID := fmt.Sprintf("txn_%d_%d_%d", time.Now().UnixNano(), userID, booking.ID)

	payload := PayUnitRequest{
		TotalAmount:    booking.TotalAmount,
		Currency:       "XAF",
		TransactionID:  transactionID,
		ReturnURL:      "www.consult-out.com/dashboard/bookings",
		NotifyURL:      NotifyURL, //TODO: change notify URL to server url endpoint
		PaymentCountry: "CM",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	encodedValue := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", PAYUNIT_API_USERNAME, PAYUNIT_API_PASSWORD)))

	// Set headers
	req.Header.Set("x-api-key", PAYUNIT_API_TOKEN_SANDBOX)
	req.Header.Set("mode", "sandbox")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedValue))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Debug print raw response (optional)

	var result store.PayUnitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	initTtranxID, err := p.store.PayUnit.InsertInitializedTransaction(ctx, &result)
	if err != nil {
		return nil, err
	}

	if err := p.store.Booking.UpdateInitTransactionID(ctx, bookingID, initTtranxID); err != nil {
		return nil, err
	}

	if err := p.store.Booking.UpdateTransactionID(ctx, bookingID, transactionID); err != nil {
		return nil, err
	}

	// add the initialized transaction ID to the booking table
	return &result, nil
}

func (p *Payunit) GetProviders(ctx context.Context, bookingID int64) (*store.PayunitProvidersResponse, error) {
	PAYUNIT_API_USERNAME := os.Getenv("PAYUNIT_API_USERNAME")
	PAYUNIT_API_PASSWORD := os.Getenv("PAYUNIT_API_PASSWORD")
	PAYUNIT_API_TOKEN_SANDBOX := os.Getenv("PAYUNIT_API_TOKEN_SANDBOX")
	// PAYUNIT_API_TOKEN_LIVE := os.Getenv("PAYUNIT_API_TOKEN_LIVE")
	// PAYUNIT_BASE_URL := os.Getenv("PAYUNIT_BASE_URL")
	// PAYUNIT_MODE := os.Getenv("PAYUNIT_MODE")

	payunitTransaction, err := p.store.PayUnit.GetPayunitInitializationByTransactionID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://gateway.payunit.net/api/gateway/gateways?t_url=%s&t_id=%s&t_sum=%d", payunitTransaction.Data.TURL, payunitTransaction.Data.TID, payunitTransaction.Data.TSum)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	encodedValue := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", PAYUNIT_API_USERNAME, PAYUNIT_API_PASSWORD)))

	req.Header.Set("x-api-key", PAYUNIT_API_TOKEN_SANDBOX)
	req.Header.Set("mode", "sandbox")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedValue))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Providers response:", string(body))

	var result store.PayunitProvidersResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &result, nil
}

func (p *Payunit) MakePayment(ctx context.Context, requestBody PaymentRequest, bookingID int64, userID int64) (*store.PaymentResponse, error) {
	PAYUNIT_API_USERNAME := os.Getenv("PAYUNIT_API_USERNAME")
	PAYUNIT_API_PASSWORD := os.Getenv("PAYUNIT_API_PASSWORD")
	PAYUNIT_API_TOKEN_SANDBOX := os.Getenv("PAYUNIT_API_TOKEN_SANDBOX")
	BackendURL := os.Getenv("BACKEND_URL")
	frontendURL := os.Getenv("FRONTEND_URL")

	ReturnURL := fmt.Sprintf("%s/dashboard/booking/paymentsuccess", frontendURL)
	NotifyURL := fmt.Sprintf("%s/api/v1/payunit/notify?bookingID=%d", BackendURL, bookingID)

	url := "https://gateway.payunit.net/api/gateway/makepayment"

	booking, err := p.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	payload := PaymentRequest{
		Gateway:       requestBody.Gateway,
		Amount:        booking.TotalAmount,
		TransactionID: booking.TransactionID.String,
		ReturnURL:     ReturnURL,
		PhoneNumber:   requestBody.PhoneNumber,
		Currency:      "XAF",
		PaymentType:   "button",
		NotifyURL:     NotifyURL,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON: %v", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	encodedValue := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", PAYUNIT_API_USERNAME, PAYUNIT_API_PASSWORD)))

	// Set headers
	req.Header.Set("x-api-key", PAYUNIT_API_TOKEN_SANDBOX)
	req.Header.Set("mode", "sandbox")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedValue))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Payment response:", string(body))

	var result store.PaymentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	paymentID, err := p.store.PayUnit.InsertPayunitPayment(ctx, &result)
	if err != nil {
		return nil, err
	}

	if err := p.store.Booking.UpdatePayunitPaymentID(ctx, bookingID, paymentID); err != nil {
		return nil, err
	}

	return &result, nil
}

func (p *Payunit) GetPaymentStatus(ctx context.Context, bookingID int64) (*store.PaymentStatusResponse, error) {
	PAYUNIT_API_USERNAME := os.Getenv("PAYUNIT_API_USERNAME")
	PAYUNIT_API_PASSWORD := os.Getenv("PAYUNIT_API_PASSWORD")
	PAYUNIT_API_TOKEN_SANDBOX := os.Getenv("PAYUNIT_API_TOKEN_SANDBOX")

	// 1️⃣ Fetch the booking
	booking, err := p.store.Booking.GetByID(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve booking: %v", err)
	}

	if !booking.TransactionID.Valid {
		return nil, fmt.Errorf("booking has no transaction ID")
	}
	transactionID := booking.TransactionID.String

	// 2️⃣ Build request
	url := fmt.Sprintf("https://gateway.payunit.net/api/gateway/paymentstatus/%s", transactionID)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	encodedValue := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", PAYUNIT_API_USERNAME, PAYUNIT_API_PASSWORD)))
	req.Header.Set("x-api-key", PAYUNIT_API_TOKEN_SANDBOX)
	req.Header.Set("mode", "sandbox")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedValue))

	// 3️⃣ Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PayUnit request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	fmt.Println("📦 PayUnit Payment Status response:", string(body))

	var result store.PaymentStatusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// 4️⃣ Insert or update DB
	exists, err := p.store.PayUnit.CheckPaymentStatusExists(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	if !exists {
		if _, err := p.store.PayUnit.InsertPaymentStatus(ctx, &result); err != nil {
			return nil, fmt.Errorf("failed to insert payment status: %v", err)
		}
	}

	// 5️⃣ Update booking and handle business logic
	switch result.Data.TransactionStatus {
	case "PENDING":
		err = p.store.Booking.UpdatePaymentStatus(ctx, bookingID, "pending", booking.UserID)
	case "SUCCESS":
		if err := p.store.Booking.UpdatePaymentStatus(ctx, bookingID, "paid", booking.UserID); err != nil {
			return nil, fmt.Errorf("failed to update booking status: %v", err)
		}
		if booking.ZoomMeetingID.Valid {
			return &result, nil
		}
		meeting, err := zoom.CreateZoomMeeting(booking)
		if err != nil {
			return nil, fmt.Errorf("failed to create zoom meeting: %v", err)
		}
		zmtID, err := p.store.ZoomMeeting.Insert(ctx, meeting, booking.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to insert zoom meeting: %v", err)
		}
		booking.ZoomMeetingID = sql.NullInt64{Int64: zmtID, Valid: true}
		booking.BKStatus = "confirmed"
		if err = p.store.Booking.Update(ctx, booking); err != nil {
			return nil, fmt.Errorf("failed to update booking with zoom meeting ID: %v", err)
		}
  
	case "FAILED":
		err = p.store.Booking.UpdatePaymentStatus(ctx, bookingID, "failed", booking.UserID)
	case "CANCELLED":
		err = p.store.Booking.UpdatePaymentStatus(ctx, bookingID, "cancelled", booking.UserID)
	default:
		return nil, fmt.Errorf("unknown payment status: %s", result.Data.TransactionStatus)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update booking: %v", err)
	}

	return &result, nil
}

func NewPayunit(db *sql.DB) Payunit {
	storage := store.NewStorage(db)
	return Payunit{
		store: storage,
	}
}
