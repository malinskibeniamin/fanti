package seed

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const horseOracleTitle = "File:馬-oracle.svg"

func TestQueryCommonsUsesEtiquetteHeadersAndReadsGzip(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.UserAgent(), "Fanti/") {
			t.Errorf("User-Agent = %q, want Fanti identity", req.UserAgent())
		}
		if got := req.URL.Query().Get("maxlag"); got != "5" {
			t.Errorf("maxlag = %q, want 5", got)
		}
		if got := req.URL.Query().Get("titles"); got != horseOracleTitle {
			t.Errorf("titles = %q", got)
		}
		if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			t.Errorf("Accept-Encoding = %q, want gzip", req.Header.Get("Accept-Encoding"))
		}

		w.Header().Set("Content-Encoding", "gzip")
		writer := gzip.NewWriter(w)
		_, _ = writer.Write([]byte(
			`{"query":{"pages":[{"title":"File:馬-oracle.svg","imageinfo":[{"extmetadata":{"Categories":{"value":123}}}]}]}}`,
		))
		if err := writer.Close(); err != nil {
			t.Errorf("close gzip response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	result, err := queryCommons(context.Background(), CharacterHistoryOptions{
		APIURL: server.URL,
		Client: server.Client(),
	}, []string{horseOracleTitle})
	if err != nil {
		t.Fatalf("queryCommons: %v", err)
	}
	if len(result.Query.Pages) != 1 {
		t.Errorf("result = %+v", result)
	}
}

func TestQueryCommonsRetriesTemporaryFailure(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "try later", http.StatusServiceUnavailable)

			return
		}
		_, _ = w.Write([]byte(`{"query":{"pages":[]}}`))
	}))
	t.Cleanup(server.Close)

	if _, err := queryCommons(context.Background(), CharacterHistoryOptions{
		APIURL: server.URL,
		Client: server.Client(),
	}, []string{horseOracleTitle}); err != nil {
		t.Fatalf("queryCommons: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestHistoryRetryDelayHonorsRetryAfter(t *testing.T) {
	response := &http.Response{Header: http.Header{"Retry-After": {"2"}}}

	if got := nextHistoryRetryDelay(response, 100*time.Millisecond); got != 2*time.Second {
		t.Errorf("retry delay = %s, want 2s", got)
	}
}

func TestValidateHistorySVGRejectsExternalStyleResource(t *testing.T) {
	err := validateHistorySVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg"><path style="fill:url(https://tracker.example/ink)"/></svg>`,
	))
	if err == nil {
		t.Fatal("validateHistorySVG accepted an external style resource")
	}
}

func TestValidateHistorySVGAllowsStandardNamespace(t *testing.T) {
	err := validateHistorySVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg"><path style="fill:#000" d="M0 0"/></svg>`,
	))
	if err != nil {
		t.Fatalf("validateHistorySVG: %v", err)
	}
}

func TestValidateHistorySVGAllowsStandardW3CDoctype(t *testing.T) {
	err := validateHistorySVG([]byte(`<?xml version="1.0"?>
		<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.0//EN"
			"http://www.w3.org/TR/2001/REC-SVG-20010904/DTD/svg10.dtd">
		<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`))
	if err != nil {
		t.Fatalf("validateHistorySVG: %v", err)
	}
}

func TestValidateHistorySVGRejectsDoctypeEntities(t *testing.T) {
	err := validateHistorySVG([]byte(
		`<!DOCTYPE svg [<!ENTITY tracked SYSTEM "https://tracker.example/pixel">]><svg/>`,
	))
	if err == nil {
		t.Fatal("validateHistorySVG accepted a doctype entity")
	}
}

func TestQueryCommonsRejectsAPIErrorPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(
			`{"error":{"code":"maxlag","info":"Waiting for replication lag to clear"}}`,
		))
	}))
	t.Cleanup(server.Close)

	_, err := queryCommons(context.Background(), CharacterHistoryOptions{
		APIURL: server.URL,
		Client: server.Client(),
	}, []string{horseOracleTitle})
	if err == nil || !strings.Contains(err.Error(), "maxlag") {
		t.Fatalf("queryCommons error = %v, want maxlag", err)
	}
}

func TestCommonsAssetMetadataValidation(t *testing.T) {
	if !validCommonsSHA1("99f4a73ab7e5027f0191ce8093740e7dac8fa722") {
		t.Error("valid Commons SHA-1 was rejected")
	}
	if validCommonsSHA1("../../seed-data") {
		t.Error("path-like Commons SHA-1 was accepted")
	}

	const apiURL = "https://commons.wikimedia.org/w/api.php"
	if !allowedHistoryImageURL(
		apiURL,
		"https://upload.wikimedia.org/wikipedia/commons/5/50/horse.svg",
	) {
		t.Error("Wikimedia upload URL was rejected")
	}
	if allowedHistoryImageURL(apiURL, "https://tracker.example/horse.svg") {
		t.Error("off-host image URL was accepted")
	}
}
