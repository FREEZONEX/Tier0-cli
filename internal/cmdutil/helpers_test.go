package cmdutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
	"github.com/FREEZONEX/Tier0-cli/internal/errs"
)

func TestCheckResponseDetectsBusinessSuccessFalse(t *testing.T) {
	resp := `{"code":200,"data":{"success":true,"results":[{"topic":"Plant/Metric/A","success":true},{"topic":"Plant/Metric/B","success":false}]}}`
	err := CheckResponse(resp)
	if err == nil {
		t.Fatal("expected business failure")
	}
	if !strings.Contains(err.Error(), "Plant/Metric/B") {
		t.Fatalf("error = %q, want failed topic", err.Error())
	}
}

func TestCheckResponseBusinessCodeIsNotHTTPRetryable(t *testing.T) {
	err := CheckResponse(`{"code":100009,"msg":"not found"}`)
	if err == nil {
		t.Fatal("expected business error")
	}
	var apiErr *apierr.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.ToEnvelope().Error.Retryable {
		t.Fatal("business code must not be marked retryable as an HTTP 5xx")
	}
	if apiErr.ToEnvelope().Error.Code != 100009 {
		t.Fatalf("business code = %d", apiErr.ToEnvelope().Error.Code)
	}
}

func TestCheckResponseDetectsOuterDataSuccessFalse(t *testing.T) {
	resp := `{"code":200,"data":{"success":false,"results":[]}}`
	err := CheckResponse(resp)
	if err == nil {
		t.Fatal("expected outer data success failure")
	}
	if !strings.Contains(err.Error(), "data.success=false") {
		t.Fatalf("error = %q, want data.success=false", err.Error())
	}
}

func TestIsClassifiedDistinguishesAPIErrorsFromCobraFallbacks(t *testing.T) {
	apiErr := apierr.New(500, `{"code":500,"msg":"server error"}`)
	if !IsClassified(apiErr) {
		t.Fatal("APIError must be recognized as classified")
	}
	if CategoryOf(apiErr) != errs.CategoryAPI {
		t.Fatalf("category = %q, want api", CategoryOf(apiErr))
	}
	if IsClassified(assertionError("unknown flag")) {
		t.Fatal("plain Cobra-style errors must remain unclassified")
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
