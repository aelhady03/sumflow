package main

import (
	"net/http"
)

// healthcheckHandler returns a simple status message to indicate that the API is running.
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {

	env := envelope{
		"status": "available",
		"system_info": envelope{
			"version":     version,
			"environment": app.config.env,
		},
	}

	err := app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getResultHandler returns a simple sum result.
func (app *application) getResultHandler(w http.ResponseWriter, r *http.Request) {

	total, err := app.service.Get()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"result": total}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
