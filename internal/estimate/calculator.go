// Package estimate calculates transparent estimates from a pricing catalog.
package estimate

import (
	"context"
	"errors"
	"fmt"
	"math"

	"costguard/internal/catalog"
	"costguard/internal/domain"
)

var ErrInvalidInput = errors.New("invalid estimate input")

type Catalog interface {
	Lookup(catalog.ProductKey) (catalog.HourlyRate, error)
	Version() string
	Source() string
}

type Calculator struct{ catalog Catalog }

func NewCalculator(c Catalog) *Calculator { return &Calculator{catalog: c} }

func (c *Calculator) Estimate(_ context.Context, request domain.EstimateRequest) (domain.EstimateResult, error) {
	if request.Provider != "aws" || request.Service != "ec2" {
		return domain.EstimateResult{}, fmt.Errorf("%w: only aws ec2 is available in the offline catalog", ErrInvalidInput)
	}
	if request.Region == "" || request.Usage.Instance == "" {
		return domain.EstimateResult{}, fmt.Errorf("%w: region and instance are required", ErrInvalidInput)
	}
	if math.IsNaN(request.Usage.Hours) || math.IsInf(request.Usage.Hours, 0) || request.Usage.Hours < 0 {
		return domain.EstimateResult{}, fmt.Errorf("%w: hours must be a finite value greater than or equal to zero", ErrInvalidInput)
	}

	rate, err := c.catalog.Lookup(catalog.ProductKey{
		Provider: request.Provider, Service: request.Service, Region: request.Region, Instance: request.Usage.Instance,
	})
	if err != nil {
		return domain.EstimateResult{}, fmt.Errorf("%w: %w", err, catalog.ErrRateNotFound)
	}

	return domain.EstimateResult{
		Provider: request.Provider, Service: request.Service, Region: request.Region,
		Currency: "USD", HourlyTotal: money(rate.MicroUSD),
		DailyTotal:     money(costMicroUSD(rate.MicroUSD, 24)),
		MonthlyTotal:   money(costMicroUSD(rate.MicroUSD, request.Usage.Hours)),
		AnnualTotal:    money(costMicroUSD(rate.MicroUSD, request.Usage.Hours*12)),
		CatalogVersion: c.catalog.Version(), CatalogSource: c.catalog.Source(),
		Breakdown: []domain.LineItem{{
			Description: "Amazon EC2 " + request.Usage.Instance + " on-demand compute",
			Quantity:    request.Usage.Hours, Unit: "instance-hours", Rate: rateUSD(rate.MicroUSD),
			Total: money(costMicroUSD(rate.MicroUSD, request.Usage.Hours)),
		}},
		Assumptions: []string{
			"Linux shared tenancy, on-demand pricing", "daily total uses 24 hours",
			"monthly and annual totals use the requested hours value", rate.Source,
		},
		Warnings: []string{
			"Excludes data transfer, EBS, public IPv4, taxes, support, Savings Plans, Reserved Instances, Spot, and operating-system licensing.",
		},
	}, nil
}

// costMicroUSD rounds user-supplied hours to micro-hours, then rounds the
// resulting amount half-up to cents. Displayed currency never uses float64.
func costMicroUSD(rateMicroUSD int64, hours float64) int64 {
	microHours := int64(math.Round(hours * 1_000_000))
	microUSD := rateMicroUSD * microHours / 1_000_000
	return microUSD
}

func money(microUSD int64) string {
	cents := (microUSD + 5_000) / 10_000
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func rateUSD(microUSD int64) string {
	return fmt.Sprintf("%d.%06d", microUSD/1_000_000, microUSD%1_000_000)
}
