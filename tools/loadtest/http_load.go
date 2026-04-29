package loadtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RunHTTPCheckIns exercises POST /v1/machines/{id}/check-ins for each machine row.
func RunHTTPCheckIns(ctx context.Context, baseURL string, machines []MachineRow, concurrency int, recorder *LatencyRecorder) error {
	if len(machines) == 0 {
		return fmt.Errorf("no machines")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	sem := make(chan struct{}, concurrency)
	errCh := make(chan error, len(machines))
	for _, m := range machines {
		m := m
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
		}
		go func() {
			defer func() { <-sem }()
			start := time.Now()
			err := postMachineCheckIn(ctx, baseURL, m)
			recorder.Add(time.Since(start), err != nil)
			if err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	}
	var firstErr error
	for range machines {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func postMachineCheckIn(ctx context.Context, base string, m MachineRow) error {
	body := map[string]any{
		"package_name":    "loadtest.avf",
		"version_name":    "1.0-loadtest",
		"version_code":    1,
		"android_release": "14",
		"sdk_int":         34,
		"manufacturer":    "AVF",
		"model":           "LoadTest",
		"timezone":        "UTC",
		"network_state":   "wifi",
		"boot_id":         "boot-loadtest",
		"occurred_at":     time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/v1/machines/%s/check-ins", base, m.MachineID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.JWT)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("check-in %s: status=%d body=%s", m.MachineID, resp.StatusCode, string(b))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// AdminPaths returns REST paths used by dashboard-style smoke loads (caller substitutes organization UUID).
func AdminPaths(orgID uuid.UUID) []string {
	o := orgID.String()
	from := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano)
	to := time.Now().UTC().Format(time.RFC3339Nano)
	q := fmt.Sprintf("from=%s&to=%s&limit=50", from, to)
	return []string{
		"/health/live",
		"/health/ready",
		"/v1/admin/machines?limit=50&offset=0",
		fmt.Sprintf("/v1/admin/organizations/%s/reports/machine-health?%s", o, q),
		fmt.Sprintf("/v1/admin/organizations/%s/reports/commands?%s", o, q),
		fmt.Sprintf("/v1/admin/organizations/%s/reports/inventory?%s", o, q),
	}
}

// RunHTTPAdminSequence GETs admin paths once per iteration (JWT required).
func RunHTTPAdminSequence(ctx context.Context, baseURL, adminJWT string, orgID uuid.UUID, iterations int, recorder *LatencyRecorder) error {
	baseURL = strings.TrimRight(baseURL, "/")
	paths := AdminPaths(orgID)
	for i := 0; i < iterations; i++ {
		for _, p := range paths {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			start := time.Now()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+p, nil)
			if err != nil {
				recorder.Add(time.Since(start), true)
				return err
			}
			if strings.HasPrefix(p, "/v1/admin") {
				req.Header.Set("Authorization", "Bearer "+adminJWT)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				recorder.Add(time.Since(start), true)
				return err
			}
			ok := resp.StatusCode < 400
			if resp.StatusCode >= 400 {
				_, _ = io.Copy(io.Discard, resp.Body)
			} else {
				_, _ = io.Copy(io.Discard, resp.Body)
			}
			_ = resp.Body.Close()
			recorder.Add(time.Since(start), !ok)
			if !ok {
				return fmt.Errorf("admin GET %s: status=%d", p, resp.StatusCode)
			}
		}
	}
	return nil
}
