package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth_WhenTokenEmpty_ShouldCallNextHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mw := BearerAuth("")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("when token empty: want 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("when token empty: want body ok, got %q", body)
	}
}

func TestBearerAuth_WhenTokenSetAndHeaderMissing_ShouldReturn401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called when auth fails")
	})
	mw := BearerAuth("secret")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing header: want 401, got %d", rec.Code)
	}
}

func TestBearerAuth_WhenTokenSetAndHeaderWrong_ShouldReturn401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called when auth fails")
	})
	mw := BearerAuth("secret")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: want 401, got %d", rec.Code)
	}
}

func TestBearerAuth_WhenTokenSetAndHeaderCorrect_ShouldCallNextHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mw := BearerAuth("secret")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("correct token: want 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("correct token: want body ok, got %q", body)
	}
}

func TestBearerAuth_WhenAuthorizationNotBearer_ShouldReturn401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})
	mw := BearerAuth("secret")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("non-Bearer scheme: want 401, got %d", rec.Code)
	}
}
