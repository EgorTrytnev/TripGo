package domainerror_test

import (
	"errors"
	"net/http"
	"testing"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{{"not found", domaintrip.ErrNotFound, http.StatusNotFound, "trip_not_found"}, {"completed", domaintrip.ErrCompleted, http.StatusConflict, "trip_completed"}, {"driver busy", domaintrip.ErrDriverBusy, http.StatusConflict, "driver_busy"}, {"validation", domaintrip.ValidationError{Field: "price", Message: "price must be greater than or equal to zero"}, http.StatusBadRequest, "invalid_request"}, {"internal", errors.New("secret database failure"), http.StatusInternalServerError, "internal_error"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, p := domainerror.Problem(tt.err)
			if status != tt.status || p.Code != tt.code {
				t.Fatalf("got %d/%s, want %d/%s", status, p.Code, tt.status, tt.code)
			}
			if status == http.StatusInternalServerError && *p.Detail == "secret database failure" {
				t.Fatal("internal error details leaked")
			}
		})
	}
}
