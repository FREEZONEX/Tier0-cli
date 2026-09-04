package cmd

import (
	"encoding/json"
	"testing"
)

func TestAssetsUploadResponseAcceptsNumericAndStringFileID(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "numeric", body: `{"code":200,"data":{"fileId":42,"filePath":"workspace/test.txt","postUrl":"https://upload.example.test"}}`},
		{name: "string", body: `{"code":200,"data":{"fileId":"42","filePath":"workspace/test.txt","postUrl":"https://upload.example.test"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var response assetsUploadResponse
			if err := json.Unmarshal([]byte(test.body), &response); err != nil {
				t.Fatal(err)
			}
			if got := int64(response.Data.FileID); got != 42 {
				t.Fatalf("fileId = %d, want 42", got)
			}
		})
	}
}

func TestAssetsUploadResponseRejectsInvalidStringFileID(t *testing.T) {
	var response assetsUploadResponse
	err := json.Unmarshal([]byte(`{"code":200,"data":{"fileId":"not-an-id"}}`), &response)
	if err == nil {
		t.Fatal("expected invalid string fileId to fail")
	}
}
