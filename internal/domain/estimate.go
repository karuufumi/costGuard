// Package domain contains provider-neutral estimate contracts and value types.
package domain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
)

const microHoursPerHour int64 = 1_000_000

const maxUsageHours = 24 * 366 * 10

var (
	ErrInvalidEstimate     = errors.New("invalid estimate input")
	ErrUnsupportedEstimate = errors.New("unsupported estimate input")
)

type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderAzure Provider = "azure"
	ProviderGCP   Provider = "gcp"
)

type Service string
type Region string
type InstanceType string
type Currency string

const CurrencyUSD Currency = "USD"

// Usage stores billed time as integer micro-hours. Floating-point input is
// accepted only by NewUsage, at the presentation boundary.
type Usage struct {
	Instance   InstanceType
	MicroHours int64
}

func NewUsage(instance string, hours float64) (Usage, error) {
	instance = strings.TrimSpace(instance)
	if instance == "" {
		return Usage{}, invalid("usage.instance", "is required")
	}
	if math.IsNaN(hours) || math.IsInf(hours, 0) || hours < 0 || hours > maxUsageHours {
		return Usage{}, invalid("usage.hours", fmt.Sprintf("must be finite and between 0 and %d", maxUsageHours))
	}
	return Usage{Instance: InstanceType(instance), MicroHours: int64(math.Round(hours * float64(microHoursPerHour)))}, nil
}

func (u Usage) Hours() float64 {
	return float64(u.MicroHours) / float64(microHoursPerHour)
}

func (u Usage) Validate() error {
	if strings.TrimSpace(string(u.Instance)) == "" {
		return invalid("usage.instance", "is required")
	}
	if u.MicroHours < 0 || u.MicroHours > maxUsageHours*microHoursPerHour {
		return invalid("usage.hours", fmt.Sprintf("must be between 0 and %d", maxUsageHours))
	}
	return nil
}

// Money stores USD at micro-dollar precision. It is intentionally a value
// object so calculation code never uses binary floating point for currency.
type Money struct {
	MicroUSD int64
	Currency Currency
}

func NewUSDMoney(microUSD int64) Money {
	return Money{MicroUSD: microUSD, Currency: CurrencyUSD}
}

// String rounds half-up to cents for presentation.
func (m Money) String() string {
	sign := ""
	amount := m.MicroUSD
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	cents := (amount + 5_000) / 10_000
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// PreciseString is suitable for displaying a catalog rate at micro-dollar precision.
func (m Money) PreciseString() string {
	sign := ""
	amount := m.MicroUSD
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	return fmt.Sprintf("%s%d.%06d", sign, amount/1_000_000, amount%1_000_000)
}

type EstimateRequest struct {
	Provider Provider
	Service  Service
	Region   Region
	Usage    Usage
}

func (r EstimateRequest) Validate() error {
	if r.Provider != ProviderAWS && r.Provider != ProviderAzure && r.Provider != ProviderGCP {
		return invalid("provider", "must be one of: aws, azure, gcp")
	}
	if strings.TrimSpace(string(r.Service)) == "" {
		return invalid("service", "is required")
	}
	if strings.TrimSpace(string(r.Region)) == "" {
		return invalid("region", "is required")
	}
	return r.Usage.Validate()
}

type LineItem struct {
	Description string
	Quantity    Usage
	Unit        string
	Rate        Money
	Total       Money
}

type EstimateResult struct {
	Provider       Provider
	Service        Service
	Region         Region
	Currency       Currency
	HourlyTotal    Money
	DailyTotal     Money
	MonthlyTotal   Money
	AnnualTotal    Money
	CatalogVersion string
	CatalogSource  string
	Breakdown      []LineItem
	Assumptions    []string
	Warnings       []string
}

type Estimator interface {
	Estimate(context.Context, EstimateRequest) (EstimateResult, error)
}

func invalid(field, message string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidEstimate, field, message)
}
