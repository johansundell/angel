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
			Pattern:     "/caregiver",
			HandlerFunc: handler.CaregiverView,
			Role:        auth.RoleCaregiver,
		},
		Route{
			Name:        "ClientDashboard",
			Method:      "GET",
			Pattern:     "/client",
			HandlerFunc: handler.ClientDashboard,
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
