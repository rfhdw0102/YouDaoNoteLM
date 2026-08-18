package entity_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"YoudaoNoteLm/internal/model/entity"
)

func TestNotionBindingDoesNotExposeTokenInJSON(t *testing.T) {
	binding := entity.NotionBinding{UserID: 7, AccessTokenEncrypted: "ciphertext"}
	encoded, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("ciphertext")) {
		t.Fatal("encrypted token must not be serialized")
	}
}
