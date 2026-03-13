package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
)

type failingHelper struct{}

func (failingHelper) Get(_ string) (string, string, error) {
	return "", "", errors.New("authentication exploded")
}
func (failingHelper) Add(_ *credentials.Credentials) error { return nil }
func (failingHelper) Delete(_ string) error                { return nil }
func (failingHelper) List() (map[string]string, error)     { return nil, nil }

// TestServeGetWritesErrorToOut pins the docker credential protocol contract:
// docker reads helper errors from stdout, so a failure must land on the out
// writer, not stderr.
func TestServeGetWritesErrorToOut(t *testing.T) {
	var out bytes.Buffer
	err := serveGet(failingHelper{}, strings.NewReader("myregistry.azurecr.io"), &out)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(out.String(), "authentication exploded") {
		t.Errorf("error text missing from out writer, got %q", out.String())
	}
}
