package store

import (
	"context"
	"database/sql"
	"time"
)

type ZoomMeeting struct {
	ID        int64     `json:"meeting_id"`
	JoinURL   string    `json:"join_url"`
	StartURL  string    `json:"start_url"`
	MeetingID int64     `json:"id"` // Zoom's Meeting ID
	Agenda    *string   `json:"agenda"`
	CreatedBy int64     `json:"created_by"`
	Topic     *string   `json:"topic"`
	Type      int       `json:"type"`
	HostID    string    `json:"host_id"`
	HostEmail string    `json:"host_email"`
	StartTime time.Time `json:"start_time"`
	Duration  int64     `json:"duration"`
	Password  string    `json:"password"`
	TimeZone  string    `json:"timezone"`
	Status    string    `json:"status"`
}

type ZoomMeetingStore struct {
	db *sql.DB
}

func (s *ZoomMeetingStore) Insert(ctx context.Context, meeting *ZoomMeeting, userID int64) (int64, error) {
	query := `  
		INSERT INTO zoom_meetings (
			zoom_meeting_id, meeting_url, start_url,
			created_by, zoom_host_id, zoom_host_email,
			topic, start_time, 
			duration, agenda, zoom_status, password
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var id int64
	err := s.db.QueryRowContext(ctx, query,
		meeting.MeetingID,
		meeting.JoinURL,
		meeting.StartURL,
		userID, // created_by (you already have userID)
		meeting.HostID,
		meeting.HostEmail,
		meeting.Topic,
		meeting.StartTime,
		meeting.Duration,
		meeting.Agenda,
		meeting.Status,
		meeting.Password,
	).Scan(&id)

	if err != nil {
		return 0, err
	}

	// Assign it to the struct too (optional)
	meeting.ID = id

	return id, nil
}

// GetByID retrieves a Zoom meeting by its ID
func (s *ZoomMeetingStore) GetByID(ctx context.Context, id int64) (*ZoomMeeting, error) {
	query := `
		SELECT id, zoom_meeting_id, meeting_url, start_url,
			created_by, zoom_host_id, zoom_host_email,
			topic, start_time, duration, agenda, zoom_status, password
		FROM zoom_meetings
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	meeting := &ZoomMeeting{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&meeting.ID, &meeting.MeetingID, &meeting.JoinURL, &meeting.StartURL,
		&meeting.CreatedBy, &meeting.HostID, &meeting.HostEmail,
		&meeting.Topic, &meeting.StartTime, &meeting.Duration,
		&meeting.Agenda, &meeting.Status, &meeting.Password,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return meeting, nil
}

// Delete removes a Zoom meeting from the database
func (s *ZoomMeetingStore) Delete(ctx context.Context, id int64) error {
	query := `
		DELETE FROM zoom_meetings
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

// Update updates an existing Zoom meeting in the database
func (s *ZoomMeetingStore) Update(ctx context.Context, meeting *ZoomMeeting) error {
	query := `
		UPDATE zoom_meetings
		SET topic = $1, agenda = $2, duration = $3, zoom_status = $4
		WHERE id = $5
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query,
		meeting.Topic, meeting.Agenda, meeting.Duration, meeting.Status,
		meeting.ID,
	).Scan(&meeting.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrRecordNotFound
		}
		return err
	}

	return nil
}

// GetAll for specific user
func (s *ZoomMeetingStore) GetAll(ctx context.Context, userID int64) ([]*ZoomMeeting, error) {
	query := `
		SELECT id, zoom_meeting_id, meeting_url, start_url,
			created_by, zoom_host_id, zoom_host_email,
			topic, start_time, duration, agenda, zoom_status, password
		FROM zoom_meetings
		WHERE created_by = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meetings := []*ZoomMeeting{}
	for rows.Next() {
		var meeting ZoomMeeting
		if err := rows.Scan(
			&meeting.ID, &meeting.MeetingID, &meeting.JoinURL, &meeting.StartURL,
			&meeting.CreatedBy, &meeting.HostID, &meeting.HostEmail,
			&meeting.Topic, &meeting.StartTime, &meeting.Duration,
			&meeting.Agenda, &meeting.Status, &meeting.Password,
		); err != nil {
			return nil, err
		}
		meetings = append(meetings, &meeting)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return meetings, nil
}

// GetByZoomID retrieves a Zoom meeting by its Zoom meeting ID
func (s *ZoomMeetingStore) GetByZoomID(ctx context.Context, zoomID int64) (*ZoomMeeting, error) {
	query := `
		SELECT id, zoom_meeting_id, meeting_url, start_url,
			created_by, zoom_host_id, zoom_host_email,
			topic, start_time, duration, agenda, zoom_status, password
		FROM zoom_meetings
		WHERE zoom_meeting_id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	meeting := &ZoomMeeting{}
	err := s.db.QueryRowContext(ctx, query, zoomID).Scan(
		&meeting.ID, &meeting.MeetingID, &meeting.JoinURL, &meeting.StartURL,
		&meeting.CreatedBy, &meeting.HostID, &meeting.HostEmail,
		&meeting.Topic, &meeting.StartTime, &meeting.Duration,
		&meeting.Agenda, &meeting.Status, &meeting.Password,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return meeting, nil
}
