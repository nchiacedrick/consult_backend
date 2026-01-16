package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"consult_app.cedrickewi/internal/aws"
	"consult_app.cedrickewi/internal/mailer"
	"consult_app.cedrickewi/internal/store"
	"consult_app.cedrickewi/internal/validator"
	"github.com/google/uuid"
)

type UserPayload struct {
	Name     string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
}

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var payload UserPayload

	if err := app.readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validate a struct
	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()

	user := &store.User{
		Name:        payload.Name,
		Email:       payload.Email,
		Phone:       payload.Phone,
		IsActivated: false,
	}
	// Use the Password.Set() method to generate and store the hashed and plaintext
	// passwords.
	err := user.Password.Set(payload.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// insert a user to the database users table
	if err := app.store.User.Create(ctx, user); err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicateEmail):
			app.badRequestResponse(w, r, err)
			return
		case errors.Is(err, store.ErrDuplicateUsername):
			app.badRequestResponse(w, r, err)
			return
		case strings.Contains(err.Error(), "users_phone_key"):
			app.badRequestResponse(w, r, fmt.Errorf("phone number already in use"))
			return
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// After the user record has been created in the database, generate a new activation
	// token for the user.
	token, err := app.store.Token.New(ctx, user.ID, 3*24*time.Hour, store.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Launch a goroutine which runs an anonymous function that sends the welcome email.
	app.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}
		// if err := app.mailer.Send(user.Email, "user_welcome.tmpl", data); err != nil {
		// 	app.logger.Errorln(err)
		// }
		mailerErr := mailer.NewResend(user.Email, "user_welcome.tmpl", data)
		if mailerErr != nil {
			app.logger.Errorln(mailerErr)
		}
	})
	app.contextSetUser(r, user)
	// send a statusAccepted to indicates that the request has been accepted for processing
	if err := app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// activate a user using the activation code sent to thier email
func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlainText string `json:"token"`
	}

	if err := app.readJSON(w, r, &input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	if store.ValidateTokenPlaintext(v, input.TokenPlainText); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	ctx := r.Context()

	user, err := app.store.User.GetForToken(ctx, store.ScopeActivation, input.TokenPlainText)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Update the user's activation status.
	user.IsActivated = true

	// Save the updated user record in our database, checking for any edit conflicts in
	// the same way that we did for our movie records.
	err = app.store.User.Update(ctx, user)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// If everything went successfully, then we delete all activation tokens for the
	// user.
	err = app.store.Token.DeleteAllForUser(ctx, store.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Add the timeslots:read permission for the user.
	err = app.store.Permissions.AddForUser(ctx, user.ID, "bookings:write", "timeslots:read", "experts:read", "organisations:read", "branches:read", "users:write", "users:read", "users:write", "bookings:read")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Send the updated user details to the client in a JSON response.
	if err := app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// get user by id
func (app *application) getUserAccountHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user, err := app.store.User.GetByID(r.Context(), id)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	isexpert, err := app.store.Expert.IsExpert(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	user.IsExpert = isexpert

	app.contextSetUser(r, user)

	if err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// update user
func (app *application) updateUserHandler(w http.ResponseWriter, r *http.Request) {

	ctxuser := app.contextGetUser(r)

	user, err := app.store.User.GetByID(r.Context(), ctxuser.ID)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// ✅ Parse multipart form (10MB limit)
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("invalid form data"))
		return
	}

	// ✅ Get text fields
	name := r.FormValue("username")
	email := r.FormValue("email")
	phone := r.FormValue("phone")

	if name != "" {
		user.Name = name
	}
	if email != "" {
		user.Email = email
	}
	if phone != "" {
		user.Phone = phone
	}

	// ✅ Handle optional image
	file, header, err := r.FormFile("image")

	if err == nil {
		defer file.Close()

		// Generate S3 key
		ext := filepath.Ext(header.Filename)
		key := fmt.Sprintf("profiles/%d%s", time.Now().UnixNano(), ext)

		contentType := header.Header.Get("Content-Type")

		// Upload to S3
		imageURL, err := aws.UploadToS3(file, key, contentType)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		// ✅ Save image URL to user
		user.ImageURL = imageURL
	}

	// ✅ Update timestamp
	user.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// ✅ Save to DB
	if err := app.store.User.Update(r.Context(), user); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.contextSetUser(r, user)

	// ✅ Response
	if err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	if err := app.store.User.Delete(r.Context(), id); err != nil {
		app.serverErrorResponse(w, r, err)
	}

	if err = app.writeJSON(w, http.StatusOK, envelope{"message": "user deleted"}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

const UploadDir = "/Uploads"

// Use a map to define allowed MIME types for better performance
// and to avoid using a switch statement

func (app *application) updateUserProfileHanlder(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("image")
	if err != nil {
		app.badRequestResponse(w, r, err)
	}

	defer file.Close()

	if header.Size == 0 {
		app.badRequestResponse(w, r, errors.New("cannot upload empty file"))
		return
	}

	//dectect file type
	filetype, err := detectMIME(file)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	var allowedMIMEs = map[string]struct{}{
		"image/jpeg":      {},
		"image/png":       {},
		"application/pdf": {},
		"text/plain":      {},
	}

	if _, ok := allowedMIMEs[filetype]; !ok {
		app.badRequestResponse(w, r, errors.New("failed to detect file type"))
		return
	}

	// Reset file pointer back to beginning
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		app.internalServerError(w, r, err)
		return
	}

	filename := filepath.Base(header.Filename)        // basic sanitization
	filename = filepath.Clean(filename)               // clean up the filename
	filename = strings.ReplaceAll(filename, " ", "_") // replace spaces with underscores
	// NOOB Mistake 4: Not using a unique file name
	// Save with a UUID filename to avoid name collisions
	// generate unique filename
	newFilename := fmt.Sprintf("%s-%s", uuid.NewString(), filename)

	// outPath := filepath.Join("uploads", newFilename)

	// Make sure the uploads directory exists
	// err = os.MkdirAll("uploads", os.ModePerm)
	// if err != nil {
	// 	http.Error(w, "Unable to create upload directory: "+err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	// // Create destination file
	// dst, err := os.Create(outPath)
	// if err != nil {
	// 	http.Error(w, "Unable to create file: "+err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// defer dst.Close()

	// //move file to destination
	// _, err = io.Copy(dst, file)
	// if err != nil {
	// 	http.Error(w, "Unable to save file: "+err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	location, err := aws.UploadToS3(file, newFilename, filetype)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	app.store.User.UpdateUserImage(r.Context(), app.contextGetUser(r).ID, location)

	fmt.Fprintf(w, "File uploaded successfully: %s\n", location)
}

// getPayunitPayment providers
func (app *application) getPayunitPaymentProvidersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bkID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
	}

	user := app.contextGetUser(r)

	booking, err := app.store.Booking.GetByID(ctx, bkID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if booking.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	providers, err := app.payunit.GetProviders(ctx, booking.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case strings.Contains(err.Error(), "no PayUnit initialization found"):
			app.notFoundResponse(w, r)

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"providers": providers}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
