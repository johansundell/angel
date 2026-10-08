package router

import (
	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/handlers"
)

// GetRoutes returns default Route configurations for the provided handler
func GetRoutes(handler *handlers.Handler) Routes {
	return Routes{
		Route{
			Name:        "Entry",
			Method:      "GET",
			Pattern:     "/",
			HandlerFunc: handler.Entry,
		},
		Route{
			Name:        "SubmitPIN",
			Method:      "POST",
			Pattern:     "/pin",
			HandlerFunc: handler.SubmitPIN,
		},
		Route{
			Name:        "Logout",
			Method:      "POST",
			Pattern:     "/logout",
			HandlerFunc: handler.Logout,
		},
		Route{
			Name:        "CaregiverView",
			Method:      "GET",
			Pattern:     "/note",
			HandlerFunc: handler.CaregiverView,
			Role:        auth.RoleCaregiver,
		},
		Route{
			Name:        "AcknowledgeNote",
			Method:      "POST",
			Pattern:     "/note/ack",
			HandlerFunc: handler.AcknowledgeNote,
			Role:        auth.RoleCaregiver,
		},
		Route{
			Name:        "ClientDashboard",
			Method:      "GET",
			Pattern:     "/admin",
			HandlerFunc: handler.ClientDashboard,
			Role:        auth.RoleClient,
		},
		Route{
			Name:        "AckFeed",
			Method:      "GET",
			Pattern:     "/admin/acks",
			HandlerFunc: handler.AckFeed,
			Role:        auth.RoleClient,
		},
		Route{
			Name:        "SaveNote",
			Method:      "POST",
			Pattern:     "/admin/note",
			HandlerFunc: handler.SaveNote,
			Role:        auth.RoleClient,
		},
		Route{
			Name:        "ClearNote",
			Method:      "POST",
			Pattern:     "/admin/note/clear",
			HandlerFunc: handler.ClearNote,
			Role:        auth.RoleClient,
		},
		Route{
			Name:        "SaveAdvanceNote",
			Method:      "POST",
			Pattern:     "/admin/advance",
			HandlerFunc: handler.SaveAdvanceNote,
			Role:        auth.RoleClient,
		},
		Route{
			Name:        "ClearAdvanceNote",
			Method:      "POST",
			Pattern:     "/admin/advance/clear",
			HandlerFunc: handler.ClearAdvanceNote,
			Role:        auth.RoleClient,
		},
		Route{
			Name:        "HealthCheck",
			Method:      "GET",
			Pattern:     "/healthz",
			HandlerFunc: handler.HealthCheck,
		},
	}
}
