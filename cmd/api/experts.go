package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"consult_app.cedrickewi/internal/data"
	"consult_app.cedrickewi/internal/store"
	"consult_app.cedrickewi/internal/validator"
)

func (app *application) createExpertHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Expertise string  `json:"expertise"`
		Bio       string  `json:"bio"`
		FeesPerHr float64 `json:"fees_per_hr"`
		Language  string  `json:"language"`
	}

	user := app.contextGetUser(r)

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err = Validate.Struct(input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	expert := store.Expert{
		UserID:    user.ID,
		Expertise: input.Expertise,
		Bio:       input.Bio,
		FeesPerHr: input.FeesPerHr,
		Language:  input.Language,
	}

	ctx := r.Context()

	if err := app.store.Expert.Insert(ctx, &expert); err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicateExpert):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err = app.store.Permissions.AddForUser(ctx, user.ID, "experts:write", "organisations:write"); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.contextSetExpert(r, &expert)

	if err = app.writeJSON(w, http.StatusOK, envelope{"expert": expert}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) getAllExpertsHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Expertise []string
		Language  []string
		FeesPerHr int
		data.Filters
	}
	v := validator.New()

	user := app.contextGetUser(r)

	qs := r.URL.Query()

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readStrings(qs, "sort", "id")
	input.Expertise = app.readCSV(qs, "expertise", []string{})
	input.Language = app.readCSV(qs, "language", []string{})
	input.FeesPerHr = app.readInt(qs, "fees_per_hr", 0, v)
	input.Filters.SortSafe = []string{"id", "expertise", "language", "fees_per_hr"}

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	experts, err := app.store.Expert.GetAllExperts(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"experts": experts}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// add expert to branch
func (app *application) expertToBranchHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ExpertID int64 `json:"expert_id"`
		BranchID int64 `json:"branch_id"`
	}

	ctx := r.Context()

	if err := app.readJSON(w, r, &input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	expertBranch := store.ExpertBranch{
		ExpertID: input.ExpertID,
		BranchID: input.BranchID,
	}

	err := app.store.Expert.InsertToBranch(ctx, &expertBranch)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicatExpertBranch):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"expert_branch": expertBranch}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// show expert by expertID
func (app *application) showExpertsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	expert, err := app.store.Expert.GetExpertByID(r.Context(), id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"expert": expert}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) updateExpertsHander(w http.ResponseWriter, r *http.Request) {

}

func (app *application) getExpertAvailabilityHandler(w http.ResponseWriter, r *http.Request) {
	expertID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	expert, err := app.store.Expert.GetExpertByID(r.Context(), expertID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	availability, err := app.store.Expert.GetExpertAvailability(r.Context(), expert.ID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"availability": availability}, nil); err != nil {
		app.internalServerError(w, r, err)
	}
}

// get expert information by userID
func (app *application) getExpertByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	user := app.contextGetUser(r)

	if user.ID != id {
		app.notPermittedResponse(w, r)
		return
	}

	expert, err := app.store.Expert.GetExpertByUserID(r.Context(), id)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"expert": expert}, nil); err != nil {
		app.internalServerError(w, r, err)
	}
}

// get all consultations for an expert
func (app *application) getAllConsultationsForExpertHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	user := app.contextGetUser(r)

	expert, err := app.store.Expert.GetExpertByID(r.Context(), id)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if expert.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	consultations, err := app.store.Expert.GetAllExpertConsultations(r.Context(), expert.ID)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"consultations": consultations}, nil); err != nil {
		app.internalServerError(w, r, err)
	}
}

type expertAvailabilityInput struct {
	DayOfWeek string `json:"day"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// create expert availabilityHanlder
func (app *application) createExpertAvailabilityHandler(w http.ResponseWriter, r *http.Request) {
	var inputs []expertAvailabilityInput

	// Parse JSON array input
	err := app.readJSON(w, r, &inputs)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if len(inputs) == 0 {
		app.badRequestResponse(w, r, errors.New("no availability data provided"))
		return
	}

	// Validate each availability entry
	for i, input := range inputs {
		if err := Validate.Struct(input); err != nil {
			app.badRequestResponse(w, r, fmt.Errorf("invalid input at index %d: %v", i, err))
			return
		}
	}

	// Get currently logged-in user
	user := app.contextGetUser(r)

	// Retrieve expert by user ID
	expert, err := app.store.Expert.GetExpertByUserID(r.Context(), user.ID)
	if err != nil {
		app.internalServerError(w, r, fmt.Errorf("failed to get expert: %w", err))
		return
	}

	// Build slice of availabilities
	var availabilities []store.ExpertAvailability
	for _, in := range inputs {
		a := store.ExpertAvailability{
			ExpertID:  expert.ID,
			Day:       in.DayOfWeek,
			StartTime: in.StartTime,
			EndTime:   in.EndTime,
			IsWeekend: strings.ToLower(in.DayOfWeek) == "saturday" || strings.ToLower(in.DayOfWeek) == "sunday",
		}
		availabilities = append(availabilities, a)
	}

	// Insert all in one query
	ctx := r.Context()
	if err := app.store.Expert.AddWeeklyAvailability(ctx, expert.ID, availabilities); err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to add weekly availability: %w", err))
		return
	}

	// Return the newly added availabilities
	if err = app.writeJSON(w, http.StatusCreated, envelope{"availabilities": availabilities}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
