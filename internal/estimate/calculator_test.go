package estimate

import (
	"context"
	"errors"
	"math"
	"testing"

	"costguard/internal/catalog"
	"costguard/internal/domain"
)

func TestEC2MonthlyEstimate(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	result, err := calculator.Estimate(context.Background(), request(t, 730, "us-east-1"))
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}
	if got := result.MonthlyTotal.String(); got != "7.59" {
		t.Fatalf("monthly total = %s, want 7.59", got)
	}
	if got := result.Breakdown[0].Rate.PreciseString(); got != "0.010400" {
		t.Fatalf("rate = %s, want 0.010400", got)
	}
	if got := result.DailyTotal.String(); got != "0.25" {
		t.Fatalf("daily total = %s, want 0.25", got)
	}
	if result.Breakdown[0].Quantity.MicroHours != 730_000_000 {
		t.Fatalf("usage = %d micro-hours", result.Breakdown[0].Quantity.MicroHours)
	}
}

func TestEC2AllowsZeroUsage(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	result, err := calculator.Estimate(context.Background(), request(t, 0, "us-east-1"))
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}
	if got := result.MonthlyTotal.String(); got != "0.00" {
		t.Fatalf("monthly total = %s, want 0.00", got)
	}
}

func TestEC2AllowsFractionalUsage(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	result, err := calculator.Estimate(context.Background(), request(t, 1.5, "us-east-1"))
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}
	if got := result.MonthlyTotal.String(); got != "0.02" {
		t.Fatalf("monthly total = %s, want 0.02", got)
	}
}

func TestEC2RejectsUnsupportedRate(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	_, err := calculator.Estimate(context.Background(), request(t, 1, "ap-southeast-1"))
	if !errors.Is(err, catalog.ErrRateNotFound) {
		t.Fatalf("Estimate() error = %v, want rate lookup error", err)
	}
}

func TestEC2RejectsInvalidUsage(t *testing.T) {
	for _, hours := range []float64{-1, math.Inf(1), math.NaN()} {
		t.Run("invalid usage", func(t *testing.T) {
			_, err := domain.NewUsage("t3.micro", hours)
			if !errors.Is(err, domain.ErrInvalidEstimate) {
				t.Fatalf("NewUsage(%v) error = %v, want invalid estimate", hours, err)
			}
		})
	}
}

func request(t *testing.T, hours float64, region string) domain.EstimateRequest {
	t.Helper()
	usage, err := domain.NewUsage("t3.micro", hours)
	if err != nil {
		t.Fatalf("NewUsage() error = %v", err)
	}
	return domain.EstimateRequest{
		Provider: domain.ProviderAWS,
		Service:  "ec2",
		Region:   domain.Region(region),
		Usage:    usage,
	}
}
