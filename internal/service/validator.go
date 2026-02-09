package service

import (
	"net/http"
	"strings"

	"github.com/pietroagazzi/gater/internal/config"
)

// Validator validates requests before routing them to proxy
type Validator struct {
	Websocket bool
}

// NewValidator creates a simple validator with all configurations
func NewValidator(validatorConfig *config.ValidatorConfig) *Validator {
	return &Validator{
		Websocket: *validatorConfig.Websocket,
	}
}

func (v *Validator) Handle(w http.ResponseWriter, r *http.Request) bool {
	if !v.Websocket {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && r.Method == http.MethodGet && r.Header.Get("Sec-WebSocket-Key") != "" && r.Header.Get("Sec-WebSocket-Version") == "13" {
			w.Header().Add("Upgrade", "websocket")
			w.Header().Add("Connection", "Upgrade")
			w.Header().Add("Sec-WebSocket-Version", "13")
			w.WriteHeader(http.StatusUpgradeRequired)
			return false
		}
	}

	return true
}
