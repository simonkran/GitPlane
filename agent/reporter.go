package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// fluxReportGVR is the GroupVersionResource for the FluxReport CRD.
var fluxReportGVR = schema.GroupVersionResource{
	Group:    "fluxcd.controlplane.io",
	Version:  "v1",
	Resource: "fluxreports",
}

const (
	fluxReportName      = "flux"
	fluxReportNamespace = "flux-system"

	maxRetries       = 3
	baseRetryBackoff = 2 * time.Second
)

// Reporter fetches the FluxReport CR from the local cluster and POSTs it to
// the GitPlane API on a regular interval.
type Reporter struct {
	dynClient  dynamic.Interface
	httpClient *http.Client
	apiURL     string
	token      string
	interval   time.Duration
}

// NewReporter creates a Reporter using the in-cluster Kubernetes config.
func NewReporter(apiURL, token string, interval time.Duration) (*Reporter, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("building in-cluster config: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Reporter{
		dynClient: dynClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL:   apiURL,
		token:    token,
		interval: interval,
	}, nil
}

// Run starts the reporting loop. It blocks until ctx is cancelled.
func (r *Reporter) Run(ctx context.Context) {
	slog.Info("reporter loop started", "interval", r.interval.String())

	// Report immediately on startup, then on ticker.
	r.report(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("reporter loop stopping")
			return
		case <-ticker.C:
			r.report(ctx)
		}
	}
}

// report fetches the FluxReport and sends it to the API.
func (r *Reporter) report(ctx context.Context) {
	obj, err := r.fetchFluxReport(ctx)
	if err != nil {
		slog.Error("failed to fetch FluxReport", "error", err)
		return
	}

	summary := extractSummary(obj)
	slog.Info("fetched FluxReport",
		"components_healthy", summary.ComponentsHealthy,
		"reconcilers", summary.Reconcilers,
		"synced", summary.Synced,
	)

	raw, err := json.Marshal(obj.Object)
	if err != nil {
		slog.Error("failed to marshal FluxReport", "error", err)
		return
	}

	if err := r.sendReport(ctx, raw); err != nil {
		slog.Error("failed to send report", "error", err)
		return
	}

	slog.Info("report sent successfully")
}

// fetchFluxReport reads the FluxReport CR from the cluster.
func (r *Reporter) fetchFluxReport(ctx context.Context) (*unstructured.Unstructured, error) {
	return r.dynClient.Resource(fluxReportGVR).
		Namespace(fluxReportNamespace).
		Get(ctx, fluxReportName, metav1.GetOptions{})
}

// sendReport POSTs the raw FluxReport JSON to the GitPlane API with retry.
func (r *Reporter) sendReport(ctx context.Context, data []byte) error {
	endpoint := r.apiURL + "/api/v1/agent/report"

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * baseRetryBackoff
			slog.Info("retrying report", "attempt", attempt, "backoff", backoff.String())

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		lastErr = r.doPost(ctx, endpoint, data)
		if lastErr == nil {
			return nil
		}

		slog.Warn("report attempt failed", "attempt", attempt, "error", lastErr)
	}

	return fmt.Errorf("all %d report attempts failed, last error: %w", maxRetries+1, lastErr)
}

// doPost performs a single POST request.
func (r *Reporter) doPost(ctx context.Context, url string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.token)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}

// FluxReportSummary holds key fields extracted from a FluxReport for logging.
type FluxReportSummary struct {
	ComponentsHealthy bool
	Reconcilers       int
	Synced            bool
}

// extractSummary parses high-level status from the FluxReport unstructured object.
func extractSummary(obj *unstructured.Unstructured) FluxReportSummary {
	summary := FluxReportSummary{}

	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return summary
	}

	// Components health: check if all components report ready.
	if components, ok := spec["componentStatus"].([]interface{}); ok {
		allHealthy := true
		for _, c := range components {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if status, _ := cm["status"].(string); status != "True" {
				allHealthy = false
				break
			}
		}
		summary.ComponentsHealthy = allHealthy
	}

	// Reconciler stats.
	if reconcilers, ok := spec["reconcilerStatus"].([]interface{}); ok {
		summary.Reconcilers = len(reconcilers)
	}

	// Sync status: check if cluster sync reports ready.
	if sync, ok := spec["syncStatus"].(map[string]interface{}); ok {
		if status, _ := sync["status"].(string); status == "True" {
			summary.Synced = true
		}
	}

	return summary
}
