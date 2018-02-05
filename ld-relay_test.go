package main

import (
	"github.com/gorilla/mux"
	"github.com/streamrail/concurrent-map"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"bytes"
	"context"
	ld "gopkg.in/launchdarkly/go-client.v2"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type FakeLDClient struct {
	mock.Mock
}

func (client *FakeLDClient) AllFlags(user ld.User) map[string]interface{} {
	flags := make(map[string]interface{})
	flags["some-flag-key"] = true
	flags["another-flag-key"] = 3
	return flags
}

// Returns a key matching the UUID header pattern
func key() string {
	return "mob-ffffffff-ffff-4fff-afff-ffffffffffff"
}

func user() string {
	return "eyJrZXkiOiJ0ZXN0In0="
}

func handler() muxHandler {
	clients := cmap.New()
	clients.Set(key(), &FakeLDClient{})
	return muxHandler{clients: clients}
}

func buildRequest(verb string, vars map[string]string, headers map[string]string, body string) *http.Request {
	req, _ := http.NewRequest(verb, "", bytes.NewBuffer([]byte(body)))
	req = mux.SetURLVars(req, vars)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	ctx := evalContextImpl{client: &FakeLDClient{}}
	req = req.WithContext(context.WithValue(req.Context(), "context", ctx))
	return req
}

func TestGetFlagEvalSucceeds(t *testing.T) {
	vars := map[string]string{"user": user()}
	req := buildRequest("GET", vars, nil, "")
	resp := httptest.NewRecorder()
	evaluateAllFeatureFlags(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	b, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, `{"another-flag-key":3,"some-flag-key":true}`, string(b))
}

func TestReportFlagEvalSucceeds(t *testing.T) {
	vars := map[string]string{"user": user()}
	headers := map[string]string{"Content-Type": "application/json"}
	req := buildRequest("REPORT", vars, headers, `{"user":"key"}`)
	resp := httptest.NewRecorder()
	evaluateAllFeatureFlags(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	b, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, `{"another-flag-key":3,"some-flag-key":true}`, string(b))
}

func TestAuthorizeMethodFailsOnInvalidAuthKey(t *testing.T) {
	vars := map[string]string{"user": user()}
	headers := map[string]string{"Authorization": "mob-eeeeeeee-eeee-4eee-aeee-eeeeeeeeeeee", "Content-Type": "application/json"}
	req := buildRequest("REPORT", vars, headers, `{"user":"key"}`)
	resp := httptest.NewRecorder()
	handler().authorizeMethod(func(http.ResponseWriter, *http.Request) { t.Fail() })(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestFlagEvalFailsOnInvalidUserJson(t *testing.T) {
	vars := map[string]string{"user": user()}
	headers := map[string]string{"Content-Type": "application/json"}
	req := buildRequest("REPORT", vars, headers, `{"user":"key"}notjson`)
	resp := httptest.NewRecorder()
	evaluateAllFeatureFlags(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestFindEnvironmentFailsOnInvalidEnvId(t *testing.T) {
	vars := map[string]string{"envId": "blah", "user": user()}
	req := buildRequest("GET", vars, nil, "")
	resp := httptest.NewRecorder()
	handler().findEnvironment(evaluateAllFeatureFlags)(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
}
