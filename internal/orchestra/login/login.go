// Package login contains code to login with OONI orchestra.
package login

import (
	"context"
	"net/http"
	"time"

	"github.com/ooni/probe-engine/httpx/jsonapi"
	"github.com/ooni/probe-engine/log"
)

// Config contains configs for logging in with the OONI orchestra.
type Config struct {
	BaseURL    string
	ClientID   string
	HTTPClient *http.Client
	Logger     log.Logger
	Password   string
	UserAgent  string
}

type request struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Auth contains authentication info
type Auth struct {
	Expire time.Time `json:"expire"`
	Token  string    `json:"token"`
}

// Do logs this probe in with OONI orchestra
func Do(ctx context.Context, config Config) (*Auth, error) {
	req := &request{
		Password: config.Password,
		Username: config.ClientID,
	}
	var resp Auth
	err := (&jsonapi.Client{
		BaseURL:    config.BaseURL,
		HTTPClient: config.HTTPClient,
		Logger:     config.Logger,
		UserAgent:  config.UserAgent,
	}).Create(ctx, "/api/v1/login", req, &resp)
	if err != nil {
		return nil, err
	}
	// Implementation note: the API does not return 200 unless there
	// is success, so we don't bother with reading the error field
	return &resp, nil
}
