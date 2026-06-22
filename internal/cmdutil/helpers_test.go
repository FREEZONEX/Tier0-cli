package cmdutil

import (
	"strings"
	"testing"
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
