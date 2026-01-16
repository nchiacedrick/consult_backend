package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

const (
	key    = "veryrandomkeyeinsodoncezy"
	MaxAge = 86400 * 30 // store user info for 30days
	IsProd = false
)

// google authentication
func (app *application) getAuthCallbackFunction(w http.ResponseWriter, r *http.Request) {
	// Attempt to complete user authentication
	user, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		fmt.Println("error completing user authentication", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	fmt.Println("user :::", user)
	// Check if the user already exists in the database, by email

}

func (app *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	// Default to Google as the provider
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		app.badRequestResponse(w, r, fmt.Errorf("an oauth provider must be provided (google, apple,facebook)"))
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), "provider", provider))

	store := sessions.NewCookieStore([]byte(key))
	store.MaxAge(MaxAge)
	gothic.Store = store

	// debug
	fmt.Println("Store:", gothic.Store)
	// debug
	fmt.Println("Providers:", goth.GetProviders())

	// Begin the OAuth authentication process
	gothic.BeginAuthHandler(w, r)

	// debug
	fmt.Printf("Authentification avec %s", provider)
}
