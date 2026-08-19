package apis_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"costguard/apis"
	"costguard/internal/catalog"
	"costguard/internal/estimate"
)

func TestAPIEndToEnd(t *testing.T) {
	server := httptest.NewServer(apis.NewHandler(apis.HandlerConfig{
		Estimator: estimate.NewCalculator(catalog.NewEmbedded()),
	}))
	t.Cleanup(server.Close)

	t.Run("discovery and documentation", func(t *testing.T) {
		response := get(t, server.URL+"/")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("home status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		assertCommonHeaders(t, response)
		if contentType := response.Header.Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Fatalf("home Content-Type = %q", contentType)
		}

		response = get(t, server.URL+"/healthz")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("health status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		assertCommonHeaders(t, response)
		var health map[string]string
		decodeJSON(t, response, &health)
		if health["status"] != "ok" {
			t.Fatalf("health status body = %q, want ok", health["status"])
		}

		response = get(t, server.URL+"/v1/providers")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("providers status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var providers []apis.Provider
		decodeJSON(t, response, &providers)
		if len(providers) != 3 || providers[0].ID != "aws" || providers[1].ID != "azure" || providers[2].ID != "gcp" {
			t.Fatalf("providers = %#v, want aws, azure, gcp", providers)
		}

		response = get(t, server.URL+"/v1/providers/aws/regions")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("regions status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var regions []apis.Region
		decodeJSON(t, response, &regions)
		if len(regions) == 0 || regions[0].ID == "" {
			t.Fatalf("regions = %#v, want named regions", regions)
		}

		response = get(t, server.URL+"/v1/providers/aws/services")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("services status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var services []apis.Service
		decodeJSON(t, response, &services)
		if len(services) == 0 || services[0].ID != "ec2" {
			t.Fatalf("services = %#v, want EC2 service", services)
		}

		response = get(t, server.URL+"/v1/catalog")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("catalog status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var catalogStatus apis.CatalogResponse
		decodeJSON(t, response, &catalogStatus)
		if catalogStatus.Version == "" || catalogStatus.Source != "embedded" || catalogStatus.Status != "available" {
			t.Fatalf("catalog = %#v", catalogStatus)
		}

		response = get(t, server.URL+"/v1/account")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("account status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var account apis.AccountResponse
		decodeJSON(t, response, &account)
		if account.Configured || account.CredentialSource == "" {
			t.Fatalf("account = %#v", account)
		}

		response = get(t, server.URL+"/docs/doc.json")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("OpenAPI status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var spec map[string]any
		decodeJSON(t, response, &spec)
		if spec["swagger"] != "2.0" {
			t.Fatalf("OpenAPI version = %v, want 2.0", spec["swagger"])
		}
	})

	t.Run("configuration lifecycle", func(t *testing.T) {
		response := get(t, server.URL+"/v1/config")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET config status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var current apis.Config
		decodeJSON(t, response, &current)
		if current.Currency != "USD" || current.PricingMode != "offline" {
			t.Fatalf("default config = %#v, want offline USD defaults", current)
		}

		response = request(t, http.MethodPut, server.URL+"/v1/config", `{"default_provider":"aws","default_region":"us-east-1","currency":"USD","monthly_hours":720,"pricing_mode":"offline"}`, "application/json")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("PUT config status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		decodeJSON(t, response, &current)
		if current.MonthlyHours != 720 || current.DefaultRegion != "us-east-1" {
			t.Fatalf("replaced config = %#v", current)
		}

		response = request(t, http.MethodPatch, server.URL+"/v1/config", `{"currency":"EUR"}`, "application/json")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("PATCH config status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		decodeJSON(t, response, &current)
		if current.Currency != "EUR" || current.MonthlyHours != 720 {
			t.Fatalf("patched config = %#v", current)
		}

		response = request(t, http.MethodDelete, server.URL+"/v1/config", "", "")
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("DELETE config status = %d, want %d", response.StatusCode, http.StatusNoContent)
		}

		response = get(t, server.URL+"/v1/config")
		defer response.Body.Close()
		decodeJSON(t, response, &current)
		if current.Currency != "USD" || current.MonthlyHours != 730 {
			t.Fatalf("reset config = %#v, want default config", current)
		}
	})

	t.Run("estimate success and client errors", func(t *testing.T) {
		response := request(t, http.MethodPost, server.URL+"/v1/estimates", `{"provider":"aws","service":"ec2","region":"us-east-1","usage":{"instance":"t3.micro","hours":730}}`, "application/json")
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("estimate status = %d, want %d", response.StatusCode, http.StatusOK)
		}
		var result apis.EstimateResult
		decodeJSON(t, response, &result)
		if result.MonthlyTotal != "7.59" || result.HourlyTotal != "0.01" || result.CatalogVersion == "" || len(result.Warnings) == 0 {
			t.Fatalf("estimate result = %#v", result)
		}

		response = request(t, http.MethodPost, server.URL+"/v1/estimates", `{"provider":"aws","service":"ec2","region":"us-east-1","usage":{"instance":"t3.micro","hours":-1}}`, "application/json")
		defer response.Body.Close()
		assertProblem(t, response, http.StatusBadRequest, "/problems/invalid-estimate-input")

		response = request(t, http.MethodPost, server.URL+"/v1/estimates", `{"provider":"aws","service":"ec2","region":"us-east-1","usage":{"instance":"t3.micro","hours":1},"unknown":true}`, "application/json")
		defer response.Body.Close()
		assertProblem(t, response, http.StatusBadRequest, "/problems/invalid-request")

		response = request(t, http.MethodPost, server.URL+"/v1/estimates", `{"provider":"aws","service":"ec2","region":"ap-southeast-1","usage":{"instance":"t3.micro","hours":1}}`, "application/json")
		defer response.Body.Close()
		assertProblem(t, response, http.StatusServiceUnavailable, "/problems/estimate-unavailable")

		response = request(t, http.MethodPost, server.URL+"/v1/estimates", `{}`, "text/plain")
		defer response.Body.Close()
		assertProblem(t, response, http.StatusUnsupportedMediaType, "/problems/unsupported-media-type")

		response = request(t, http.MethodPost, server.URL+"/v1/estimates", `{} {}`, "application/json")
		defer response.Body.Close()
		assertProblem(t, response, http.StatusBadRequest, "/problems/invalid-request")
	})

	t.Run("unknown routes return a problem response", func(t *testing.T) {
		response := get(t, server.URL+"/v1/not-a-route")
		defer response.Body.Close()
		assertProblem(t, response, http.StatusNotFound, "/problems/not-found")
	})
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return response
}

func request(t *testing.T, method, url, body, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("create %s request: %v", method, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return response
}

func assertCommonHeaders(t *testing.T, response *http.Response) {
	t.Helper()
	if response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", response.Header.Get("X-Content-Type-Options"))
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header.Get("Cache-Control"))
	}
	if response.Header.Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is missing")
	}
}

func assertProblem(t *testing.T, response *http.Response, status int, problemType string) {
	t.Helper()
	if response.StatusCode != status {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, status, body)
	}
	assertCommonHeaders(t, response)
	var problem apis.Problem
	decodeJSON(t, response, &problem)
	if problem.Type != problemType || problem.Status != status {
		t.Fatalf("problem = %#v, want type %q and status %d", problem, problemType, status)
	}
}

func decodeJSON(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}
}
