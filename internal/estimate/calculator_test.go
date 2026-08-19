package estimate

import (
	"context"
	"errors"
	"testing"

	"costguard/internal/catalog"
	"costguard/internal/domain"
)

func TestEC2MonthlyEstimate(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	result, err := calculator.Estimate(context.Background(), domain.EstimateRequest{
		Provider: "aws", Service: "ec2", Region: "us-east-1", Usage: domain.Usage{Instance: "t3.micro", Hours: 730},
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}
	if result.MonthlyTotal != "7.59" {
		t.Fatalf("monthly total = %s, want 7.59", result.MonthlyTotal)
	}
	if result.Breakdown[0].Rate != "0.010400" {
		t.Fatalf("rate = %s, want 0.010400", result.Breakdown[0].Rate)
	}
	if result.DailyTotal != "0.25" {
		t.Fatalf("daily total = %s, want 0.25", result.DailyTotal)
	}
}

func TestEC2AllowsZeroUsage(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	result, err := calculator.Estimate(context.Background(), domain.EstimateRequest{
		Provider: "aws", Service: "ec2", Region: "us-east-1", Usage: domain.Usage{Instance: "t3.micro"},
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}
	if result.MonthlyTotal != "0.00" {
		t.Fatalf("monthly total = %s, want 0.00", result.MonthlyTotal)
	}
}

func TestEC2RejectsUnsupportedRate(t *testing.T) {
	calculator := NewCalculator(catalog.NewEmbedded())
	_, err := calculator.Estimate(context.Background(), domain.EstimateRequest{
		Provider: "aws", Service: "ec2", Region: "ap-southeast-1", Usage: domain.Usage{Instance: "t3.micro", Hours: 1},
	})
	if !errors.Is(err, catalog.ErrRateNotFound) {
		t.Fatalf("Estimate() error = %v, want rate lookup error", err)
	}
}
