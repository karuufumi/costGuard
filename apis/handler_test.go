package apis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"costguard/internal/catalog"
	"costguard/internal/domain"
	"costguard/internal/estimate"
)

type fakeEstimator struct{}

func (fakeEstimator) Estimate(_ context.Context, request domain.EstimateRequest) (domain.EstimateResult, error) {
	return domain.EstimateResult{
		Provider:       request.Provider,
		Service:        request.Service,
		Region:         request.Region,
		Currency:       "USD",
		MonthlyTotal:   "1.23",
		CatalogVersion: "test",
		Assumptions:    []string{"test catalog"},
	}, nil
}

func TestProviders(t *testing.T) {
	response := request(t, NewHandler(HandlerConfig{}), http.MethodGet, "/v1/providers", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var providers []Provider
	decodeBody(t, response, &providers)
	if len(providers) != 3 {
		t.Fatalf("providers = %d, want 3", len(providers))
	}
}

func TestDocsRoute(t *testing.T) {
	redirect := request(t, NewHandler(HandlerConfig{}), http.MethodGet, "/docs", "")
	if redirect.Code != http.StatusMovedPermanently {
		t.Fatalf("redirect status = %d, want %d", redirect.Code, http.StatusMovedPermanently)
	}

	response := request(t, NewHandler(HandlerConfig{}), http.MethodGet, "/docs/index.html", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "Swagger UI") {
		t.Fatal("docs response does not contain Swagger UI")
	}
}

func TestHomeRoute(t *testing.T) {
	response := request(t, NewHandler(HandlerConfig{}), http.MethodGet, "/", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "costGuard API") {
		t.Fatal("home page does not contain the application title")
	}
}

func TestProviderServices(t *testing.T) {
	response := request(t, NewHandler(HandlerConfig{}), http.MethodGet, "/v1/providers/azure/services", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var services []Service
	decodeBody(t, response, &services)
	if services[0].ID != "virtual-machines" {
		t.Fatalf("first service = %q, want virtual-machines", services[0].ID)
	}
}

func TestUnknownProvider(t *testing.T) {
	response := request(t, NewHandler(HandlerConfig{}), http.MethodGet, "/v1/providers/digitalocean/services", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}

	var problem Problem
	decodeBody(t, response, &problem)
	if problem.Type != "/problems/provider-not-found" {
		t.Fatalf("problem type = %q", problem.Type)
	}
}

func TestConfigLifecycle(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	response := request(t, handler, http.MethodGet, "/v1/config", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", response.Code, http.StatusOK)
	}

	response = request(t, handler, http.MethodPatch, "/v1/config", `{"currency":"EUR"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want %d", response.Code, http.StatusOK)
	}

	var config Config
	decodeBody(t, response, &config)
	if config.Currency != "EUR" {
		t.Fatalf("currency = %q, want EUR", config.Currency)
	}

	response = request(t, handler, http.MethodDelete, "/v1/config", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response = request(t, handler, http.MethodGet, "/v1/config", "")
	decodeBody(t, response, &config)
	if config.Currency != "USD" {
		t.Fatalf("reset currency = %q, want USD", config.Currency)
	}
}

func TestPutConfigRejectsCredentialsAndUnknownFields(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	response := request(t, handler, http.MethodPut, "/v1/config", `{"default_provider":"aws","default_region":"us-east-1","currency":"USD","monthly_hours":730,"pricing_mode":"offline","secret_key":"do-not-accept"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestEstimateWithoutDomainReturnsServiceUnavailable(t *testing.T) {
	handler := NewHandler(HandlerConfig{})

	response := request(t, handler, http.MethodPost, "/v1/estimates", `{"provider":"gcp","service":"compute-engine","region":"asia-southeast1","usage":{"hours":730}}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestEstimateDelegatesToDomain(t *testing.T) {
	handler := NewHandler(HandlerConfig{Estimator: fakeEstimator{}})

	response := request(t, handler, http.MethodPost, "/v1/estimates", `{"provider":"aws","service":"ec2","region":"ap-southeast-1","usage":{"hours":730}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var result EstimateResult
	decodeBody(t, response, &result)
	if result.MonthlyTotal != "1.23" {
		t.Fatalf("monthly total = %q, want 1.23", result.MonthlyTotal)
	}
}

func TestEstimateCalculatesEmbeddedEC2Rate(t *testing.T) {
	handler := NewHandler(HandlerConfig{Estimator: estimate.NewCalculator(catalog.NewEmbedded())})

	response := request(t, handler, http.MethodPost, "/v1/estimates", `{"provider":"aws","service":"ec2","region":"us-east-1","usage":{"instance":"t3.micro","hours":730}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var result EstimateResult
	decodeBody(t, response, &result)
	if result.MonthlyTotal != "7.59" {
		t.Fatalf("monthly total = %q, want 7.59", result.MonthlyTotal)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected exclusions warning")
	}
}

func TestEstimateRejectsInvalidInput(t *testing.T) {
	handler := NewHandler(HandlerConfig{Estimator: fakeEstimator{}})

	response := request(t, handler, http.MethodPost, "/v1/estimates", `{"provider":"aws","service":"ec2","region":"ap-southeast-1","usage":{"hours":-1}}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeBody(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
