package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// routes sets up the router and the routes for the API.
func (app *application) routes() http.Handler {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)
	router.HandlerFunc(http.MethodGet, "/v1/results", app.getResultHandler)

	return app.recoverPanic(router)
}
