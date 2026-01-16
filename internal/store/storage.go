package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrNotFound              = errors.New("resource not found")
	ErrConflict              = errors.New("resource already exist")
	QueryTimeoutDuration     = time.Second * 5
	ErrDuplicateEmail        = errors.New("duplicate email")
	ErrDuplicateUsername     = errors.New("duplicate username")
	ErrDuplicateOrganisation = errors.New("duplicate organisation")
	ErrRecordNotFound        = errors.New("record not found")
	ErrEditConflict          = errors.New("error editing user")
	ErrDuplicateExpert       = errors.New("error duplicat expert")
	ErrDuplicatExpertBranch  = errors.New("error duplicate expert branch, expert already belongs to branch")
	ErrExpertNotFound        = errors.New("expert not found")
	ErrOverlappingTimeSlot   = errors.New("overlapping time slot")
	ErrEndTimeBeforeStart    = errors.New("end time must be after start time")
	ErrNoExpertOverlap       = errors.New("expert cannot double book the same time slot")
	ErrNoUserOverlap         = errors.New("user cannot double book the same time slot")
)

type Storage struct {
	User interface {
		Create(context.Context, *User) error
		GetByEmail(context.Context, string) (*User, error)
		Update(context.Context, *User) error
		GetForToken(context.Context, string, string) (*User, error)
		GetByID(context.Context, int64) (*User, error)
		Delete(context.Context, int64) error
		UpdateUserImage(context.Context, int64, string) error
	}

	Organisation interface {
		Create(context.Context, *Organisation, *Branch) error  
		GetAllOrgs(context.Context) (*[]Organisation, error)
		GetOrganisationByID(context.Context, int64) (*GetOrganisationByIDResult, error)
		Delete(context.Context, int64) error
		Update(context.Context, *Organisation) error
		GetOwnerID(context.Context, int64) (int64, error)
		IsVerified(context.Context, int64) (bool, error)
		IsOwner(context.Context, int64, int64) (bool, error)
		GetAllForUser(context.Context, int64) (*[]Organisation, error)
	}

	Branch interface {
		Create(context.Context, *Branch) error
		GetBranchByID(context.Context, int64) (*Branch, error)
		GetAllBranches(context.Context) (*[]Branch, error)
		Delete(context.Context, int64) error
		Update(context.Context, *Branch) error
		GetBranchByOrgID(context.Context, int64) (*[]Branch, error)
		GetByID(context.Context, int64) (*Branch, error)
		GetAllOrganisationBranches(context.Context, int64) (*[]Branch, error)
		GetBranchByExpertID(context.Context, int64) (*Branch, error)
		GetAllExpertBranches(context.Context, int64) (*[]Branch, error)
		GetAllExpertsForBranch(context.Context, int64) (*[]Expert, error)
		IsOwner(context.Context, int64, int64) (bool, error)
		RemoveExpertFromBranch(context.Context, int64, int64) error
		AddExpertToBranch(ctx context.Context, branchID, expertID int64) error
	}

	Expert interface {
		Insert(context.Context, *Expert) error
		UpdateExpert(context.Context, *Expert) error
		InsertToBranch(context.Context, *ExpertBranch) error
		IsExpert(context.Context, int64) (bool, error)
		GetExpertByUserID(context.Context, int64) (*Expert, error)
		GetExpertByID(context.Context, int64) (*Expert, error)
		GetUserByExpertID(context.Context, int64) (*User, error)
		GetAllExperts(context.Context, int64) (*[]Expert, error)
		GetAllExpertConsultations(context.Context, int64) (*[]Consultation, error)
		GetAnExpertConsultationByBookingID(ctx context.Context, bookingID int64, expertID int64) (*Consultation, error)
		GetExpertAvailability(context.Context, int64) (*[]ExpertAvailability, error)
		AddExpertAvailability(ctx context.Context, availability *ExpertAvailability) error 
		AddWeeklyAvailability(ctx context.Context, expertID int64, availabilities []ExpertAvailability) error
	}

	Token interface {
		New(context.Context, int64, time.Duration, string) (*Token, error)
		Insert(context.Context, *Token) error
		DeleteAllForUser(context.Context, string, int64) error
	}

	ZoomMeeting interface {
		Insert(context.Context, *ZoomMeeting, int64) (int64, error)
		Delete(context.Context, int64) error
		Update(context.Context, *ZoomMeeting) error
		GetByID(context.Context, int64) (*ZoomMeeting, error)
		GetByZoomID(context.Context, int64) (*ZoomMeeting, error)
	}

	MeetingParticipant interface {
		Insert(context.Context, *MeetingParticipant) error
		DeleteByMeetingID(context.Context, int64) error
		InsertParticipantJoined(context.Context, *MeetingParticipant) error
		UpdateParticipantLeft(context.Context, *MeetingParticipant) error
	}

	Booking interface {
		Insert(context.Context, *Booking) error
		Update(ctx context.Context, booking *Booking) error
		Delete(context.Context, int64) error
		GetByID(context.Context, int64) (*Booking, error)
		GetAllUserBookings(context.Context, int64) (*[]CustomBooking, error)
		GetBookingDetails(context.Context, int64) (*CustomBooking, error)
		IsUserMeeting(context.Context, int64, int64) (bool, error)
		UpdatePaymentStatus(context.Context, int64, string, int64) error
		UpdateBookingStatus(context.Context, int64, string) error
		IsExpertMeeting(context.Context, int64, int64) (bool, error)
		UpdateInitTransactionID(ctx context.Context, bookingID, initTransx int64) error
		UpdatePayunitPaymentID(ctx context.Context, bookingID, initTransx int64) error
		UpdateTransactionID(ctx context.Context, bookingID int64, transactionID string) error 
		GetByTransactionID(ctx context.Context, transactionID string) (*Booking, error)
		UpdateBookingReminders(ctx context.Context, bookingID int64, userReminder int, expertReminder int) error
	}

	PayUnit interface {
		InsertInitializedTransaction(context.Context, *PayUnitResponse) (int64, error)
		InsertPayunitPayment(context.Context, *PaymentResponse) (int64, error)
		InsertPaymentStatus(context.Context, *PaymentStatusResponse) (int64, error)
		UpdatePaymentStatus(context.Context, *PaymentStatusResponse) error
		CheckPaymentStatusExists(context.Context, string) (bool, error) 
		GetPayunitInitializationByTransactionID(ctx context.Context, bookingID int64) (*PayUnitResponse, error)
	}

	Permissions interface {
		GetAllForUser(int64) (Permissions, error)
		AddForUser(context.Context, int64, ...string) error
	}

	Roles interface {
		GetByName(context.Context, string) (*Role, error)
	}
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Organisation:       &OrganisationStore{db: db},
		Branch:             &BranchStore{db: db},
		User:               &UserStore{db: db},
		Token:              &TokenStore{db: db},
		Booking:            &BookingStore{db: db},
		Expert:             &ExpertsStore{db: db},
		ZoomMeeting:        &ZoomMeetingStore{db: db},
		MeetingParticipant: &MeetingParticipantStore{db: db},
		Roles:              &RoleStore{db: db},
		Permissions:        &PermissionStore{db: db},
		PayUnit:            &PayunitStore{db: db},
	}
}

// Working with database Transactions
func withTx(db *sql.DB, ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
