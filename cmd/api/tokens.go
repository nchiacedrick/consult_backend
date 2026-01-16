package main

import (
	"errors"
	"net/http"
	"time"

	"consult_app.cedrickewi/internal/store"
)

// logging a user into the system/database
func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Validate the email and password provided by the client.
	if err := Validate.Struct(input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()

	user, err := app.store.User.GetByEmail(ctx, input.Email)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the provided password matches the actual password for the user.
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// If the passwords don't match, then we call the app.invalidCredentialsResponse()
	// helper again and return.
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	// Otherwise, if the password is correct, we generate a new token with a 24-hour
	// expiry time and the scope 'authentication'.
	token, err := app.store.Token.New(ctx, user.ID, 24*time.Hour, store.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Set the user in the request context for use in subsequent handlers
	// This allows middleware and other handlers to access the authenticated user
	app.contextSetUser(r, user)
	isexpert, err := app.store.Expert.IsExpert(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	user.IsExpert = isexpert

	if isexpert {
		expert, err := app.store.Expert.GetExpertByUserID(r.Context(), user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		app.contextSetExpert(r, expert)
	}

	// Encode the token to JSON and send it in the response along with a 201 Created
	// status code.
	err = app.writeJSON(w, http.StatusCreated, envelope{"token": token, "user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
