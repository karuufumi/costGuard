// Package cli provides the terminal presentation adapter for costGuard.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"costguard/internal/domain"
)

var ErrInteractiveInputUnavailable = errors.New("interactive mode requires a terminal")

const banner = `
   ____          __  ______                 __
  / ___|___  ___| |_|  ____|_   _  __ _ _ __/ /_
 | |   / _ \/ __| __| |  _| | | | |/ _' | '__| '_ \
 | |__| (_) \__ \ |_| | |___| |_| | (_| | |  | | | |
  \____\___/|___/\__| |_____|\__,_|\__,_|_|  |_| |_|
`

type App struct {
	estimator domain.Estimator
	in        io.Reader
	out       io.Writer
	err       io.Writer
	isTTY     func() bool
}

func New(estimator domain.Estimator, in io.Reader, out, err io.Writer, isTTY func() bool) *App {
	return &App{estimator: estimator, in: in, out: out, err: err, isTTY: isTTY}
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		if a.isTTY == nil || !a.isTTY() {
			return ErrInteractiveInputUnavailable
		}
		return a.runInteractive(ctx)
	}

	switch args[0] {
	case "help", "--help", "-h":
		a.writeHelp()
		return nil
	case "estimate":
		return a.runEstimate(ctx, args[1:])
	case "services":
		return a.runServices(args[1:])
	case "regions":
		return a.runRegions(args[1:])
	case "catalog":
		return a.runCatalog(args[1:])
	case "version":
		fmt.Fprintln(a.out, "costguard dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q; run costguard help", args[0])
	}
}

func (a *App) runEstimate(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("estimate", flag.ContinueOnError)
	flags.SetOutput(a.err)
	provider := flags.String("provider", "aws", "cloud provider")
	service := flags.String("service", "ec2", "service")
	region := flags.String("region", "us-east-1", "provider region")
	instance := flags.String("instance", "t3.micro", "instance type")
	hours := flags.Float64("hours", 730, "monthly instance-hours")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	usage, err := domain.NewUsage(*instance, *hours)
	if err != nil {
		return err
	}
	result, err := a.estimator.Estimate(ctx, domain.EstimateRequest{
		Provider: domain.Provider(*provider),
		Service:  domain.Service(*service),
		Region:   domain.Region(*region),
		Usage:    usage,
	})
	if err != nil {
		return err
	}

	switch *format {
	case "text":
		writeTextEstimate(a.out, result)
		return nil
	case "json":
		return json.NewEncoder(a.out).Encode(jsonEstimate(result))
	default:
		return fmt.Errorf("unsupported format %q; use text or json", *format)
	}
}

func (a *App) runInteractive(ctx context.Context) error {
	reader := bufio.NewReader(a.in)
	writeBanner(a.out)
	fmt.Fprintln(a.out, "costGuard estimate (offline catalog)")
	provider, err := prompt(reader, a.out, "Provider", "aws")
	if err != nil {
		return err
	}
	service, err := prompt(reader, a.out, "Service", "ec2")
	if err != nil {
		return err
	}
	region, err := prompt(reader, a.out, "Region", "us-east-1")
	if err != nil {
		return err
	}
	instance, err := prompt(reader, a.out, "Instance", "t3.micro")
	if err != nil {
		return err
	}
	hours, err := prompt(reader, a.out, "Hours", "730")
	if err != nil {
		return err
	}
	return a.runEstimate(ctx, []string{
		"--provider", provider,
		"--service", service,
		"--region", region,
		"--instance", instance,
		"--hours", hours,
	})
}

func (a *App) runServices(args []string) error {
	provider, err := providerFlag("services", args)
	if err != nil {
		return err
	}
	if provider != "aws" {
		return fmt.Errorf("no offline services are available for provider %q", provider)
	}
	fmt.Fprintln(a.out, "ec2\tAmazon EC2")
	return nil
}

func (a *App) runRegions(args []string) error {
	provider, err := providerFlag("regions", args)
	if err != nil {
		return err
	}
	if provider != "aws" {
		return fmt.Errorf("no offline regions are available for provider %q", provider)
	}
	fmt.Fprintln(a.out, "us-east-1\tUS East (N. Virginia)")
	return nil
}

func (a *App) runCatalog(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("catalog does not accept arguments")
	}
	fmt.Fprintln(a.out, "version: 2026-08-18.1")
	fmt.Fprintln(a.out, "source: embedded")
	fmt.Fprintln(a.out, "supported product: aws/ec2/us-east-1/t3.micro")
	return nil
}

func (a *App) writeHelp() {
	writeBanner(a.out)
	fmt.Fprint(a.out, `costGuard estimates cloud costs from a local pricing catalog.

Usage:
  costguard                         interactive estimate
  costguard estimate [flags]        non-interactive estimate
  costguard services --provider aws list available services
  costguard regions --provider aws  list available regions
  costguard catalog                 show local catalog metadata
  costguard version                 show build version
  costguard serve                   start the private HTTP adapter

Estimate flags:
  --provider aws --service ec2 --region us-east-1
  --instance t3.micro --hours 730 --format text|json
`)
}

func writeBanner(out io.Writer) {
	fmt.Fprint(out, banner)
}

func providerFlag(name string, args []string) (string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	provider := flags.String("provider", "aws", "cloud provider")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return *provider, nil
}

func prompt(reader *bufio.Reader, out io.Writer, label, fallback string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", label, fallback)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		if errors.Is(err, io.EOF) {
			return "", errors.New("interactive input closed")
		}
		return fallback, nil
	}
	return value, nil
}

func writeTextEstimate(out io.Writer, result domain.EstimateResult) {
	fmt.Fprintf(out, "Provider: %s\nService: %s\nRegion: %s\n", result.Provider, result.Service, result.Region)
	for _, item := range result.Breakdown {
		fmt.Fprintf(out, "\n%s\n  %.6g %s at %s USD\n  Total: %s USD\n", item.Description, item.Quantity.Hours(), item.Unit, item.Rate.PreciseString(), item.Total.String())
	}
	fmt.Fprintf(out, "\nHourly:  %s %s\nDaily:   %s %s\nMonthly: %s %s\nAnnual:  %s %s\n", result.HourlyTotal.String(), result.Currency, result.DailyTotal.String(), result.Currency, result.MonthlyTotal.String(), result.Currency, result.AnnualTotal.String(), result.Currency)
	fmt.Fprintf(out, "Catalog: %s (%s)\n", result.CatalogVersion, result.CatalogSource)
}

type estimateJSON struct {
	Provider       string         `json:"provider"`
	Service        string         `json:"service"`
	Region         string         `json:"region"`
	Currency       string         `json:"currency"`
	HourlyTotal    string         `json:"hourly_total"`
	DailyTotal     string         `json:"daily_total"`
	MonthlyTotal   string         `json:"monthly_total"`
	AnnualTotal    string         `json:"annual_total"`
	CatalogVersion string         `json:"catalog_version"`
	CatalogSource  string         `json:"catalog_source"`
	Breakdown      []lineItemJSON `json:"breakdown"`
	Assumptions    []string       `json:"assumptions"`
	Warnings       []string       `json:"warnings"`
}

type lineItemJSON struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	Rate        string  `json:"rate"`
	Total       string  `json:"total"`
}

func jsonEstimate(result domain.EstimateResult) estimateJSON {
	breakdown := make([]lineItemJSON, len(result.Breakdown))
	for i, item := range result.Breakdown {
		breakdown[i] = lineItemJSON{item.Description, item.Quantity.Hours(), item.Unit, item.Rate.PreciseString(), item.Total.String()}
	}
	return estimateJSON{
		Provider: string(result.Provider), Service: string(result.Service), Region: string(result.Region), Currency: string(result.Currency),
		HourlyTotal: result.HourlyTotal.String(), DailyTotal: result.DailyTotal.String(), MonthlyTotal: result.MonthlyTotal.String(), AnnualTotal: result.AnnualTotal.String(),
		CatalogVersion: result.CatalogVersion, CatalogSource: result.CatalogSource, Breakdown: breakdown,
		Assumptions: result.Assumptions, Warnings: result.Warnings,
	}
}
