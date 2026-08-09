package main

import (
	"os"
	"strings"
	"testing"
)

func TestDockerStartupSeedsFreshDatabase(t *testing.T) {
	entrypoint, err := os.ReadFile("../../../deploy/docker-entrypoint.sh")
	if err != nil {
		t.Fatalf("read Docker entrypoint: %v", err)
	}
	for _, want := range []string{
		"fanti seed --if-empty --download-dir /app/datasets",
		"exec /app/fanti \"$@\"",
	} {
		if !strings.Contains(string(entrypoint), want) {
			t.Errorf("Docker entrypoint missing %q", want)
		}
	}

	dockerfile, err := os.ReadFile("../../../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	for _, want := range []string{
		"/src/backend/data/downloads /app/datasets",
		"deploy/docker-entrypoint.sh /app/entrypoint.sh",
		`ENTRYPOINT ["/app/entrypoint.sh"]`,
	} {
		if !strings.Contains(string(dockerfile), want) {
			t.Errorf("Dockerfile missing %q", want)
		}
	}
}
