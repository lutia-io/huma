package user

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestUser_JSONContract(t *testing.T) {
	deletedAt := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	u := user{
		ID:        "user-1",
		FirstName: "Ada",
		LastName:  "Lovelace",
		Email:     "ada@example.com",
		Password:  "hashed-password",
		CreatedAt: deletedAt,
		UpdatedAt: deletedAt,
		DeletedAt: &deletedAt,
	}

	b, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"id":"user-1"`,
		`"firstName":"Ada"`,
		`"lastName":"Lovelace"`,
		`"email":"ada@example.com"`,
		`"createdAt":`,
		`"updatedAt":`,
		`"deletedAt":`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("json %s missing from %s", want, got)
		}
	}
	if strings.Contains(got, "password") || strings.Contains(got, "hashed-password") {
		t.Fatalf("password leaked in json: %s", got)
	}
}

func TestUser_JSONOmitsDeletedAtWhenNil(t *testing.T) {
	b, err := json.Marshal(user{ID: "user-1"})
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	if strings.Contains(string(b), "deletedAt") {
		t.Fatalf("deletedAt should be omitted: %s", b)
	}
}

func TestUser_BSONContract(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	b, err := bson.Marshal(user{
		FirstName: "Ada",
		LastName:  "Lovelace",
		Email:     "ada@example.com",
		Password:  "hashed-password",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("marshal bson: %v", err)
	}

	var got bson.M
	if err := bson.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal bson: %v", err)
	}
	for _, key := range []string{"first_name", "last_name", "email", "password", "created_at", "updated_at"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("bson key %q missing from %#v", key, got)
		}
	}
	if _, ok := got["_id"]; ok {
		t.Fatalf("_id should be omitted when empty: %#v", got)
	}
	if _, ok := got["deleted_at"]; ok {
		t.Fatalf("deleted_at should be omitted when nil: %#v", got)
	}
}

func TestInsertUserRequest_JSONContract(t *testing.T) {
	var req insertUserRequest
	if err := json.Unmarshal([]byte(`{
		"firstName": "Ada",
		"lastName": "Lovelace",
		"email": "ada@example.com",
		"password": "plain-password"
	}`), &req); err != nil {
		t.Fatalf("unmarshal insert request: %v", err)
	}
	if req.FirstName != "Ada" || req.LastName != "Lovelace" || req.Email != "ada@example.com" || req.Password != "plain-password" {
		t.Fatalf("request: got %#v", req)
	}
}

func TestUpdateUserRequest_JSONContract(t *testing.T) {
	var req updateUserRequest
	if err := json.Unmarshal([]byte(`{
		"firstName": "Ada",
		"lastName": "Lovelace"
	}`), &req); err != nil {
		t.Fatalf("unmarshal update request: %v", err)
	}
	if req.FirstName != "Ada" || req.LastName != "Lovelace" {
		t.Fatalf("request: got %#v", req)
	}
}
