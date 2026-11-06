package client

import "testing"

func TestResponseContracts(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
		validate   func(string) error
	}{
		{"cloud whoami", `{"code":200,"data":{"workspaceID":23,"userID":9,"keyType":"service"}}`, true, ValidateWhoami},
		{"enterprise agent", `{"code":200,"data":{"workspaceID":1,"userID":0,"keyType":"agent"}}`, true, ValidateWhoami},
		{"unknown key type", `{"code":200,"data":{"workspaceID":1,"keyType":"future"}}`, true, ValidateWhoami},
		{"legacy type omitted", `{"code":200,"data":{"workspaceID":1}}`, true, ValidateWhoami},
		{"whoami html", `<html>login</html>`, false, ValidateWhoami},
		{"whoami empty", `{}`, false, ValidateWhoami},
		{"whoami empty data", `{"code":200,"data":{}}`, false, ValidateWhoami},
		{"whoami zero workspace", `{"code":200,"data":{"workspaceID":0}}`, false, ValidateWhoami},
		{"info cloud", `{"code":200,"data":{"version":"1.2.3"}}`, true, ValidateInfo},
		{"info enterprise", `{"code":200,"data":{"name":"edge-enterprise","version":"dev"}}`, true, ValidateInfo},
		{"info html", `<html>login</html>`, false, ValidateInfo},
		{"info empty", `{}`, false, ValidateInfo},
		{"info empty data", `{"code":200,"data":{}}`, false, ValidateInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if (tc.validate(tc.body) == nil) != tc.valid {
				t.Fatal("unexpected validation result")
			}
		})
	}
}
