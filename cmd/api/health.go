package main

import (
	"net/http"
)

func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {

	data := map[string]string{
		"status":  "ok",
		"env":     app.config.env,
		"version": version,
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"data": data}, nil); err != nil {
		//error
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
}
