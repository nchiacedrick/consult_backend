package auth

import (
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/google"
)

const (
	key = ""

	IsProd = false
)

func NewAuth(GOOGLE_CLIENT_ID_ENV, GOOGLE_CLIENT_SECRET_ENV string) {

	gothstore := sessions.NewCookieStore([]byte(key))
	gothstore.MaxAge(1)

	gothstore.Options.Path = "/"
	gothstore.Options.HttpOnly = true
	gothstore.Options.Secure = IsProd

	gothic.Store = gothstore

	goth.UseProviders(
		google.New(
			GOOGLE_CLIENT_ID_ENV,
			GOOGLE_CLIENT_SECRET_ENV,
			"http://localhost:8080/v1/auth/callback?provider=google", // Match this with your callback route
			"email",
			"profile",
		),
		facebook.New("", "", ""),
	)
}
