// Package catalog provides deterministic pricing data sources.
package catalog

import "errors"

var ErrRateNotFound = errors.New("pricing rate not found")

type ProductKey struct {
	Provider string
	Service  string
	Region   string
	Instance string
}

// HourlyRate uses micro-USD, avoiding binary floating-point currency values.
type HourlyRate struct {
	MicroUSD int64
	Source   string
}

type Embedded struct {
	rates map[ProductKey]HourlyRate
}

func NewEmbedded() *Embedded {
	return &Embedded{rates: map[ProductKey]HourlyRate{
		{Provider: "aws", Service: "ec2", Region: "us-east-1", Instance: "t3.micro"}: {
			MicroUSD: 10400,
			Source:   "AWS EC2 On-Demand Linux shared-tenancy manual catalog snapshot",
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
