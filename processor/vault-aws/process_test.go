package vaultaws

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testAWSPath       = "/testing/path"
	testVaultEndpoint = "http://vault.local:8200"
	testRole          = "aRole"
	testClientToken   = "aToken"
)

type mockDoer struct {
	callCount  int
	successful bool
	goodtoken  bool
}

func mockServer(success bool, goodtoken bool) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !goodtoken {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"auth":{"client_token":"%s"}}`, testClientToken)
		return
	}))

	return ts
}

func TestInitConfig(t *testing.T) {

	type testSet struct {
		name        string
		active      bool
		shouldError bool
		config      func(*config)
	}

	tests := []testSet{
		testSet{"no vault", false, false, func(c *config) { c.AWSPath = testAWSPath }},
		testSet{"no AWSPath", false, false, func(c *config) { c.Vault = testVaultEndpoint }},
		testSet{"vault invalid", false, true, func(c *config) { c.Vault = "broken:/bad url"; c.AWSPath = testAWSPath; c.Role = testRole }},
		testSet{"missing role", false, true, func(c *config) { c.Vault = testVaultEndpoint; c.AWSPath = testAWSPath }},
		testSet{"ok", true, false, func(c *config) { c.Vault = testVaultEndpoint; c.AWSPath = testAWSPath; c.Role = testRole }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpc := &http.Client{}
			uut := New(httpc)
			c := uut.Config()
			test.config(c)
			active, err := uut.Init(c)

			assert.Equal(t, test.active, active, "no config should not error")
			if test.shouldError {
				assert.Error(t, err, "should error")
			} else {
				assert.NoError(t, err, "should not error")
			}
		})
	}
}

func makeEnvConsul(args []string) func(string) []string {
	return func(vault string) []string {
		varg := fmt.Sprintf("-vault-addr=%s", vault)
		envConsul := []string{"/bin/envconsul", "-config=/service.hcl", varg, "-vault-token=aToken"}
		return append(envConsul, args...)
	}
}

func TestApply(t *testing.T) {
	type testSet struct {
		name            string
		args            []string
		env             []string
		vaultSuccess    bool
		vaultGivesToken bool
		expectedArgs    func(string) []string
		expectedEnv     []string
		shouldError     bool
	}

	testEnv := []string{"abc=def"}
	tests := []testSet{
		testSet{"vault failed", []string{}, []string{}, false, false, nil, []string{}, true},
		testSet{"no token", []string{}, []string{}, true, false, nil, []string{}, true},
		testSet{"ok", []string{}, []string{}, true, true, makeEnvConsul([]string{}), []string{}, false},
		testSet{"ok env copied", []string{}, testEnv, true, true, makeEnvConsul([]string{}), testEnv, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := mockServer(test.vaultSuccess, test.vaultGivesToken)
			defer ts.Close()

			httpc := &http.Client{}
			uut := New(httpc)

			c := uut.Config()
			c.AWSPath = testAWSPath
			c.Vault = ts.URL
			c.Role = testRole
			c.Filename = "/service.hcl"

			active, err := uut.Init(c)

			assert.True(t, active, "establishing test should be successful")
			assert.NoError(t, err, "establishing test should not error")

			args, env, err := uut.Apply(test.args, test.env)

			if test.shouldError {
				assert.Error(t, err, "should error")
			} else {
				assert.NoError(t, err, "should not error")
				assert.Equal(t, test.expectedEnv, env, "output args should match")
				if test.expectedArgs != nil {
					expectedArgs := test.expectedArgs(c.Vault)
					assert.Equal(t, expectedArgs, args, "output args should match")
				}
			}
		})
	}

}
