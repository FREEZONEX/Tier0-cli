package cmd

import "testing"

func TestIsTier0OpenAPIEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		want     bool
	}{
		{endpoint: "/openapi/v1/uns/read", want: true},
		{endpoint: " /openapi/v1/info ", want: true},
		{endpoint: "/openapi", want: true},
		{endpoint: "/flow/source/api/device/status", want: false},
		{endpoint: "/flow/event/api/material/track", want: false},
		{endpoint: "/custom", want: false},
	}

	for _, test := range tests {
		if got := isTier0OpenAPIEndpoint(test.endpoint); got != test.want {
			t.Errorf("isTier0OpenAPIEndpoint(%q) = %t, want %t", test.endpoint, got, test.want)
		}
	}
}

func TestCheckAPIResponseOnlyValidatesOpenAPIEnvelopes(t *testing.T) {
	applicationResponse := `{"code":500,"data":{"success":false}}`
	if err := checkAPIResponse("/flow/event/api/material/track", applicationResponse); err != nil {
		t.Fatalf("custom Flow response was treated as an OpenAPI envelope: %v", err)
	}
	if err := checkAPIResponse("/openapi/v1/uns/read", applicationResponse); err == nil {
		t.Fatal("expected OpenAPI business error")
	}
}
