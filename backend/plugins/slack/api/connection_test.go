/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/slack/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

// newFakeSlack serves auth.test under {server}/api/ with the given status and
// body, and records the Authorization header and the request path.
func newFakeSlack(t *testing.T, status int, body string) (endpoint string, lastAuth *string, lastPath *string) {
	var auth, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL + "/api/", &auth, &path
}

func setupTestGlobals() {
	mockRes := unithelper.DummyBasicRes(func(mockDal *mockdal.Dal) {})
	mockRes.On("GetConfigReader").Return(config.GetConfig())
	basicRes = mockRes
	vld = validator.New()
}

func newTestSlackConn(endpoint string, token string) models.SlackConn {
	connection := models.SlackConn{}
	connection.Endpoint = endpoint
	connection.Token = token
	return connection
}

func TestTestConnection(t *testing.T) {
	setupTestGlobals()

	t.Run("ok:true succeeds and calls auth.test with the token", func(t *testing.T) {
		endpoint, lastAuth, lastPath := newFakeSlack(t, http.StatusOK, `{"ok":true,"team":"devlake","user":"bot"}`)
		result, err := testConnection(context.TODO(), newTestSlackConn(endpoint, "xoxb-valid-token"))
		assert.Nil(t, err)
		if assert.NotNil(t, result) {
			assert.True(t, result.Success)
			assert.NotContains(t, result.Connection.Token, "valid-token")
		}
		assert.Equal(t, "/api/auth.test", *lastPath)
		assert.Equal(t, "Bearer xoxb-valid-token", *lastAuth)
	})

	t.Run("ok:false fails with the slack error code", func(t *testing.T) {
		endpoint, _, _ := newFakeSlack(t, http.StatusOK, `{"ok":false,"error":"invalid_auth"}`)
		result, err := testConnection(context.TODO(), newTestSlackConn(endpoint, "xoxb-bad-token"))
		assert.Nil(t, result)
		if assert.NotNil(t, err) {
			assert.Contains(t, err.Error(), "slack auth.test error: invalid_auth")
			assert.Equal(t, http.StatusBadRequest, err.GetType().GetHttpCode())
		}
	})

	t.Run("ok:false without an error code still fails", func(t *testing.T) {
		endpoint, _, _ := newFakeSlack(t, http.StatusOK, `{"ok":false}`)
		result, err := testConnection(context.TODO(), newTestSlackConn(endpoint, "xoxb-bad-token"))
		assert.Nil(t, result)
		if assert.NotNil(t, err) {
			assert.Contains(t, err.Error(), "slack auth.test error: unknown error")
		}
	})

	t.Run("a 401 status fails as a bad request", func(t *testing.T) {
		endpoint, _, _ := newFakeSlack(t, http.StatusUnauthorized, `{}`)
		result, err := testConnection(context.TODO(), newTestSlackConn(endpoint, "xoxb-bad-token"))
		assert.Nil(t, result)
		if assert.NotNil(t, err) {
			assert.Contains(t, err.Error(), "authentication failed")
			assert.Equal(t, http.StatusBadRequest, err.GetType().GetHttpCode())
		}
	})

	t.Run("a non-200 status fails", func(t *testing.T) {
		endpoint, _, _ := newFakeSlack(t, http.StatusInternalServerError, `{}`)
		result, err := testConnection(context.TODO(), newTestSlackConn(endpoint, "xoxb-valid-token"))
		assert.Nil(t, result)
		if assert.NotNil(t, err) {
			assert.Contains(t, err.Error(), "500")
		}
	})
}
