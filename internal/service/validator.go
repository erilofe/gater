package service

import (
	"net/http"
	"strings"

	"github.com/pietroagazzi/gater/internal/config"
)

// Validator validates requests before routing them to proxy
type Validator struct {
	AllowWebsocket bool
}

// NewValidator creates a simple validator with all configurations
func NewValidator(validatorConfig *config.ValidatorConfig) *Validator {
	return &Validator{
		AllowWebsocket: *validatorConfig.AllowWebsocket,
	}
}

func (v *Validator) Handle(w http.ResponseWriter, r *http.Request) bool {
	if !v.AllowWebsocket {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && r.Method == http.MethodGet && r.Header.Get("Sec-WebSocket-Key") != "" && r.Header.Get("Sec-WebSocket-Version") == "13" {
			w.Header().Add("Connection", "close")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("Websocket connections are not allowed."))
			return false
		}
	}

	return true
}
