package main

import (
	"context"
	"net/http"

	"consult_app.cedrickewi/internal/store"
)

type contextKey string

const userContextKey = contextKey("user")

func (app *application) contextSetUser(r *http.Request, user *store.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user) 
	return r.WithContext(ctx)
}

func (app *application) contextGetUser(r *http.Request) *store.User {

	user, ok := r.Context().Value(userContextKey).(*store.User)
	if !ok {
		panic("missing user value in request context")
	}

	return user
}

const expertContextKey = contextKey("expert")

func (app *application) contextSetExpert(r *http.Request, expert *store.Expert) *http.Request {
	ctx := context.WithValue(r.Context(), expertContextKey, expert)
	return r.WithContext(ctx)
}

func (app *application) contextGetExpert(r *http.Request) (*store.Expert, bool) {
	exp, ok := r.Context().Value(expertContextKey).(*store.Expert)
	return exp, ok
}

// contextEnsureExpert ensures the request context contains expert info for the current user.
// The loader function is responsible for deciding whether the given user is an expert and
// returning the corresponding *store.Expert (or nil if not an expert). If the loader
// returns an expert, it is stored in the request context and the new request is returned.
func (app *application) contextEnsureExpert(r *http.Request, loader func(*store.User) (*store.Expert, error)) (*store.Expert, *http.Request, error) {
	// already present in context?
	if exp, ok := app.contextGetExpert(r); ok {
		return exp, r, nil
	}

	// get user from context (will panic if missing as in existing helper)
	user := app.contextGetUser(r)
	if user == nil {
		return nil, r, nil
	}

	// use provided loader to decide and fetch expert info
	exp, err := loader(user)
	if err != nil || exp == nil {
		// not an expert or loader failed
		return nil, r, err
	}

	// store expert in context and return updated request
	r2 := app.contextSetExpert(r, exp)
	return exp, r2, nil
}
