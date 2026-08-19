// Package domain contains provider-neutral estimate contracts.
package domain

import "context"

// EstimateRequest describes the resource and usage to estimate. Provider-specific
// inputs belong in Options so they are not mistaken for cross-cloud concepts.
type EstimateRequest struct {
	Provider string
	Service  string
	Region   string
	Usage    Usage
	Options  map[string]any
}

type Usage struct {
	Hours    float64
	Instance string
}

type LineItem struct {
	Description string
	Quantity    float64
	Unit        string
	Rate        string
	Total       string
}

type EstimateResult struct {
	Provider       string
	Service        string
	Region         string
	Currency       string
	HourlyTotal    string
	DailyTotal     string
	MonthlyTotal   string
	AnnualTotal    string
	CatalogVersion string
	CatalogSource  string
	Breakdown      []LineItem
	Assumptions    []string
	Warnings       []string
}

type Estimator interface {
	Estimate(context.Context, EstimateRequest) (EstimateResult, error)
}
