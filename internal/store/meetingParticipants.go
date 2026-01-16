package store

import (
	"context"
	"database/sql"
	"time"
)

type MeetingParticipant struct {
	ID               int64     `json:"id"`
	MeetingID        int64     `json:"meeting_id"`
	ZoomMeetingID    int64     `json:"zoom_meeting_id"`
	UserID           int64     `json:"user_id"`
	Role             string    `js0n:"role"`
	JoinedAt         time.Time `json:"joined_at"`
	LeftAt           time.Time `json:"left_at"`
	Duration         int64     `json:"duration_seconds"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
	ParticipantID    string    `json:"participant_id"`
	ParticipantName  string    `json:"participant_name"`
	ParticipantEmail string    `json:"participant_email"`
	MeetingUUID      string    `json:"meeting_uuid"`
}

type MeetingParticipantStore struct {
	db *sql.DB
}

func (s *MeetingParticipantStore) Insert(ctx context.Context, meetPart *MeetingParticipant) error {
	query := `
		INSERT INTO
		zoom_meeting_participants (meeting_id, user_id, role) 
		VALUES ($1, $2, $3) 
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if err := s.db.QueryRowContext(
		ctx, query, meetPart.MeetingID, meetPart.UserID, meetPart.Role,
	).Scan(&meetPart.ID); err != nil {
		return err
	}

	return nil
}

func (s *MeetingParticipantStore) UpdateRole(ctx context.Context, mtPt *MeetingParticipant) error {
	query := `
		UPDATE zoom_meeting_participants
		SET role = $1
		WHERE id = $2
		RETURNING meeting_id
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	return s.db.QueryRowContext(ctx, query, mtPt.Role, mtPt.ID).Scan(&mtPt.MeetingID)
}

func (s *MeetingParticipantStore) DeleteByMeetingID(ctx context.Context, meetingID int64) error {
	query := `
		DELETE FROM zoom_meeting_participants
		WHERE meeting_id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, meetingID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *MeetingParticipantStore) InsertParticipantJoined(ctx context.Context, meetingParticipant *MeetingParticipant) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO zoom_meeting_participants (
			meeting_id, zoom_meeting_id, meeting_uuid, participant_id, participant_name, participant_email, joined_at, user_id, role 
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'participant')
	`, meetingParticipant.MeetingID, meetingParticipant.ZoomMeetingID, meetingParticipant.MeetingUUID, meetingParticipant.ParticipantID, meetingParticipant.ParticipantName, meetingParticipant.ParticipantEmail, meetingParticipant.JoinedAt, meetingParticipant.UserID)

	return err
}

func (s *MeetingParticipantStore) UpdateParticipantLeft(ctx context.Context, meetingParticipant *MeetingParticipant) error {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
		UPDATE zoom_meeting_participants
		SET left_at = $1,
		    duration_seconds = EXTRACT(EPOCH FROM $1 - joined_at),
		    updated_at = NOW()
		WHERE zoom_meeting_id = $2
		  AND participant_id = $3
		  AND left_at IS NULL
	`, meetingParticipant.LeftAt, meetingParticipant.ZoomMeetingID, meetingParticipant.ParticipantID)
	return err
}
