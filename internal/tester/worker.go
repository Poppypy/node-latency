package tester

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"node-latency/internal/model"
)

// RunTests is the main entry point. Dispatches to core-based or direct testing.
// onResult is called for each completed node — the caller (app.go) converts this to Wails events.
func RunTests(ctx context.Context, nodes []model.Node, settings model.TestSettings, onResult func(idx int, res model.Result), logf func(string, ...interface{})) {
	if settings.UseCoreTest {
		RunTestsWithCore(ctx, nodes, settings, onResult, logf)
		return
	}

	workers := settings.Concurrency
	if workers <= 0 {
		workers = 1
	}

	jobs := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range jobs {
			if ctx.Err() != nil {
				return
			}
			res := testNode(ctx, nodes[idx], settings)
			onResult(idx, res)
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range nodes {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}

	close(jobs)
	wg.Wait()
}

func testNode(ctx context.Context, node model.Node, settings model.TestSettings) model.Result {
	return TestNodeWithMeasure(ctx, settings, func(timeout time.Duration) (time.Duration, error) {
		return MeasureOnce(node, timeout)
	})
}

func TestNodeWithMeasure(ctx context.Context, settings model.TestSettings, measure func(timeout time.Duration) (time.Duration, error)) model.Result {
	var (
		latencies []int64
		sum       int64
		maxMs     int64
	)
	pass := settings.RequireAll
	anySuccess := false
	var errMsg string
	for i := 0; i < settings.Attempts; i++ {
		if ctx.Err() != nil {
			return model.Result{Done: true, Pass: false, Err: "canceled", Attempts: i}
		}
		d, err := measure(settings.Timeout)
		if err != nil {
			errMsg = err.Error()
			if settings.RequireAll || settings.StopOnFail {
				pass = false
				break
			}
			continue
		}
		ms := d.Milliseconds()
		anySuccess = true
		latencies = append(latencies, ms)
		sum += ms
		if ms > maxMs {
			maxMs = ms
		}
		if time.Duration(ms)*time.Millisecond > settings.Threshold {
			if settings.RequireAll {
				pass = false
				if settings.StopOnFail {
					break
				}
			}
		} else if !settings.RequireAll {
			pass = true
		}
	}
	avg := int64(0)
	if len(latencies) > 0 {
		avg = sum / int64(len(latencies))
	}
	if settings.RequireAll && len(latencies) < settings.Attempts {
		pass = false
	}
	if settings.RequireAll && maxMs > settings.Threshold.Milliseconds() {
		pass = false
	}
	if !settings.RequireAll && !anySuccess {
		pass = false
	}
	return model.Result{
		Done:       true,
		Pass:       pass,
		Err:        errMsg,
		LatencyMs:  latencies,
		AvgMs:      avg,
		MaxMs:      maxMs,
		Attempts:   settings.Attempts,
		Successful: len(latencies),
	}
}

func MeasureOnce(node model.Node, timeout time.Duration) (time.Duration, error) {
	addr := net.JoinHostPort(node.Host, strconv.Itoa(node.Port))
	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()

	if strings.EqualFold(node.Security, "tls") || strings.EqualFold(node.Security, "reality") || strings.EqualFold(node.Scheme, "trojan") {
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return 0, err
		}
		_ = conn.SetDeadline(time.Now().Add(timeout))
		cfg := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         node.SNI,
		}
		tlsConn := tls.Client(conn, cfg)
		if err := tlsConn.Handshake(); err != nil {
			_ = tlsConn.Close()
			return 0, err
		}
		_ = tlsConn.Close()
		return time.Since(start), nil
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start), nil
}
