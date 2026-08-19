// Package catalog provides deterministic pricing data sources.
package catalog

import (
	"errors"
	"time"

	"costguard/internal/domain"
)

var ErrRateNotFound = errors.New("pricing rate not found")

type ProductKey struct {
	Provider domain.Provider
	Service  domain.Service
	Region   domain.Region
	Instance domain.InstanceType
}

type HourlyRate struct {
	Price        domain.Money
	Unit         string
	Source       string
	LastVerified time.Time
}

type Embedded struct {
	rates map[ProductKey]HourlyRate
}

func NewEmbedded() *Embedded {
	return &Embedded{rates: map[ProductKey]HourlyRate{
		{Provider: domain.ProviderAWS, Service: "ec2", Region: "us-east-1", Instance: "t3.micro"}: {
			Price:        domain.NewUSDMoney(10_400),
			Unit:         "instance-hours",
			Source:       "AWS EC2 On-Demand Linux shared-tenancy manual catalog snapshot",
			LastVerified: time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC),
		},
	}}
}

func (e *Embedded) Lookup(key ProductKey) (HourlyRate, error) {
	rate, ok := e.rates[key]
	if !ok {
		return HourlyRate{}, ErrRateNotFound
	}
	return rate, nil
}

func (e *Embedded) Version() string { return "2026-08-18.1" }
func (e *Embedded) Source() string  { return "embedded" }
