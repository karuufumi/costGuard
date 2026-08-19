package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"costguard/internal/catalog"
	"costguard/internal/estimate"
)

func TestEstimateJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	app := New(estimate.NewCalculator(catalog.NewEmbedded()), strings.NewReader(""), &out, &errOut, func() bool { return false })

	err := app.Run(context.Background(), []string{
		"estimate", "--provider", "aws", "--service", "ec2", "--region", "us-east-1",
		"--instance", "t3.micro", "--hours", "730", "--format", "json",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var result estimateJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if result.MonthlyTotal != "7.59" || result.Currency != "USD" || result.CatalogVersion == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHelpIncludesBanner(t *testing.T) {
	var out, errOut bytes.Buffer
	app := New(estimate.NewCalculator(catalog.NewEmbedded()), strings.NewReader(""), &out, &errOut, func() bool { return false })

	if err := app.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "____") || !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help output = %q", out.String())
	}
}

func TestInteractiveEstimate(t *testing.T) {
	var out, errOut bytes.Buffer
	app := New(
		estimate.NewCalculator(catalog.NewEmbedded()),
		strings.NewReader("aws\nec2\nus-east-1\nt3.micro\n730\n"),
		&out,
		&errOut,
		func() bool { return true },
	)

	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "Monthly: 7.59 USD") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestInteractiveEstimateRequiresTerminal(t *testing.T) {
	app := New(estimate.NewCalculator(catalog.NewEmbedded()), strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, func() bool { return false })
	if err := app.Run(context.Background(), nil); !errors.Is(err, ErrInteractiveInputUnavailable) {
		t.Fatalf("Run() error = %v, want terminal error", err)
	}
}

func TestCatalogAndDiscoveryOnlyReportEmbeddedSupport(t *testing.T) {
	var out, errOut bytes.Buffer
	app := New(estimate.NewCalculator(catalog.NewEmbedded()), strings.NewReader(""), &out, &errOut, func() bool { return false })

	if err := app.Run(context.Background(), []string{"services", "--provider", "aws"}); err != nil {
		t.Fatalf("services error = %v", err)
	}
	if got := out.String(); got != "ec2\tAmazon EC2\n" {
		t.Fatalf("services = %q", got)
	}
	out.Reset()

	if err := app.Run(context.Background(), []string{"catalog"}); err != nil {
		t.Fatalf("catalog error = %v", err)
	}
	if !strings.Contains(out.String(), "aws/ec2/us-east-1/t3.micro") {
		t.Fatalf("catalog = %q", out.String())
	}
}
