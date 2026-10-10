package main

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

const csrfCookieName = "csrf_token"

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func setCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    generateCSRFToken(),
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
	})
}

func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" {
		return false
	}
	return cookie.Value == header
}

func requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	if !validateCSRF(r) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return false
	}
	return true
}
