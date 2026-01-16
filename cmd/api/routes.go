package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func (app *application) routes() http.Handler {
	if err := godotenv.Load(); err != nil {
		app.logger.Info("No .env file found")
	}

	r := chi.NewRouter()

	// Global middlewares applied to all routes
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(app.secureHeaders)
	r.Use(app.rateLimit)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(app.recoverPanic)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://localhost:4000",
			"https://consult-out.com",
			"https://www.consult-out.com",
			"https://consult-out-frontend.vercel.app",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public Routes (No Authentication)
	r.Group(func(r chi.Router) {
		r.Post("/v1/webhook", app.zoomWebhookHandler)
		r.Post("/api/v1/payunit/notify", app.payunitWebHookHandler)
		r.Get("/v1/health", app.healthCheckHandler)
		r.Route("/v1/auth", func(r chi.Router) {
			r.Post("/register", app.registerUserHandler)
			r.Post("/activate", app.activateUserHandler)
			r.Post("/login", app.createAuthenticationTokenHandler)
		})
	})

	// Authenticated Routes
	r.Route("/v1", func(r chi.Router) {

		r.Use(app.authenticate)

		// OAuth Routes
		r.Route("/0auth", func(r chi.Router) {
			r.Get("/login", app.loginHandler)
			r.Get("/callback", app.getAuthCallbackFunction)
		})

		// Users Routes
		r.Route("/users", func(r chi.Router) {
			r.Get("/me/{id}", app.requireAuthenticatedUser(app.getUserAccountHandler))
			r.Put("/me/{id}", app.requiredPermission("users:write", app.updateUserHandler))
		})

		// Organisation Routes
		r.Route("/organisations", func(r chi.Router) {
			r.Get("/", app.requireAuthenticatedUser(app.getAllOrganisations))
			r.Get("/{id}", app.requireAuthenticatedUser(app.getAnOrganisationDetails))
			r.Post("/", app.requiredPermission("organisations:write", app.createOrganisationHandler))
		})  

		// Experts Routes
		r.Route("/experts", func(r chi.Router) {
			r.Get("/", app.requireAuthenticatedUser(app.getAllExpertsHandler))
			r.Get("/{id}/availability", app.requireAuthenticatedUser(app.getExpertAvailabilityHandler))
			r.Get("/me/{id}", app.requireAuthenticatedUser(app.getExpertByUserIDHandler))
			r.Post("/", app.requiredPermission("experts:read", app.createExpertHandler))
			r.Post("/add", app.requiredPermission("experts:write", app.expertToBranchHandler))
			r.Get("/{id}", app.requiredPermission("experts:read", app.showExpertsHandler))
			r.Get("/{id}/consultations", app.requiredPermission("experts:read", app.getAllConsultationsForExpertHandler))
			r.Put("/{id}", app.requiredPermission("experts:write", app.updateExpertsHander))
			r.Delete("/{id}", app.requiredPermission("experts:write", app.removeExpertFromBranch))
			r.Post("/availability/create", app.requiredPermission("experts:write", app.createExpertAvailabilityHandler))
		})

		// Bookings Routes
		r.Route("/bookings", func(r chi.Router) {
			r.Post("/{id}", app.requiredPermission("bookings:write", app.createBookingHandler))
			r.Put("/{id}", app.requiredPermission("bookings:write", app.rescheduleBookingHandler))

			r.Get("/me", app.requiredPermission("bookings:read", app.getAllBookingsForUser))
			r.Get("/me/{id}", app.requiredPermission("bookings:read", app.getABookingForUser))
			r.Get("/expert/{id}", app.requiredPermission("bookings:read", app.getABookingForExpert))
			r.Patch("/{id}/status", app.requiredPermission("bookings:read", app.approveBookingHandler))
			r.Post("/api/signature", app.requiredPermission("bookings:read", app.getSignatureHandler))

			// Payment Routes within Bookings
			r.Post("/{id}/payunit/initiate", app.requiredPermission("bookings:write", app.initializeBookingPaymentHandler))
			r.Post("/{id}/payunit/makepayment", app.requiredPermission("bookings:write", app.makePaymentHandler))   
			r.Get("/{id}/getpaymentproviders", app.requiredPermission("bookings:write", app.getPayunitPaymentProvidersHandler))

			// Send Reminders to experts and users
			r.Post("/{id}/send_expert", app.requiredPermission("bookings:write", app.sendBookingReminderToExpertHandler))
			r.Post("/{id}/send_user", app.requiredPermission("bookings:write", app.sendBookingReminderToUserHandler))
		})

		// Branches Routes
		r.Route("/branches", func(r chi.Router) {
			r.Post("/", app.requiredPermission("branches:write", app.createBranchHandler))
		})
	})

	return r
}
