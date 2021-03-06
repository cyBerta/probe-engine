package dnscheck

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/apex/log"
	"github.com/ooni/probe-engine/internal/mockable"
	"github.com/ooni/probe-engine/model"
)

func TestExperimentNameAndVersion(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{Domain: "example.com"})

	if measurer.ExperimentName() != "dnscheck" {
		t.Error("unexpected experiment name")
	}

	if measurer.ExperimentVersion() != "0.0.1" {
		t.Error("unexpected experiment version")
	}
}

func TestDNSCheckFailsWithoutInput(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{Domain: "example.com"})

	err := measurer.Run(
		context.Background(),
		newsession(),
		new(model.Measurement),
		model.NewPrinterCallbacks(log.Log),
	)

	if !errors.Is(err, ErrInputRequired) {
		t.Fatal("expected no input error")
	}
}

func TestDNSCheckFailsInvalidInput(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{})

	// test with invalid URL
	err := measurer.Run(
		context.Background(),
		newsession(),
		&model.Measurement{Input: "Not a valid URL \x7f"},
		model.NewPrinterCallbacks(log.Log),
	)

	if !errors.Is(err, ErrInvalidURL) {
		t.Fatal("expected invalid input error")
	}

	// test with unsupported protocol
	err = measurer.Run(
		context.Background(),
		newsession(),
		&model.Measurement{Input: "file://1.1.1.1"},
		model.NewPrinterCallbacks(log.Log),
	)

	if !errors.Is(err, ErrUnsupportedURLScheme) {
		t.Fatal("expected unsupported scheme error")
	}
}

func TestWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel the context
	measurer := NewExperimentMeasurer(Config{})

	// test with valid DNS endpoint
	err := measurer.Run(
		ctx,
		newsession(),
		&model.Measurement{Input: "dot://1.1.1.1"},
		model.NewPrinterCallbacks(log.Log),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMakeResolverURL(t *testing.T) {
	// test address substitution
	addr := "255.255.255.0"
	resolver := makeResolverURL(&url.URL{Host: "example.com"}, addr)
	resolverURL, err := url.Parse(resolver)
	if err != nil {
		t.Fatal(err)
	}
	if resolverURL.Host != addr {
		t.Fatal("expected address to be set as host")
	}

	// test IPv6 URLs are quoted
	addr = "2001:db8:85a3:8d3:1319:8a2e:370"
	resolver = makeResolverURL(&url.URL{Host: "example.com"}, addr)
	resolverURL, err = url.Parse(resolver)
	if err != nil {
		t.Fatal(err)
	}
	if resolverURL.Host != "["+addr+"]" {
		t.Fatal("expected URL host to be quoted")
	}
}

func TestDNSCheckValid(t *testing.T) {
	measurer := NewExperimentMeasurer(Config{})
	measurement := model.Measurement{Input: "dot://one.one.one.one:853"}
	// test with valid DNS endpoint
	err := measurer.Run(
		context.Background(),
		newsession(),
		&measurement,
		model.NewPrinterCallbacks(log.Log),
	)

	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	tk := measurement.TestKeys.(*TestKeys)
	if tk.Domain != defaultDomain {
		t.Fatal("unexpected default value for domain")
	}
	if tk.Bootstrap == nil {
		t.Fatal("unexpected value for bootstrap")
	}
	if tk.BootstrapFailure != nil {
		t.Fatal("unexpected value for bootstrap_failure")
	}
	if len(tk.Lookups) <= 0 {
		t.Fatal("unexpected value for lookups")
	}
}

func newsession() model.ExperimentSession {
	return &mockable.Session{MockableLogger: log.Log}
}
