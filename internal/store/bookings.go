package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type BookingStatus string

func (s BookingStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusConfirmed, StatusCancelled, StatusCompleted:
		return true
	default:
		return false
	}
}

func (s BookingStatus) String() string {
	return string(s)
}

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusCancelled BookingStatus = "cancelled"
	StatusCompleted BookingStatus = "completed"
)

type Booking struct {
	ID                       int64          `json:"id"`
	UserID                   int64          `json:"user_id"`
	SlotID                   int64          `json:"slot_id"`
	ZoomMeetingID            sql.NullInt64  `json:"ref_meeting"`
	PaymentStatus            string         `json:"payment_status"`
	StartTime                string         `json:"start_time"`
	EndTime                  string         `json:"end_time"`
	Topic                    string         `json:"topic"`
	AdditionalNotes          string         `json:"additional_notes"`
	TotalAmount              int            `json:"total_amount"`
	TransactionID            sql.NullString `json:"transaction_id" example:"txn_123456789"`
	PayunitTransactionInitID sql.NullInt64  `json:"payunit_transaction_init_id"`
	PayunitPaymentID         sql.NullInt64  `json:"payunit_payment_id"`
	ExpertID                 int64          `json:"expert_id"`
	BKStatus                 string         `json:"bk_status"`
	UserReminder             int            `json:"user_reminder"`
	ExpertReminder           int            `json:"expert_reminder"`
	TimeRange                string         `json:"time_range"`
	CreatedAt                string         `json:"created_at"`
}

type CustomBooking struct {
	Booking      Booking     `json:"booking"`
	ZoomMeeting  ZoomMeeting `json:"zoom_meeting"`
	ExpertDetail Expert      `json:"expert"`
	Expert       User        `json:"expert_info"`
	UserDetails  User        `json:"user_details"`
}

type BookingStore struct {
	db *sql.DB
}

func (s *BookingStore) Insert(ctx context.Context, booking *Booking) error {
	query := `INSERT INTO 
	bookings (user_id, expert_id, start_time, end_time, topic, additional_notes, amount_to_pay)
	 VALUES ($1, $2, $3, $4, $5, $6, $7)
	 RETURNING id
	 `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query,
		booking.UserID, booking.ExpertID, booking.StartTime, booking.EndTime, booking.Topic, booking.AdditionalNotes, booking.TotalAmount,
	).Scan(&booking.ID)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			// Handle unique constraint violations
			switch pqErr.Constraint {
			case "no_user_overlap":
				return fmt.Errorf("booking overlaps with existing user booking")
			case "no_expert_overlap":
				return fmt.Errorf("booking overlaps with expert's schedule")
			}

			// Handle trigger-raised exceptions from enforce_booking_rules()
			switch pqErr.Message {
			case "Cannot book a session in the past.":
				return fmt.Errorf("you cannot book a session in the past")
			case "End time must be after start time.":
				return fmt.Errorf("the end time must be after the start time")
			case "Booking duration must be at least 30 minutes.":
				return fmt.Errorf("the booking must last at least 30 minutes")
			default:
				// Match trigger error pattern for availability
				if strings.Contains(pqErr.Message, "outside expert available hours") {
					return fmt.Errorf("the selected time is outside the expert’s available hours")
				}
				if strings.Contains(pqErr.Message, "An expert cannot book himself") {
					return fmt.Errorf("an expert cannot book themselves")
				}
			}
		}

		return err
	}

	return nil
}

// update booking
func (s *BookingStore) Update(ctx context.Context, booking *Booking) error {
	query := `
		UPDATE bookings
		SET zoom_meeting_id = $2, start_time = $3, end_time = $4, topic = $5, additional_notes = $6, bk_status = $7
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query,
		booking.ID, booking.ZoomMeetingID, booking.StartTime, booking.EndTime, booking.Topic, booking.AdditionalNotes, booking.BKStatus,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetByID retrieves a booking by its ID
func (s *BookingStore) GetByID(ctx context.Context, id int64) (*Booking, error) {
	query := `
		SELECT transaction_id, id, user_id, payment_status, created_at, start_time, end_time, expert_id, bk_status, time_range, payunit_transactions_init_id, payunit_payment_id, amount_to_pay, topic, additional_notes
		FROM bookings
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	booking := &Booking{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&booking.TransactionID,
		&booking.ID, &booking.UserID,
		&booking.PaymentStatus, &booking.CreatedAt, &booking.StartTime, &booking.EndTime, &booking.ExpertID, &booking.BKStatus, &booking.TimeRange,
		&booking.PayunitTransactionInitID, &booking.PayunitPaymentID, &booking.TotalAmount, &booking.Topic, &booking.AdditionalNotes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return booking, nil
}

// GetByTransactionID retrieves a booking by its transaction ID
func (s *BookingStore) GetByTransactionID(ctx context.Context, transactionID string) (*Booking, error) {
	query := `
		SELECT transaction_id, id, user_id, payment_status, created_at, start_time, end_time, expert_id, bk_status, time_range, payunit_transactions_init_id, payunit_payment_id, amount_to_pay
		FROM bookings
		WHERE transaction_id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	booking := &Booking{}
	err := s.db.QueryRowContext(ctx, query, transactionID).Scan(
		&booking.TransactionID,
		&booking.ID, &booking.UserID,
		&booking.PaymentStatus, &booking.CreatedAt, &booking.StartTime, &booking.EndTime, &booking.ExpertID, &booking.BKStatus, &booking.TimeRange,
		&booking.PayunitTransactionInitID, &booking.PayunitPaymentID, &booking.TotalAmount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return booking, nil
}

// Delete removes a booking from the database
func (s *BookingStore) Delete(ctx context.Context, id int64) error {
	query := `
		DELETE FROM bookings
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetAllUserBookings retrieves all bookings for a specific user from the database
func (s *BookingStore) GetAllUserBookings(ctx context.Context, userID int64) (*[]CustomBooking, error) {
	query := `
		SELECT COALESCE(b.transaction_id, '') AS transaction_id, b.id, b.user_id, b.start_time, b.end_time, b.expert_id, b.zoom_meeting_id, b.payment_status, b.created_at,b.bk_status,b.topic, b.additional_notes, ex.bio, ex.expertise, ex.fees_per_hr, ex.rating, ex.verified,
		expertinfo.id as expertinfo, expertinfo.username, expertinfo.email,
		zmt.agenda, COALESCE(zmt.meeting_url, '') AS meeting_url
		FROM bookings b
		JOIN experts ex ON b.expert_id = ex.id
		JOIN users expertinfo ON ex.user_id = expertinfo.id 
		LEFT JOIN zoom_meetings zmt ON b.zoom_meeting_id = zmt.id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []CustomBooking

	for rows.Next() {
		customBooking := CustomBooking{}
		err := rows.Scan(
			&customBooking.Booking.TransactionID,
			&customBooking.Booking.ID,
			&customBooking.Booking.UserID,
			&customBooking.Booking.StartTime,
			&customBooking.Booking.EndTime,
			&customBooking.Booking.ExpertID,
			&customBooking.Booking.ZoomMeetingID,
			&customBooking.Booking.PaymentStatus,
			&customBooking.Booking.CreatedAt,
			&customBooking.Booking.BKStatus,
			&customBooking.Booking.Topic,
			&customBooking.Booking.AdditionalNotes,
			&customBooking.ExpertDetail.Bio,
			&customBooking.ExpertDetail.Expertise,
			&customBooking.ExpertDetail.FeesPerHr,
			&customBooking.ExpertDetail.Rating,
			&customBooking.ExpertDetail.Verified,
			&customBooking.Expert.ID,
			&customBooking.Expert.Name,
			&customBooking.Expert.Email,
			&customBooking.ZoomMeeting.Agenda,
			&customBooking.ZoomMeeting.JoinURL,
		)

		if err != nil {
			return nil, err
		}
		bookings = append(bookings, customBooking)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &bookings, nil
}

// GetAllExpertPaidBookings retrieves all paid bookings for a specific expert from the database
func (s *BookingStore) GetAllExpertPaidBookings(ctx context.Context, expertID int64) (*[]CustomBooking, error) {
	query := `
		SELECT COALESCE(b.transaction_id, '') AS transaction_id,b.amount_to_pay, b.id, b.user_id, b.start_time, b.end_time, b.expert_id, b.zoom_meeting_id, b.payment_status, b.created_at,b.bk_status, ex.bio, ex.expertise, ex.fees_per_hr, ex.rating, ex.verified,
		expertinfo.id as expertinfo, expertinfo.username, expertinfo.email,
		zmt.agenda, COALESCE(zmt.meeting_url, '') AS meeting_url
		FROM bookings b
		JOIN experts ex ON b.expert_id = ex.id
		JOIN users expertinfo ON ex.user_id = expertinfo.id 
		LEFT JOIN zoom_meetings zmt ON b.zoom_meeting_id = zmt.id
		WHERE b.expert_id = $1 and b.payment_status = 'paid'
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, expertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []CustomBooking

	for rows.Next() {
		customBooking := CustomBooking{}
		err := rows.Scan(
			&customBooking.Booking.TransactionID,
			&customBooking.Booking.TotalAmount,
			&customBooking.Booking.ID,
			&customBooking.Booking.UserID,
			&customBooking.Booking.StartTime,
			&customBooking.Booking.EndTime,
			&customBooking.Booking.ExpertID,
			&customBooking.Booking.ZoomMeetingID,
			&customBooking.Booking.PaymentStatus,
			&customBooking.Booking.CreatedAt,
			&customBooking.Booking.BKStatus,
			&customBooking.ExpertDetail.Bio,
			&customBooking.ExpertDetail.Expertise,
			&customBooking.ExpertDetail.FeesPerHr,
			&customBooking.ExpertDetail.Rating,
			&customBooking.ExpertDetail.Verified,
			&customBooking.Expert.ID,
			&customBooking.Expert.Name,
			&customBooking.Expert.Email,
			&customBooking.ZoomMeeting.Agenda,
			&customBooking.ZoomMeeting.JoinURL,
		)

		if err != nil {
			return nil, err
		}
		bookings = append(bookings, customBooking)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &bookings, nil
}

// UpdateBookingPaymentDetails updates the payment details of a booking
func (s *BookingStore) UpdateBookingPaymentDetails(ctx context.Context, bookingID int64, zoomMeetingID int64, paymentStatus string) error {
	query := `
		UPDATE bookings
		SET zoom_meeting_id = $2, payment_status = $3
		WHERE id = $1	
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, bookingID, zoomMeetingID, paymentStatus)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetBookingDetails retrieves detailed information about a specific booking
func (s *BookingStore) GetBookingDetails(ctx context.Context, bookingID int64) (*CustomBooking, error) {
	query := `
		SELECT b.transaction_id,b.start_time, b.end_time, b.id, b.user_id, b.expert_id, b.zoom_meeting_id, b.payment_status, b.created_at,b.bk_status,b.topic,b.additional_notes,b.amount_to_pay,b.user_reminder, b.expert_reminder, ex.bio, ex.expertise, ex.fees_per_hr, ex.rating, ex.verified,ex.language,
		expertinfo.id as expertinfo, expertinfo.username, expertinfo.email,
		clientinfo.id as clientinfo, clientinfo.username, clientinfo.email,
		COALESCE(zmt.id, 0) AS zoom_id, COALESCE(zmt.zoom_meeting_id, 0) AS zoom_meeting_id, COALESCE(zmt.meeting_url, '') AS meeting_url, COALESCE(zmt.start_url, '') AS start_url, COALESCE(zmt.agenda, '') AS agenda, COALESCE(zmt.topic, '') AS zoom_topic, COALESCE(zmt.zoom_host_id, '') AS zoom_host_id, COALESCE(zmt.zoom_host_email, '') AS zoom_host_email, COALESCE(zmt.start_time, '1970-01-01 00:00:00'::timestamp) AS zoom_start_time, COALESCE(zmt.duration, 0) AS duration, COALESCE(zmt.password, '') AS password, COALESCE(zmt.zoom_status, 'scheduled') AS zoom_status, COALESCE(zmt.created_by, 0) AS created_by
		FROM bookings b
		JOIN experts ex ON b.expert_id = ex.id
		JOIN users expertinfo ON ex.user_id = expertinfo.id 
		JOIN users clientinfo ON b.user_id = clientinfo.id
		LEFT JOIN zoom_meetings zmt ON b.zoom_meeting_id = zmt.id
		WHERE b.id = $1
		ORDER BY b.created_at DESC
	`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	row := s.db.QueryRowContext(ctx, query, bookingID)
	if row.Err() != nil {
		if row.Err() == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, row.Err()
	}
	customBooking := &CustomBooking{}
	err := row.Scan(
		&customBooking.Booking.TransactionID,
		&customBooking.Booking.StartTime,
		&customBooking.Booking.EndTime,
		&customBooking.Booking.ID,
		&customBooking.Booking.UserID,
		&customBooking.Booking.ExpertID,
		&customBooking.Booking.ZoomMeetingID,
		&customBooking.Booking.PaymentStatus,
		&customBooking.Booking.CreatedAt,
		&customBooking.Booking.BKStatus,
		&customBooking.Booking.Topic,
		&customBooking.Booking.AdditionalNotes,
		&customBooking.Booking.TotalAmount,
		&customBooking.Booking.UserReminder,
		&customBooking.Booking.ExpertReminder,
		&customBooking.ExpertDetail.Bio,
		&customBooking.ExpertDetail.Expertise,
		&customBooking.ExpertDetail.FeesPerHr,
		&customBooking.ExpertDetail.Rating,
		&customBooking.ExpertDetail.Verified,
		&customBooking.ExpertDetail.Language,
		&customBooking.Expert.ID,
		&customBooking.Expert.Name,
		&customBooking.Expert.Email,
		&customBooking.UserDetails.ID,
		&customBooking.UserDetails.Name,
		&customBooking.UserDetails.Email,
		&customBooking.ZoomMeeting.ID,
		&customBooking.ZoomMeeting.MeetingID,
		&customBooking.ZoomMeeting.JoinURL,
		&customBooking.ZoomMeeting.StartURL,
		&customBooking.ZoomMeeting.Agenda,
		&customBooking.ZoomMeeting.Topic,
		&customBooking.ZoomMeeting.HostID,
		&customBooking.ZoomMeeting.HostEmail,
		&customBooking.ZoomMeeting.StartTime,
		&customBooking.ZoomMeeting.Duration,
		&customBooking.ZoomMeeting.Password,
		&customBooking.ZoomMeeting.Status,
		&customBooking.ZoomMeeting.CreatedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return customBooking, nil
}

// GetAllBookingsByBranchIDAndOrganisationID retrieves all bookings for a specific branch and organisation from the database
func (s *BookingStore) GetAllBookingsByBranchIDAndOrganisationID(ctx context.Context, branchID int64, organisationID int64) ([]*Booking, error) {
	query := `
		SELECT b.id, b.user_id, b.slot_id, b.zoom_meeting_id, b.payment_status, b.created_at
		FROM bookings b
		JOIN time_slots ts ON b.slot_id = ts.id
		JOIN branches br ON ts.branch_id = br.id
		WHERE br.id = $1 AND br.organisation_id = $2
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, branchID, organisationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*Booking

	for rows.Next() {
		booking := &Booking{}
		err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.SlotID,
			&booking.ZoomMeetingID,
			&booking.PaymentStatus,
			&booking.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return bookings, nil
}

// IsOwner checks if a user owns a specific booking
func (s *BookingStore) IsUserMeeting(ctx context.Context, userID int64, bookingID int64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE id = $1 AND user_id = $2
		)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists bool
	err := s.db.QueryRowContext(ctx, query, bookingID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// UpdatePaymentStatus updates the payment status of a booking
func (s *BookingStore) UpdatePaymentStatus(ctx context.Context, bookingID int64, paymentStatus string, userID int64) error {
	query := `
		UPDATE bookings
		SET payment_status = $2
		WHERE id = $1 AND user_id = $3
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, bookingID, paymentStatus, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// UpdateBookingStatus updates the status of a booking
func (s *BookingStore) UpdateBookingStatus(ctx context.Context, bookingID int64, bkStatus string) error {
	query := `
		UPDATE bookings
		SET bk_status = $2
		WHERE id = $1	
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, bookingID, strings.ToLower(bkStatus))
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// IsExpertMeeting checks if a booking is associated with a specific expert
func (s *BookingStore) IsExpertMeeting(ctx context.Context, bookingID int64, expertID int64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE id = $1 AND expert_id = $2
		)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists bool
	err := s.db.QueryRowContext(ctx, query, bookingID, expertID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *BookingStore) UpdateInitTransactionID(ctx context.Context, bookingID, initTransx int64) error {
	query := `
		UPDATE bookings
		SET payunit_transactions_init_id = $1
		WHERE id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, initTransx, bookingID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (s *BookingStore) UpdatePayunitPaymentID(ctx context.Context, bookingID, initTransx int64) error {
	query := `
		UPDATE bookings
		SET payunit_payment_id = $1
		WHERE id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, initTransx, bookingID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// update booking table: add transaction_id
func (s *BookingStore) UpdateTransactionID(ctx context.Context, bookingID int64, transactionID string) error {
	query := `
		UPDATE bookings
		SET transaction_id = $1
		WHERE id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	result, err := s.db.ExecContext(ctx, query, transactionID, bookingID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}


// update booking reminders
func (s *BookingStore) UpdateBookingReminders(ctx context.Context, bookingID int64, userReminder int, expertReminder int) error {
	query := `
		UPDATE bookings
		SET user_reminder = $1, expert_reminder = $2
		WHERE id = $3
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	result, err := s.db.ExecContext(ctx, query, userReminder, expertReminder, bookingID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}