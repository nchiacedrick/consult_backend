package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type PayUnitResponse struct {
	StatusCode int          `json:"statusCode"`
	Message    string       `json:"message"`
	Error      string       `json:"error"`
	Data       ResponseData `json:"data"`
	// You can add other fields according to PayUnit's API documentation
}

type ResponseData struct {
	TransactionID  string          `json:"transaction_id"`
	TransactionURL string          `json:"transaction_url"`
	TID            string          `json:"t_id"`
	TSum           string          `json:"t_sum"`
	TURL           string          `json:"t_url"`
	Providers      json.RawMessage `json:"providers"`
}

type Country struct {
	CountryName string `json:"country_name"`
	CountryCode string `json:"country_code"`
}

type PaymentResponse struct {
	Status     string              `json:"status"`
	StatusCode int                 `json:"statusCode"`
	Message    string              `json:"message"`
	Data       PaymentResponseData `json:"data"`
}

type PaymentResponseData struct {
	ID                    string `json:"id"`
	TransactionID         string `json:"transaction_id"`
	PaymentStatus         string `json:"payment_status"`
	Amount                int    `json:"amount"`
	ProviderTransactionID string `json:"provider_transaction_id"`
}

type PayunitProvidersResponse struct {
	Status     string                        `json:"status"`
	StatusCode int                           `json:"statusCode"`
	Message    string                        `json:"message"`
	Data       []PayunitProviderResponseData `json:"data"`
}

type PayunitProviderResponseData struct {
	ShortCode string `json:"shortcode"`
	Name      string `json:"name"`
	Logo      string `json:"logo"`
	Country   struct {
		CountryName string `json:"country_name"`
		CountryCode string `json:"country_code"`
	}
}

type PaymentStatusResponse struct {
	Status     string                    `json:"status"`
	StatusCode int                       `json:"statusCode"`
	Message    string                    `json:"message"`
	Data       PaymentStatusResponseData `json:"data"`
}

type PaymentStatusResponseData struct {
	TransactionAmount   int     `json:"transaction_amount"`
	TransactionStatus   string  `json:"transaction_status"`
	TransactionID       string  `json:"transaction_id"`
	PurchaseRef         *string `json:"purchase_ref"`
	NotifyURL           string  `json:"notify_url"`
	CallbackURL         string  `json:"callback_url"`
	TransactionCurrency string  `json:"transaction_currency"`
	TransactionGateway  string  `json:"transaction_gateway"`
	Message             string  `json:"message"`
}

type PayunitStore struct {
	db *sql.DB
}

// InsertInitializedTransaction Insert a payment initialization
func (s *PayunitStore) InsertInitializedTransaction(ctx context.Context, resp *PayUnitResponse) (int64, error) {
	query := `
	INSERT INTO payunit_transactions_init (
	payunit_t_id,
	payunit_t_sum,
	payunit_t_url,
	transaction_id,
	transaction_url,
	providers_json
	)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id;	
	`
	timeoutCtx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	var insertedID int64

	err := s.db.QueryRowContext(timeoutCtx, query, resp.Data.TID, resp.Data.TSum, resp.Data.TURL, resp.Data.TransactionID, resp.Data.TransactionURL, resp.Data.Providers).Scan(&insertedID)
	if err != nil {
		return 0, err
	}

	return insertedID, nil
}

func (s *PayunitStore) InsertPayunitPayment(ctx context.Context, payres *PaymentResponse) (int64, error) {
	query := ` INSERT INTO payunit_payments (
		transaction_id,
		amount,
		payunit_id,
		payment_status,
		provider_transaction_id
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id;
	`
	timeoutCtx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	var insertedID int64
	err := s.db.QueryRowContext(timeoutCtx, query, payres.Data.TransactionID, payres.Data.Amount, payres.Data.ID, payres.Data.PaymentStatus, payres.Data.ProviderTransactionID).Scan(&insertedID)
	return insertedID, err
}

func (s *PayunitStore) InsertPaymentStatus(ctx context.Context, payStatus *PaymentStatusResponse) (int64, error) {
	query := `INSERT INTO payunit_payment_status (
		transaction_id,
		transaction_amount,
		transaction_status,
		purchase_ref,
		notify_url,
		callback_url,
		transaction_currency,
		transaction_gateway,
		pps_message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id;
	`

	timeoutCtx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()
	var insertedID int64
	err := s.db.QueryRowContext(timeoutCtx, query, payStatus.Data.TransactionID, payStatus.Data.TransactionAmount, payStatus.Data.TransactionStatus, payStatus.Data.PurchaseRef, payStatus.Data.NotifyURL, payStatus.Data.CallbackURL, payStatus.Data.TransactionCurrency, payStatus.Data.TransactionGateway, payStatus.Data.Message).Scan(&insertedID)
	if err != nil {
		return 0, err
	}
	return insertedID, nil
}

func (s *PayunitStore) UpdatePaymentStatus(ctx context.Context, payStatus *PaymentStatusResponse) error {
	query := `UPDATE payunit_payment_status SET
		transaction_amount = $1,
		transaction_status = $2,
		purchase_ref = $3,
		notify_url = $4,
		callback_url = $5,
		transaction_currency = $6,
		transaction_gateway = $7,
		pps_message = $8
	WHERE transaction_id = $9
	`

	timeoutCtx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(timeoutCtx, query, payStatus.Data.TransactionAmount, payStatus.Data.TransactionStatus, payStatus.Data.PurchaseRef, payStatus.Data.NotifyURL, payStatus.Data.CallbackURL, payStatus.Data.TransactionCurrency, payStatus.Data.TransactionGateway, payStatus.Data.Message, payStatus.Data.TransactionID)
	if err != nil {
		return err
	}
	return nil
}

func (s *PayunitStore) CheckPaymentStatusExists(ctx context.Context, transactionID string) (bool, error) {
	query := `SELECT COUNT(1) FROM payunit_payment_status WHERE transaction_id = $1`
	var count int
	timeoutCtx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(timeoutCtx, query, transactionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetPayunitInitializationByTransactionID retrieves a PayUnit initialization by transaction ID
func (s *PayunitStore) GetPayunitInitializationByTransactionID(ctx context.Context, bookingID int64) (*PayUnitResponse, error) {
	query := `
		SELECT payunit_t_id, payunit_t_sum, payunit_t_url, bookings.transaction_id, transaction_url, providers_json
		FROM payunit_transactions_init
		JOIN bookings ON bookings.payunit_transactions_init_id = payunit_transactions_init.id
		WHERE bookings.id = $1;
	`
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var result PayUnitResponse
	err := s.db.QueryRowContext(timeoutCtx, query, bookingID).Scan(
		&result.Data.TID,
		&result.Data.TSum,
		&result.Data.TURL,
		&result.Data.TransactionID,
		&result.Data.TransactionURL,
		&result.Data.Providers,
	)

	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, fmt.Errorf("no PayUnit initialization found for booking ID: %d", bookingID)
		default:
			return nil, err
		}
	}

	return &result, nil
}
