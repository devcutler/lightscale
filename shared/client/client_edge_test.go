package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIErrorFormat(t *testing.T) {
	e := &APIError{Status: 404, Body: "not found"}
	if got, want := e.Error(), "api: status 404: not found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestIsNotFound(t *testing.T) {
	c := New("/tmp/x.sock")
	if !c.IsNotFound(&APIError{Status: http.StatusNotFound}) {
		t.Error("IsNotFound(404 APIError) = false, want true")
	}
	if c.IsNotFound(&APIError{Status: http.StatusConflict}) {
		t.Error("IsNotFound(409 APIError) = true, want false")
	}
	if c.IsNotFound(errors.New("plain")) {
		t.Error("IsNotFound(plain err) = true, want false")
	}
	if c.IsNotFound(nil) {
		t.Error("IsNotFound(nil) = true, want false")
	}

	wrapped := errors.Join(errors.New("ctx"), &APIError{Status: http.StatusNotFound})
	if !c.IsNotFound(wrapped) {
		t.Error("IsNotFound(wrapped 404) = false, want true")
	}
}

type echo struct {
	Name string `json:"name"`
}

func TestGetDecodesJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/thing" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"hello"}`))
	}))
	defer ts.Close()
	t.Setenv("LIGHTSCALE_URL", ts.URL)

	c := New("")
	var out echo
	if err := c.Get(context.Background(), "/api/thing", &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if out.Name != "hello" {
		t.Errorf("Name = %q, want hello", out.Name)
	}
}

func TestPostSendsBodyAndDecodes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var in echo
		if err := decode(r, &in); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if in.Name != "req" {
			t.Errorf("body Name = %q, want req", in.Name)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"resp"}`))
	}))
	defer ts.Close()
	t.Setenv("LIGHTSCALE_URL", ts.URL)

	c := New("")
	var out echo
	if err := c.Post(context.Background(), "/api/thing", echo{Name: "req"}, &out); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if out.Name != "resp" {
		t.Errorf("Name = %q, want resp", out.Name)
	}
}

func TestNon2xxReturnsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("name taken"))
	}))
	defer ts.Close()
	t.Setenv("LIGHTSCALE_URL", ts.URL)

	c := New("")
	err := c.Get(context.Background(), "/api/thing", &echo{})
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if ae.Status != http.StatusConflict {
		t.Errorf("Status = %d, want 409", ae.Status)
	}
	if ae.Body != "name taken" {
		t.Errorf("Body = %q, want 'name taken'", ae.Body)
	}
}

func TestNoContentLeavesOutUntouched(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	t.Setenv("LIGHTSCALE_URL", ts.URL)

	c := New("")
	out := echo{Name: "untouched"}
	if err := c.Post(context.Background(), "/api/thing", nil, &out); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if out.Name != "untouched" {
		t.Errorf("Name = %q, want untouched (204 must not decode)", out.Name)
	}
}

func TestGetTextReturnsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain text body"))
	}))
	defer ts.Close()
	t.Setenv("LIGHTSCALE_URL", ts.URL)

	c := New("")
	got, err := c.GetText(context.Background(), "/api/conf")
	if err != nil {
		t.Fatalf("GetText: %v", err)
	}
	if got != "plain text body" {
		t.Errorf("GetText = %q", got)
	}
}

func TestGetTextErrorReturnsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("no such user"))
	}))
	defer ts.Close()
	t.Setenv("LIGHTSCALE_URL", ts.URL)

	c := New("")
	_, err := c.GetText(context.Background(), "/api/conf")
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if ae.Status != http.StatusNotFound || ae.Body != "no such user" {
		t.Errorf("APIError = %+v", ae)
	}
	if !c.IsNotFound(err) {
		t.Error("IsNotFound on GetText 404 = false, want true")
	}
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
