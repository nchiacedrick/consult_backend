package main

import (
	"errors"
	"net/http"

	"consult_app.cedrickewi/internal/store"
)

type BranchPayload struct {
	Name           string `json:"branch_name"`
	OrganisationID int64  `json:"organisation_id"`
	About          string `json:"about_branch"`
}

// create a branch, user must be owner of the organisation to create a branch
func (app *application) createBranchHandler(w http.ResponseWriter, r *http.Request) {
	var payload BranchPayload
	ctx := r.Context()
	user := app.contextGetUser(r)

	if err := app.readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validate a struct
	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	isOwner, err := app.store.Organisation.IsOwner(ctx, user.ID, payload.OrganisationID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !isOwner {
		app.notPermittedResponse(w, r)
		return
	}

	branch := &store.Branch{
		Name:           payload.Name,
		OrganisationID: payload.OrganisationID,
		About:          payload.About,
	}

	if err := app.store.Branch.Create(ctx, branch); err != nil {
		if err.Error() == `pq: duplicate key value violates unique constraint "branches_name_key"` {
			app.writeJSON(w, http.StatusBadRequest, envelope{"branch": "branch with name already exist"}, nil)
			return
		}
		writeJSON(w, http.StatusInternalServerError, err.Error())
		return
	} 
 
	if err = app.store.Permissions.AddForUser(ctx, user.ID, "branches:write"); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusCreated, envelope{"branch": "created successfully"}, nil); err != nil {
		app.internalServerError(w, r, err)
		return
	}
}

// get a branch by id
func (app *application) getBranchHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	branch, err := app.store.Branch.GetBranchByID(r.Context(), id)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"branch": branch}, nil); err != nil {
		app.internalServerError(w, r, err)
	}
}

// update a branch, user must be owner of the branch to update
func (app *application) updateBranchHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user := app.contextGetUser(r)

	ok, err := app.store.Branch.IsOwner(r.Context(), user.ID, id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !ok {
		app.notPermittedResponse(w, r)
		return
	}

	branch, err := app.store.Branch.GetBranchByID(r.Context(), id)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var payload BranchPayload
	if err := app.readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err = app.store.Branch.Update(r.Context(), branch); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"branch": "Branch updated successfully"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// delete a branch, user must be owner of the branch to delete
func (app *application) deleteBranchHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user := app.contextGetUser(r)

	ok, err := app.store.Branch.IsOwner(r.Context(), user.ID, id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !ok {
		app.notPermittedResponse(w, r)
		return
	}

	if err = app.store.Branch.Delete(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"branch": "Branch deleted successfully"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// get all branches for an expert
func (app *application) getAllExpertBranches(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	branches, err := app.store.Branch.GetAllExpertBranches(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"branches": branches}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// get all experts in a branch
func (app *application) getAllExpertsInBranch(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	experts, err := app.store.Branch.GetAllExpertsForBranch(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"experts": experts}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// remove expert from a branch, user most be owner of the branch to remove an expert
func (app *application) removeExpertFromBranch(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user := app.contextGetUser(r)

	var input struct {
		ExpertID int64 `json:"expert_id"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	expert, err := app.store.Expert.GetUserByExpertID(r.Context(), input.ExpertID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	ok, err := app.store.Branch.IsOwner(r.Context(), user.ID, id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !ok {
		app.notPermittedResponse(w, r)
		return
	}

	if err = app.store.Branch.RemoveExpertFromBranch(r.Context(), id, expert.ID); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"message": "Expert removed from branch successfully"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// add expert to a branch, user most be owner of the branch to add an expert
func (app *application) addExpertToBranch(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}   

	user := app.contextGetUser(r)

	var input struct {
		ExpertID int64 `json:"expert_id"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	expert, err := app.store.Expert.GetUserByExpertID(r.Context(), input.ExpertID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	ok, err := app.store.Branch.IsOwner(r.Context(), user.ID, id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !ok {
		app.notPermittedResponse(w, r)
		return
	}

	if err = app.store.Branch.AddExpertToBranch(r.Context(), id, expert.ID); err != nil {
		app.internalServerError(w,r,err)
		return 
	}

	
}