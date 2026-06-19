package user

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/apperror"
)

func TestNewHTTPHandler_registersRoutes(t *testing.T) {
	mux, _, _ := newHTTPTestMux(&fakeStore{findUsers: []user{}}, &fakeHasher{})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHTTPHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		want := []user{{ID: "user-1", FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"}}
		mux, _, _ := newHTTPTestMux(&fakeStore{findUsers: want}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))

		requireStatus(t, rec, http.StatusOK)
		var got []user
		decodeJSON(t, rec.Body, &got)
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("users: got %#v, want %#v", got, want)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{
			findErr: apperror.NewInternalError("user.service.Find", "Failed to fetch users", nil),
		}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))

		requireErrorResponse(t, rec, http.StatusInternalServerError, "Internal error")
	})
}

func TestHTTPHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		want := &user{ID: "user-1", FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"}
		mux, _, _ := newHTTPTestMux(&fakeStore{findByIDUser: want}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/user-1", nil))

		requireStatus(t, rec, http.StatusOK)
		var got user
		decodeJSON(t, rec.Body, &got)
		if got != *want {
			t.Fatalf("user: got %#v, want %#v", got, *want)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{
			findByIDErr: apperror.NewNotFoundError("user.service.FindByID", "User not found", nil),
		}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/user-1", nil))

		requireErrorResponse(t, rec, http.StatusNotFound, "User not found")
	})
}

func TestHTTPHandler_Insert(t *testing.T) {
	t.Run("invalid body", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("{")))

		requireErrorResponse(t, rec, http.StatusBadRequest, "Invalid request body")
	})

	t.Run("service error", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{
			insertErr: apperror.NewConflictError("user.store.Insert", "User already exists", nil),
		}, &fakeHasher{hash: "hashed-password"})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{
			"firstName": "Ada",
			"lastName": "Lovelace",
			"email": "ada@example.com",
			"password": "plain-password"
		}`)))

		requireErrorResponse(t, rec, http.StatusConflict, "User already exists")
	})

	t.Run("success", func(t *testing.T) {
		mux, store, hasher := newHTTPTestMux(&fakeStore{insertID: "user-1"}, &fakeHasher{hash: "hashed-password"})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{
			"firstName": "Ada",
			"lastName": "Lovelace",
			"email": "ada@example.com",
			"password": "plain-password"
		}`)))

		requireStatus(t, rec, http.StatusCreated)
		var got map[string]string
		decodeJSON(t, rec.Body, &got)
		if got["id"] != "user-1" {
			t.Fatalf("id: got %q, want user-1", got["id"])
		}
		if store.insertedUser == nil {
			t.Fatal("service did not insert user")
		}
		if !hasher.called {
			t.Fatal("service did not hash password")
		}
	})
}

func TestHTTPHandler_Update(t *testing.T) {
	t.Run("invalid body", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/users/user-1", strings.NewReader("{")))

		requireErrorResponse(t, rec, http.StatusBadRequest, "Invalid request body")
	})

	t.Run("service error", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{
			findByIDErr: apperror.NewNotFoundError("user.store.FindByID", "User not found", nil),
		}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/users/user-1", strings.NewReader(`{
			"firstName": "Ada",
			"lastName": "Lovelace"
		}`)))

		requireErrorResponse(t, rec, http.StatusNotFound, "User not found")
	})

	t.Run("success", func(t *testing.T) {
		store := &fakeStore{findByIDUser: &user{ID: "user-1", Email: "ada@example.com"}}
		mux, _, _ := newHTTPTestMux(store, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/users/user-1", strings.NewReader(`{
			"firstName": "Ada",
			"lastName": "Lovelace"
		}`)))

		requireStatus(t, rec, http.StatusNoContent)
		if body := strings.TrimSpace(rec.Body.String()); body != "" {
			t.Fatalf("body: got %q, want empty", body)
		}
		if store.updatedID != "user-1" {
			t.Fatalf("updated id: got %q, want user-1", store.updatedID)
		}
	})
}

func TestHTTPHandler_Delete(t *testing.T) {
	t.Run("service error", func(t *testing.T) {
		mux, _, _ := newHTTPTestMux(&fakeStore{
			softDeleteErr: apperror.NewNotFoundError("user.store.SoftDelete", "User not found", nil),
		}, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/users/user-1", nil))

		requireErrorResponse(t, rec, http.StatusNotFound, "User not found")
	})

	t.Run("success", func(t *testing.T) {
		store := &fakeStore{}
		mux, _, _ := newHTTPTestMux(store, &fakeHasher{})

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/users/user-1", nil))

		requireStatus(t, rec, http.StatusNoContent)
		if body := strings.TrimSpace(rec.Body.String()); body != "" {
			t.Fatalf("body: got %q, want empty", body)
		}
		if store.softDeletedID != "user-1" {
			t.Fatalf("soft deleted id: got %q, want user-1", store.softDeletedID)
		}
	})
}

func newHTTPTestMux(store *fakeStore, hasher *fakeHasher) (*http.ServeMux, *fakeStore, *fakeHasher) {
	mux := http.NewServeMux()
	newHTTPHandler(newTestService(store, hasher), mux)
	return mux, store, hasher
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("status: got %d, want %d; body=%q", rec.Code, want, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type: got %q, want application/json", got)
	}
}

func requireErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()

	requireStatus(t, rec, wantStatus)
	var body map[string]string
	decodeJSON(t, rec.Body, &body)
	if body["error"] != wantMsg {
		t.Fatalf("error: got %q, want %q", body["error"], wantMsg)
	}
}

func decodeJSON(t *testing.T, r io.Reader, v any) {
	t.Helper()

	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}
