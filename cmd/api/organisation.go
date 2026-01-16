package main

import (
	"errors"
	"fmt"
	"net/http"

	"consult_app.cedrickewi/internal/store"
)

type CreateOrgPayload struct {
	OrgName        string `json:"org_name"`
	AboutOrg       string `json:"about_org"`
	Purpose        string `json:"purpose"`
	OrgEmail       string `json:"org_email"`
	OrgPhone       string `json:"org_phone"`
	OrgWebsite     string `json:"org_website"`
	OrgAddress     string `json:"org_address"`
	Location       string `json:"location"`
	Founded        string `json:"founded"`
	Category       string `json:"category"`
	Logo           string `json:"logo"`
	BranchName     string `json:"branch_name"`
	AboutBranch    string `json:"about_branch"`
	BranchPhone    string `json:"branch_phone"`
	BranchLocation string `json:"branch_location"`
}

// create organisation
func (app *application) createOrganisationHandler(w http.ResponseWriter, r *http.Request) {
	var payload CreateOrgPayload

	if err := app.readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validate a struct
	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	isExpert, err := app.store.Expert.IsExpert(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !isExpert {
		fmt.Println("user is not an expert from isExpert check")
		app.notPermittedResponse(w, r)
		return
	}

	org := &store.Organisation{
		Name:     payload.OrgName,
		AboutOrg: payload.AboutOrg,
		OwnerID:  user.ID,
		Purpose:  payload.Purpose,
		OrgEmail: payload.OrgEmail,
		Phone:    payload.OrgPhone,
		Website:  payload.OrgWebsite,
		Address:  payload.OrgAddress,
		Location: payload.Location,
		Founded:  payload.Founded,
		Category: payload.Category,
		Logo:     payload.Logo,
	}

	branch := &store.Branch{
		Name:           payload.BranchName,
		About:          payload.AboutBranch,
		Phone:          payload.BranchPhone,
		BranchLocation: payload.BranchLocation,
	}

	ctx := r.Context()


	if err := app.store.Organisation.Create(ctx, org, branch); err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicateOrganisation):
			app.badRequestResponse(w, r, err)
			return
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err := app.writeJSON(w, http.StatusCreated, envelope{"organisation": "created successfully"}, nil); err != nil {
		app.internalServerError(w, r, err)
		return
	}
}

// get all organisations
func (app *application) getAllOrganisations(w http.ResponseWriter, r *http.Request) {

	organs, err := app.store.Organisation.GetAllOrgs(r.Context())
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"organisations": organs}, nil); err != nil {
		app.internalServerError(w, r, err)
	}
}

// get organisation by id
func (app *application) getAnOrganisationDetails(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	ctx := r.Context()
	org, err := app.store.Organisation.GetOrganisationByID(ctx, id)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"organisation": org}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

