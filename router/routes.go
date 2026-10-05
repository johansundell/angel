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
		// Never UseLogger: the request log would store the submitted PIN.
		Route{
			Name:        "SubmitPIN",
			Method:      "POST",
			Pattern:     "/pin",
			HandlerFunc: handler.SubmitPIN,
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
			Pattern:     "/health",
			HandlerFunc: handler.HealthCheck,
		},
		Route{
			Name:        "Ping",
			Method:      "GET",
			Pattern:     "/ping/:argument",
			HandlerFunc: handler.Ping,
			UseLogger:   true,
		},
		Route{
			Name:        "Pong",
			Method:      "POST",
			Pattern:     "/pong",
			HandlerFunc: handler.Pong,
			UseLogger:   true,
			UseAuth:     true,
		},
		Route{
			Name:        "GetLogs",
			Method:      "GET",
			Pattern:     "/logs/:from/:to",
			HandlerFunc: handler.GetLogs,
			UseAuth:     true,
		},
	}
}
