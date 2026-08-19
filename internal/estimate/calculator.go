// Package estimate calculates transparent estimates from a pricing catalog.
package estimate

import (
	"context"
	"fmt"

	"costguard/internal/catalog"
	"costguard/internal/domain"
)

var ErrInvalidInput = domain.ErrInvalidEstimate

type Catalog interface {
	Lookup(catalog.ProductKey) (catalog.HourlyRate, error)
	Version() string
	Source() string
}

type Calculator struct{ catalog Catalog }

func NewCalculator(c Catalog) *Calculator { return &Calculator{catalog: c} }

func (c *Calculator) Estimate(_ context.Context, request domain.EstimateRequest) (domain.EstimateResult, error) {
	if err := request.Validate(); err != nil {
		return domain.EstimateResult{}, err
	}
	if request.Provider != domain.ProviderAWS || request.Service != "ec2" {
		return domain.EstimateResult{}, fmt.Errorf("%w: only aws ec2 is available in the offline catalog", domain.ErrUnsupportedEstimate)
	}

	rate, err := c.catalog.Lookup(catalog.ProductKey{
		Provider: request.Provider,
		Service:  request.Service,
		Region:   request.Region,
		Instance: request.Usage.Instance,
	})
	if err != nil {
		return domain.EstimateResult{}, fmt.Errorf("lookup hourly rate: %w", err)
	}

	monthly := cost(rate.Price, request.Usage.MicroHours)
	return domain.EstimateResult{
		Provider:       request.Provider,
		Service:        request.Service,
		Region:         request.Region,
		Currency:       rate.Price.Currency,
		HourlyTotal:    rate.Price,
		DailyTotal:     cost(rate.Price, 24*1_000_000),
		MonthlyTotal:   monthly,
		AnnualTotal:    cost(rate.Price, request.Usage.MicroHours*12),
		CatalogVersion: c.catalog.Version(),
		CatalogSource:  c.catalog.Source(),
		Breakdown: []domain.LineItem{{
			Description: "Amazon EC2 " + string(request.Usage.Instance) + " on-demand compute",
			Quantity:    request.Usage,
			Unit:        rate.Unit,
			Rate:        rate.Price,
			Total:       monthly,
		}},
		Assumptions: []string{
			"Linux shared tenancy, on-demand pricing",
			"daily total uses 24 hours",
			"monthly and annual totals use the requested hours value",
			rate.Source,
			"catalog last verified " + rate.LastVerified.Format("2006-01-02"),
		},
		Warnings: []string{
			"Excludes data transfer, EBS, public IPv4, taxes, support, Savings Plans, Reserved Instances, Spot, and operating-system licensing.",
		},
	}, nil
}

func cost(rate domain.Money, microHours int64) domain.Money {
	return domain.Money{MicroUSD: rate.MicroUSD * microHours / 1_000_000, Currency: rate.Currency}
}
