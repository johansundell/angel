package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck reports the service and storage state. It answers 503 when
// the storage ping fails, so health checks see the service as unhealthy.
func (h *Handler) HealthCheck(c *gin.Context) error {
	status := http.StatusOK
	dbStatus := "OK"
	if err := h.store.Ping(c.Request.Context()); err != nil {
		status = http.StatusServiceUnavailable
		dbStatus = err.Error()
	}

	data := gin.H{
		"title":    "Health Check",
		"name":     h.nameOfService,
		"version":  h.versionOfService,
		"dbStatus": dbStatus,
	}

	// JSON when the client prefers it (Accept), HTML otherwise, including
	// for browsers and requests without an Accept header.
	if c.NegotiateFormat(gin.MIMEHTML, gin.MIMEJSON) == gin.MIMEJSON {
		c.JSON(status, data)
		return nil
	}

	return h.render(c, status, "health.html", data)
}
