package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAppBasePath(t *testing.T) {
	assert.Equal(t, "", normalizeAppBasePath(""))
	assert.Equal(t, "", normalizeAppBasePath("/"))
	assert.Equal(t, "/zhuxiangwei-macmini", normalizeAppBasePath("zhuxiangwei-macmini"))
	assert.Equal(t, "/zhuxiangwei-macmini", normalizeAppBasePath("/zhuxiangwei-macmini/"))
}

func TestWithAppBasePathStripsConfiguredPrefix(t *testing.T) {
	original := os.Getenv("VITE_APP_BASE_PATH")
	t.Cleanup(func() {
		require.NoError(t, os.Setenv("VITE_APP_BASE_PATH", original))
	})
	require.NoError(t, os.Setenv("VITE_APP_BASE_PATH", "/zhuxiangwei-macmini"))

	handler := withAppBasePath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/zhuxiangwei-macmini/static/js/app.js", nil)
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/static/js/app.js", recorder.Body.String())
}

func TestWithAppBasePathRewritesRootRelativeRedirects(t *testing.T) {
	original := os.Getenv("VITE_APP_BASE_PATH")
	t.Cleanup(func() {
		require.NoError(t, os.Setenv("VITE_APP_BASE_PATH", original))
	})
	require.NoError(t, os.Setenv("VITE_APP_BASE_PATH", "/zhuxiangwei-macmini"))

	handler := withAppBasePath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/channel/?p=1", http.StatusMovedPermanently)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/zhuxiangwei-macmini/api/channel?p=1", nil)
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "/zhuxiangwei-macmini/api/channel/?p=1", recorder.Header().Get("Location"))
}
