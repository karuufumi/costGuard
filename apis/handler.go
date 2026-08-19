package apis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	_ "costguard/docs"
	"costguard/internal/domain"
	"costguard/internal/estimate"

	httpSwagger "github.com/swaggo/http-swagger"
)

const maxJSONBody = 1 << 20

type Provider struct {
	ID   string `json:"id" example:"aws"`
	Name string `json:"name" example:"Amazon Web Services"`
}

type Service struct {
	ID   string `json:"id" example:"ec2"`
	Name string `json:"name" example:"Amazon EC2"`
}

type Region struct {
	ID   string `json:"id" example:"ap-southeast-1"`
	Name string `json:"name" example:"Asia Pacific (Singapore)"`
}

type Usage struct {
	Hours    float64 `json:"hours" example:"730"`
	Instance string  `json:"instance" example:"t3.micro"`
}

type EstimateRequest struct {
	Provider        string         `json:"provider" example:"aws"`
	Service         string         `json:"service" example:"ec2"`
	Region          string         `json:"region" example:"ap-southeast-1"`
	Usage           Usage          `json:"usage"`
	ProviderOptions map[string]any `json:"provider_options,omitempty"`
}

type EstimateResult struct {
	Provider       string     `json:"provider"`
	Service        string     `json:"service"`
	Region         string     `json:"region"`
	Currency       string     `json:"currency"`
	HourlyTotal    string     `json:"hourly_total"`
	DailyTotal     string     `json:"daily_total"`
	MonthlyTotal   string     `json:"monthly_total"`
	AnnualTotal    string     `json:"annual_total"`
	CatalogVersion string     `json:"catalog_version"`
	CatalogSource  string     `json:"catalog_source"`
	Breakdown      []LineItem `json:"breakdown"`
	Assumptions    []string   `json:"assumptions"`
	Warnings       []string   `json:"warnings"`
}

type LineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	Rate        string  `json:"rate"`
	Total       string  `json:"total"`
}

type Config struct {
	DefaultProvider string  `json:"default_provider" example:"aws"`
	DefaultRegion   string  `json:"default_region" example:"ap-southeast-1"`
	Currency        string  `json:"currency" example:"USD"`
	MonthlyHours    float64 `json:"monthly_hours" example:"730"`
	PricingMode     string  `json:"pricing_mode" example:"offline"`
}

type ConfigPatch struct {
	DefaultProvider *string  `json:"default_provider,omitempty"`
	DefaultRegion   *string  `json:"default_region,omitempty"`
	Currency        *string  `json:"currency,omitempty"`
	MonthlyHours    *float64 `json:"monthly_hours,omitempty"`
	PricingMode     *string  `json:"pricing_mode,omitempty"`
}

type AccountResponse struct {
	Configured       bool   `json:"configured"`
	Provider         string `json:"provider,omitempty"`
	AccountReference string `json:"account_reference,omitempty"`
	CredentialSource string `json:"credential_source"`
}

type CatalogResponse struct {
	Version string `json:"version" example:"embedded-development"`
	Source  string `json:"source" example:"embedded"`
	Status  string `json:"status" example:"not_ready"`
}

type Problem struct {
	Type      string `json:"type" example:"/problems/invalid-request"`
	Title     string `json:"title" example:"Invalid request"`
	Status    int    `json:"status" example:"400"`
	Detail    string `json:"detail" example:"hours must be greater than or equal to zero"`
	Instance  string `json:"instance,omitempty" example:"/v1/estimates"`
	RequestID string `json:"request_id,omitempty" example:"a1b2c3d4"`
}

type Estimator interface {
	Estimate(context.Context, domain.EstimateRequest) (domain.EstimateResult, error)
}

type ConfigStore interface {
	Get(context.Context) (Config, error)
	Replace(context.Context, Config) error
	Update(context.Context, ConfigPatch) (Config, error)
	Reset(context.Context) error
}

type HandlerConfig struct {
	Estimator   Estimator
	ConfigStore ConfigStore
}

type Handler struct {
	estimator   Estimator
	configStore ConfigStore
}

func NewHandler(config HandlerConfig) http.Handler {
	handler := &Handler{
		estimator:   config.Estimator,
		configStore: config.ConfigStore,
	}
	if handler.configStore == nil {
		handler.configStore = NewMemoryConfigStore()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /", handler.home)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
	mux.Handle("GET /docs/", httpSwagger.Handler(httpSwagger.URL("/docs/doc.json")))
	mux.HandleFunc("GET /v1/providers", handler.providers)
	mux.HandleFunc("GET /v1/providers/{provider}/services", handler.providerServices)
	mux.HandleFunc("GET /v1/providers/{provider}/regions", handler.providerRegions)
	mux.HandleFunc("GET /v1/catalog", handler.catalog)
	mux.HandleFunc("GET /v1/config", handler.getConfig)
	mux.HandleFunc("PUT /v1/config", handler.putConfig)
	mux.HandleFunc("PATCH /v1/config", handler.patchConfig)
	mux.HandleFunc("DELETE /v1/config", handler.deleteConfig)
	mux.HandleFunc("GET /v1/account", handler.account)
	mux.HandleFunc("POST /v1/estimates", handler.estimate)
	mux.HandleFunc("/", handler.notFound)

	return withRequestContext(withSecurityHeaders(mux))
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.notFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>costGuard API</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, sans-serif; }
    body { margin: 0; background: #f4f6f8; color: #17202a; }
    main { max-width: 760px; margin: 10vh auto; padding: 2rem; }
    .card { padding: 2rem; border: 1px solid #d9dee5; border-radius: 14px; background: white; box-shadow: 0 12px 30px #17202a18; }
    h1 { margin-top: 0; letter-spacing: -0.03em; }
    p { line-height: 1.6; color: #52606d; }
    nav { display: flex; flex-wrap: wrap; gap: 0.75rem; margin-top: 1.5rem; }
    a { padding: 0.7rem 1rem; border-radius: 8px; background: #1769e0; color: white; text-decoration: none; }
    a:hover { background: #0f4fae; }
    code { padding: 0.15rem 0.35rem; border-radius: 4px; background: #eef1f4; }
    @media (prefers-color-scheme: dark) {
      body { background: #111820; color: #edf2f7; }
      .card { border-color: #344454; background: #1b2633; }
      p { color: #b8c4d0; }
      code { background: #2b3948; }
    }
  </style>
</head>
<body>
  <main>
    <section class="card">
      <h1>costGuard API</h1>
      <p>A small API for estimating cloud costs across AWS, Azure, and GCP.</p>
      <p>The presentation layer is running. Pricing calculations are still being connected.</p>
      <nav aria-label="API resources">
        <a href="/docs">OpenAPI docs</a>
        <a href="/healthz">Health check</a>
        <a href="/v1/providers">Providers</a>
      </nav>
    </section>
  </main>
</body>
</html>`))
}

// @Summary Check API health
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Router /healthz [get]
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// @Summary List supported cloud providers
// @Tags discovery
// @Produce json
// @Success 200 {array} Provider
// @Router /v1/providers [get]
func (h *Handler) providers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []Provider{
		{ID: "aws", Name: "Amazon Web Services"},
		{ID: "azure", Name: "Microsoft Azure"},
		{ID: "gcp", Name: "Google Cloud Platform"},
	})
}

// @Summary List provider services
// @Tags discovery
// @Produce json
// @Param provider path string true "Cloud provider"
// @Success 200 {array} Service
// @Failure 404 {object} Problem
// @Router /v1/providers/{provider}/services [get]
func (h *Handler) providerServices(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	services, ok := servicesFor(provider)
	if !ok {
		writeProblem(w, http.StatusNotFound, "provider-not-found", "Provider not found", "provider is not supported", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

// @Summary List provider regions
// @Tags discovery
// @Produce json
// @Param provider path string true "Cloud provider"
// @Success 200 {array} Region
// @Failure 404 {object} Problem
// @Router /v1/providers/{provider}/regions [get]
func (h *Handler) providerRegions(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	regions, ok := regionsFor(provider)
	if !ok {
		writeProblem(w, http.StatusNotFound, "provider-not-found", "Provider not found", "provider is not supported", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, regions)
}

// @Summary Get pricing catalog status
// @Tags catalog
// @Produce json
// @Success 200 {object} CatalogResponse
// @Router /v1/catalog [get]
func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, CatalogResponse{
		Version: "2026-08-18.1",
		Source:  "embedded",
		Status:  "available",
	})
}

// @Summary Get saved configuration
// @Tags configuration
// @Produce json
// @Success 200 {object} Config
// @Router /v1/config [get]
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.configStore.Get(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "config-read-failed", "Configuration unavailable", "saved configuration could not be read", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

// @Summary Replace saved configuration
// @Tags configuration
// @Accept json
// @Produce json
// @Param config body Config true "Complete preferences"
// @Success 200 {object} Config
// @Failure 400 {object} Problem
// @Router /v1/config [put]
func (h *Handler) putConfig(w http.ResponseWriter, r *http.Request) {
	var config Config
	if !decodeJSON(w, r, &config) {
		return
	}
	if err := validateConfig(config); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid-config", "Invalid configuration", err.Error(), r.URL.Path)
		return
	}
	if err := h.configStore.Replace(r.Context(), config); err != nil {
		writeProblem(w, http.StatusInternalServerError, "config-write-failed", "Configuration unavailable", "configuration could not be saved", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

// @Summary Update saved configuration
// @Tags configuration
// @Accept json
// @Produce json
// @Param config body ConfigPatch true "Partial preferences"
// @Success 200 {object} Config
// @Failure 400 {object} Problem
// @Router /v1/config [patch]
func (h *Handler) patchConfig(w http.ResponseWriter, r *http.Request) {
	var patch ConfigPatch
	if !decodeJSON(w, r, &patch) {
		return
	}
	config, err := h.configStore.Update(r.Context(), patch)
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			writeProblem(w, http.StatusBadRequest, "invalid-config", "Invalid configuration", err.Error(), r.URL.Path)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "config-write-failed", "Configuration unavailable", "configuration could not be saved", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

// @Summary Reset saved configuration
// @Tags configuration
// @Success 204
// @Failure 500 {object} Problem
// @Router /v1/config [delete]
func (h *Handler) deleteConfig(w http.ResponseWriter, r *http.Request) {
	if err := h.configStore.Reset(r.Context()); err != nil {
		writeProblem(w, http.StatusInternalServerError, "config-reset-failed", "Configuration unavailable", "configuration could not be reset", r.URL.Path)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Get safe account metadata
// @Description Never returns cloud credentials or tokens.
// @Tags account
// @Produce json
// @Success 200 {object} AccountResponse
// @Router /v1/account [get]
func (h *Handler) account(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, AccountResponse{
		Configured:       false,
		CredentialSource: "external-provider-chain",
	})
}

// @Summary Create a cost estimate
// @Description Calculates an estimate using the selected provider and pricing catalog.
// @Tags estimates
// @Accept json
// @Produce json
// @Param request body EstimateRequest true "Estimate input"
// @Success 200 {object} EstimateResult
// @Failure 400 {object} Problem
// @Failure 503 {object} Problem
// @Router /v1/estimates [post]
func (h *Handler) estimate(w http.ResponseWriter, r *http.Request) {
	var request EstimateRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	domainRequest, err := toDomainRequest(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid-estimate-input", "Invalid estimate input", err.Error(), r.URL.Path)
		return
	}
	if h.estimator == nil {
		writeProblem(w, http.StatusServiceUnavailable, "pricing-catalog-unavailable", "Pricing catalog unavailable", "estimate calculation is not configured yet", r.URL.Path)
		return
	}

	result, err := h.estimator.Estimate(r.Context(), domainRequest)
	if err != nil {
		if errors.Is(err, estimate.ErrInvalidInput) {
			writeProblem(w, http.StatusBadRequest, "unsupported-estimate-input", "Unsupported estimate input", err.Error(), r.URL.Path)
			return
		}
		writeProblem(w, http.StatusServiceUnavailable, "estimate-unavailable", "Estimate unavailable", "the estimate could not be calculated", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, fromDomainResult(result))
}

func toDomainRequest(request EstimateRequest) (domain.EstimateRequest, error) {
	if len(request.ProviderOptions) != 0 {
		return domain.EstimateRequest{}, errors.New("provider_options are not supported for the current EC2 estimate")
	}
	usage, err := domain.NewUsage(request.Usage.Instance, request.Usage.Hours)
	if err != nil {
		return domain.EstimateRequest{}, err
	}
	domainRequest := domain.EstimateRequest{
		Provider: domain.Provider(request.Provider),
		Service:  domain.Service(request.Service),
		Region:   domain.Region(request.Region),
		Usage:    usage,
	}
	return domainRequest, domainRequest.Validate()
}

func fromDomainResult(result domain.EstimateResult) EstimateResult {
	breakdown := make([]LineItem, len(result.Breakdown))
	for i, item := range result.Breakdown {
		breakdown[i] = LineItem{
			Description: item.Description,
			Quantity:    item.Quantity.Hours(),
			Unit:        item.Unit,
			Rate:        item.Rate.PreciseString(),
			Total:       item.Total.String(),
		}
	}
	return EstimateResult{Provider: string(result.Provider), Service: string(result.Service), Region: string(result.Region), Currency: string(result.Currency),
		HourlyTotal: result.HourlyTotal.String(), DailyTotal: result.DailyTotal.String(), MonthlyTotal: result.MonthlyTotal.String(), AnnualTotal: result.AnnualTotal.String(),
		CatalogVersion: result.CatalogVersion, CatalogSource: result.CatalogSource, Breakdown: breakdown,
		Assumptions: result.Assumptions, Warnings: result.Warnings}
}

var ErrInvalidConfig = errors.New("configuration is invalid")

func validateConfig(config Config) error {
	if !isProvider(config.DefaultProvider) {
		return errors.Join(ErrInvalidConfig, errors.New("default_provider must be one of: aws, azure, gcp"))
	}
	if strings.TrimSpace(config.DefaultRegion) == "" {
		return errors.Join(ErrInvalidConfig, errors.New("default_region is required"))
	}
	if len(config.Currency) != 3 {
		return errors.Join(ErrInvalidConfig, errors.New("currency must be a 3-letter code"))
	}
	if config.MonthlyHours <= 0 || config.MonthlyHours > 8784 {
		return errors.Join(ErrInvalidConfig, errors.New("monthly_hours must be greater than zero and no more than 8784"))
	}
	if config.PricingMode != "offline" && config.PricingMode != "live" {
		return errors.Join(ErrInvalidConfig, errors.New("pricing_mode must be offline or live"))
	}
	return nil
}

func isProvider(provider string) bool {
	return provider == "aws" || provider == "azure" || provider == "gcp"
}

func servicesFor(provider string) ([]Service, bool) {
	switch provider {
	case "aws":
		return []Service{{ID: "ec2", Name: "Amazon EC2"}, {ID: "s3", Name: "Amazon S3"}, {ID: "lambda", Name: "AWS Lambda"}}, true
	case "azure":
		return []Service{{ID: "virtual-machines", Name: "Azure Virtual Machines"}, {ID: "blob-storage", Name: "Azure Blob Storage"}, {ID: "functions", Name: "Azure Functions"}}, true
	case "gcp":
		return []Service{{ID: "compute-engine", Name: "Google Compute Engine"}, {ID: "cloud-storage", Name: "Google Cloud Storage"}, {ID: "cloud-functions", Name: "Google Cloud Functions"}}, true
	default:
		return nil, false
	}
}

func regionsFor(provider string) ([]Region, bool) {
	switch provider {
	case "aws":
		return []Region{{ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)"}, {ID: "us-east-1", Name: "US East (N. Virginia)"}}, true
	case "azure":
		return []Region{{ID: "southeastasia", Name: "Southeast Asia"}, {ID: "eastus", Name: "East US"}}, true
	case "gcp":
		return []Region{{ID: "asia-southeast1", Name: "Singapore"}, {ID: "us-central1", Name: "Iowa"}}, true
	default:
		return nil, false
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeProblem(w, http.StatusUnsupportedMediaType, "unsupported-media-type", "Unsupported media type", "Content-Type must be application/json", r.URL.Path)
		return false
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid-request", "Invalid request", "request body must be valid JSON", r.URL.Path)
		return false
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeProblem(w, http.StatusBadRequest, "invalid-request", "Invalid request", "request body must contain exactly one JSON value", r.URL.Path)
		return false
	}
	return true
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, http.StatusNotFound, "not-found", "Route not found", "the requested route does not exist", r.URL.Path)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, problemType, title, detail, instance string) {
	writeJSON(w, status, Problem{
		Type:     "/problems/" + problemType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	})
}

type contextKey struct{}

func withRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := make([]byte, 8)
		if _, err := rand.Read(id); err != nil {
			id = []byte("fallback")
		}
		requestID := hex.EncodeToString(id)
		ctx := context.WithValue(r.Context(), contextKey{}, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type MemoryConfigStore struct {
	mu     sync.RWMutex
	config Config
}

func NewMemoryConfigStore() *MemoryConfigStore {
	return &MemoryConfigStore{config: defaultConfig()}
}

func (s *MemoryConfigStore) Get(context.Context) (Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config, nil
}

func (s *MemoryConfigStore) Replace(_ context.Context, config Config) error {
	if err := validateConfig(config); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	return nil
}

func (s *MemoryConfigStore) Update(_ context.Context, patch ConfigPatch) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config := s.config
	if patch.DefaultProvider != nil {
		config.DefaultProvider = *patch.DefaultProvider
	}
	if patch.DefaultRegion != nil {
		config.DefaultRegion = *patch.DefaultRegion
	}
	if patch.Currency != nil {
		config.Currency = *patch.Currency
	}
	if patch.MonthlyHours != nil {
		config.MonthlyHours = *patch.MonthlyHours
	}
	if patch.PricingMode != nil {
		config.PricingMode = *patch.PricingMode
	}
	if err := validateConfig(config); err != nil {
		return Config{}, err
	}
	s.config = config
	return config, nil
}

func (s *MemoryConfigStore) Reset(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = defaultConfig()
	return nil
}

func defaultConfig() Config {
	return Config{
		DefaultProvider: "aws",
		DefaultRegion:   "ap-southeast-1",
		Currency:        "USD",
		MonthlyHours:    730,
		PricingMode:     "offline",
	}
}
