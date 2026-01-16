package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Expert struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"user_id"`
	Expertise   string  `json:"expertise"`
	Bio         string  `json:"bio"`
	FeesPerHr   float64 `json:"fees_per_hr"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	Name        string  `json:"name"`
	Language    string  `json:"language"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	TotalEarned float64 `json:"total_earned"`
	Verified    bool    `json:"verified"`
	Rating      float64 `json:"rating"`
	Version     int64   `json:"version"`
}

type ExpertAvailability struct {
	ID        int64  `json:"id"`
	ExpertID  int64  `json:"expert_id"`
	Day       string `json:"day"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	IsWeekend bool   `json:"is_weekend"`
	CreatedAt string `json:"created_at"`
}

type ExpertBranch struct {
	ID       int64 `json:"id"`
	ExpertID int64 `json:"expert_id"`
	BranchID int64 `json:"branch_id"`
}

type ExpertsStore struct {
	db *sql.DB
}

// Insert an expert
func (s *ExpertsStore) Insert(ctx context.Context, expert *Expert) error {
	query := `
		INSERT INTO experts (user_id, expertise, bio, fees_per_hr, language) 
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, version
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if err := s.db.QueryRowContext(ctx, query, expert.UserID, expert.Expertise, expert.Bio, expert.FeesPerHr, expert.Language).Scan(&expert.ID, &expert.Version); err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "experts_user_id_key"`:
			return ErrDuplicateExpert
		default:
			return err
		}
	}

	return nil
}

// InsertToBranch Add an expert to a branch
func (s *ExpertsStore) InsertToBranch(ctx context.Context, branch *ExpertBranch) error {
	query := `INSERT INTO expert_branches(expert_id, branch_id) 
	VALUES($1, $2) RETURNING id`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if err := s.db.QueryRowContext(ctx, query, branch.ExpertID, branch.BranchID).Scan(&branch.ID); err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "expert_branches_expert_id_branch_id_key"`:
			return ErrDuplicatExpertBranch
		default:
			return err
		}
	}

	return nil
}

// List of all branches an expert belongs
func (s *ExpertsStore) ExpertBranches(ctx context.Context, expertID int64) (*[]Branch, error) {
	query := `
		SELECT b.id, b.branch_name, b.about_branch, b.organisation_id, b.created_at, b.updated_at
		FROM branches b
		INNER JOIN expert_branches eb ON eb.branch_id = b.id
		WHERE eb.expert_id = $1
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	results, err := s.db.QueryContext(ctx, query, expertID)
	if err != nil {
		return nil, err
	}

	var userBranches []Branch
	for results.Next() {
		var branch Branch

		err = results.Scan(
			&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		userBranches = append(userBranches, branch)
	}

	return &userBranches, err
}

// IsExpert checks if a user is an expert
func (s *ExpertsStore) IsExpert(ctx context.Context, userID int64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM experts 
			WHERE user_id = $1
		)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists bool
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetExpertByUserID gets an expert by user ID
func (s *ExpertsStore) GetExpertByUserID(ctx context.Context, userID int64) (*Expert, error) {
	query := `
		SELECT id, user_id, expertise, bio, fees_per_hr
		FROM experts 
		WHERE user_id = $1  
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var expert Expert
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&expert.ID, &expert.UserID, &expert.Expertise, &expert.Bio, &expert.FeesPerHr,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return &expert, nil
}

// GetExpertByID gets an expert by ID
func (s *ExpertsStore) GetExpertByID(ctx context.Context, id int64) (*Expert, error) {
	query := `
		SELECT e.id, e.user_id, e.expertise, e.bio, e.fees_per_hr,
			   u.username, u.email, u.phone
		FROM experts e
		INNER JOIN users u ON u.id = e.user_id
		WHERE e.id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var expert Expert
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&expert.ID, &expert.UserID, &expert.Expertise, &expert.Bio, &expert.FeesPerHr, &expert.Name,
		&expert.Email, &expert.Phone,
	)
	if err != nil {
		return nil, err
	}

	return &expert, nil
}

// GetUserByExpertID gets a user by expert ID
func (s *ExpertsStore) GetUserByExpertID(ctx context.Context, expertID int64) (*User, error) {
	query := `
		SELECT u.id, u.email, u.created_at, u.updated_at
		FROM users u
		INNER JOIN experts e ON e.user_id = u.id
		WHERE e.id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user User
	err := s.db.QueryRowContext(ctx, query, expertID).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (s *ExpertsStore) GetAllExperts(ctx context.Context, userID int64) (*[]Expert, error) {
	query := `
		SELECT e.id, e.user_id, e.expertise, e.bio, e.fees_per_hr,
			   u.username, u.email, u.phone
		FROM experts e
		INNER JOIN users u ON u.id = e.user_id
		WHERE e.user_id <> $1;
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var experts []Expert
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var expert Expert
		err := rows.Scan(
			&expert.ID,
			&expert.UserID,
			&expert.Expertise,
			&expert.Bio,
			&expert.FeesPerHr,
			&expert.Name,
			&expert.Email,
			&expert.Phone,
		)
		if err != nil {
			return nil, err
		}
		experts = append(experts, expert)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &experts, nil
}

// update expert
func (s *ExpertsStore) UpdateExpert(ctx context.Context, expert *Expert) error {
	query := `
		UPDATE experts	
		SET expertise = $1, bio = $2, fees_per_hr = $3
		WHERE id = $4
		RETURNING id, version
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if err := s.db.QueryRowContext(ctx, query, expert.Expertise, expert.Bio, expert.FeesPerHr, expert.ID).Scan(&expert.ID, &expert.Version); err != nil {
		return err
	}

	return nil
}

// RemoveExpertFromBranch removes an expert from a branch
func (s *ExpertsStore) RemoveExpertFromBranch(ctx context.Context, expertID int64, branchID int64) error {
	query := `
		DELETE FROM expert_branches
		WHERE expert_id = $1 AND branch_id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, expertID, branchID)
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

type Consultation struct {
	Booking     Booking     `json:"booking"`
	Expert      Expert      `json:"expert"`
	User        User        `json:"user"`
	ZoomMeeting ZoomMeeting `json:"zoom_meeting"`
}

func (s *ExpertsStore) GetAllExpertConsultations(ctx context.Context, expertID int64) (*[]Consultation, error) {
	query := `
	SELECT b.amount_to_pay, b.id, b.topic, b.additional_notes, b.bk_status, b.payment_status, b.start_time, b.end_time, b.created_at, COALESCE(b.zoom_meeting_id, 0) AS zoom_meeting_id,
	u.username, u.email, COALESCE(u.image_url, '') AS image_url,
	COALESCE(zm.meeting_url, '') AS meeting_url, COALESCE(zm.start_url, '') AS start_url, COALESCE(zm.zoom_status, 'scheduled') AS zoom_status
	FROM bookings b 
	JOIN users u ON b.user_id = u.id
	LEFT JOIN zoom_meetings zm ON zm.id = b.zoom_meeting_id
	WHERE expert_id = $1   
	ORDER BY b.start_time DESC
	`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var consultations []Consultation
	rows, err := s.db.QueryContext(ctx, query, expertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var consultation Consultation
		err := rows.Scan(
			&consultation.Booking.TotalAmount,
			&consultation.Booking.ID,
			&consultation.Booking.Topic,
			&consultation.Booking.AdditionalNotes,
			&consultation.Booking.BKStatus,
			&consultation.Booking.PaymentStatus,
			&consultation.Booking.StartTime,
			&consultation.Booking.EndTime,
			&consultation.Booking.CreatedAt,
			&consultation.Booking.ZoomMeetingID,
			&consultation.User.Name,
			&consultation.User.Email,
			&consultation.User.ImageURL,
			&consultation.ZoomMeeting.JoinURL,
			&consultation.ZoomMeeting.StartURL,
			&consultation.ZoomMeeting.Status,
		)
		if err != nil {
			return nil, err
		}
		consultations = append(consultations, consultation)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &consultations, nil
}

// GetAnExpertConsultationByBookingID gets a consultation by booking ID
func (s *ExpertsStore) GetAnExpertConsultationByBookingID(ctx context.Context, bookingID int64, expertID int64) (*Consultation, error) {
	query := `
	SELECT b.transaction_id,b.start_time, b.end_time, b.id, b.user_id, b.expert_id, b.zoom_meeting_id, b.payment_status, b.created_at,b.bk_status,b.topic,b.additional_notes,b.amount_to_pay, ex.bio, ex.expertise, ex.fees_per_hr, ex.rating, ex.verified,ex.language,
		expertinfo.id as expertinfo, expertinfo.username, expertinfo.email,
		clientinfo.id as clientinfo, clientinfo.username, clientinfo.email,
		COALESCE(zmt.id, 0) AS zoom_id, COALESCE(zmt.zoom_meeting_id, 0) AS zoom_meeting_id, COALESCE(zmt.meeting_url, '') AS meeting_url, COALESCE(zmt.start_url, '') AS start_url, COALESCE(zmt.agenda, '') AS agenda, COALESCE(zmt.topic, '') AS zoom_topic, COALESCE(zmt.zoom_host_id, '') AS zoom_host_id, COALESCE(zmt.zoom_host_email, '') AS zoom_host_email, COALESCE(zmt.start_time, '1970-01-01 00:00:00'::timestamp) AS zoom_start_time, COALESCE(zmt.duration, 0) AS duration, COALESCE(zmt.password, '') AS password, COALESCE(zmt.zoom_status, 'scheduled') AS zoom_status, COALESCE(zmt.created_by, 0) AS created_by
		FROM bookings b
		JOIN experts ex ON b.expert_id = ex.id
		JOIN users expertinfo ON ex.user_id = expertinfo.id 
		JOIN users clientinfo ON b.user_id = clientinfo.id
		LEFT JOIN zoom_meetings zmt ON b.zoom_meeting_id = zmt.id
		WHERE b.id = $1 AND b.expert_id = $2
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var consultation Consultation
	err := s.db.QueryRowContext(ctx, query, bookingID, expertID).Scan(
		&consultation.Booking.TransactionID,
		&consultation.Booking.StartTime,
		&consultation.Booking.EndTime,
		&consultation.Booking.ID,
		&consultation.Booking.UserID,
		&consultation.Booking.ExpertID,
		&consultation.Booking.ZoomMeetingID,
		&consultation.Booking.PaymentStatus,
		&consultation.Booking.CreatedAt,
		&consultation.Booking.BKStatus,
		&consultation.Booking.Topic,
		&consultation.Booking.AdditionalNotes,
		&consultation.Booking.TotalAmount,
		&consultation.Expert.Bio,
		&consultation.Expert.Expertise,
		&consultation.Expert.FeesPerHr,
		&consultation.Expert.Rating,
		&consultation.Expert.Verified,
		&consultation.Expert.Language,
		&consultation.User.ID,
		&consultation.User.Name,
	)
	if err != nil {
		return nil, err
	}

	return &consultation, nil
}

// GetExpertAvailability gets availability for an expert

func (s *ExpertsStore) GetExpertAvailability(ctx context.Context, expertID int64) (*[]ExpertAvailability, error) {
	query := `
		SELECT id, expert_id, day_of_week,  TO_CHAR(start_time, 'HH24:MI') AS start_time, TO_CHAR(end_time, 'HH24:MI') AS end_time, is_weekend, created_at
		FROM expert_availabilities
		WHERE expert_id = $1
		ORDER BY created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var availabilities []ExpertAvailability
	rows, err := s.db.QueryContext(ctx, query, expertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var availability ExpertAvailability
		err := rows.Scan(
			&availability.ID,
			&availability.ExpertID,
			&availability.Day,
			&availability.StartTime,
			&availability.EndTime,
			&availability.IsWeekend,
			&availability.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		availabilities = append(availabilities, availability)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &availabilities, nil
}

// AddExpertAvailability adds availability for an expert
func (s *ExpertsStore) AddExpertAvailability(ctx context.Context, availability *ExpertAvailability) error {
	query := `
		INSERT INTO expert_availabilities (expert_id, day_of_week, start_time, end_time, is_weekend)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if err := s.db.QueryRowContext(ctx, query, availability.ExpertID, availability.Day, availability.StartTime, availability.EndTime, availability.IsWeekend).Scan(&availability.ID); err != nil {
		return err
	}

	return nil
}

func (s *ExpertsStore) AddWeeklyAvailability(ctx context.Context, expertID int64, availabilities []ExpertAvailability) error {
	if len(availabilities) == 0 {
		return nil
	}

	var days []string
	var starts []string
	var ends []string
	var weekends []bool

	for _, a := range availabilities {
		// Optional: validate time format
		if _, err := time.Parse("15:04", a.StartTime); err != nil {
			return fmt.Errorf("invalid start_time format: %s", a.StartTime)
		}
		if _, err := time.Parse("15:04", a.EndTime); err != nil {
			return fmt.Errorf("invalid end_time format: %s", a.EndTime)
		}

		days = append(days, a.Day)
		starts = append(starts, a.StartTime)
		ends = append(ends, a.EndTime)
		weekends = append(weekends, a.IsWeekend)
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Remove old availability for this expert
	deleteQuery := `DELETE FROM expert_availabilities WHERE expert_id = $1`
	if _, err := tx.ExecContext(ctx, deleteQuery, expertID); err != nil {
		return fmt.Errorf("failed to delete old availability: %w", err)
	}

	// Step 2: Insert new availability records in one batch
	insertQuery := `
		INSERT INTO expert_availabilities (expert_id, day_of_week, start_time, end_time, is_weekend)
		SELECT 
			$1,
			unnest($2::text[]),
			unnest($3::time[]),
			unnest($4::time[]),
			unnest($5::bool[])
	`

	if _, err := tx.ExecContext(ctx, insertQuery,
		expertID,
		pq.Array(days),
		pq.Array(starts),
		pq.Array(ends),
		pq.Array(weekends),
	); err != nil {
		return fmt.Errorf("failed to insert weekly availability: %w", err)
	}

	// Step 3: Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
