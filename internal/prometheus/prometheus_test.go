package prometheus

import (
	"fmt"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"hytera.com/ncp/internal/metrics"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
)

// TestCounter puts some deltas through the counter, and then calls the value
// func to check that the counter has the correct final value.
func CounterTool(counter metrics.Counter, value func() float64) error {
	want := FillCounter(counter)
	if have := value(); want != have {
		return fmt.Errorf("want %f, have %f", want, have)
	}

	return nil
}

// FillCounter puts some deltas through the counter and returns the total value.
func FillCounter(counter metrics.Counter) float64 {
	a := rand.Perm(100)
	n := rand.Intn(len(a))

	var want float64
	for i := 0; i < n; i++ {
		f := float64(a[i])
		counter.Add(f)
		want += f
	}
	return want
}
func TestCounter(t *testing.T) {
	promhttp.Handler()
	s := httptest.NewServer(promhttp.HandlerFor(stdprometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	defer s.Close()

	// 从默认服务中抓取一个指标
	scrape := func() string {
		resp, _ := http.Get(s.URL)
		buf, _ := io.ReadAll(resp.Body)
		return string(buf)
	}

	namespace, subsystem, name := "ns", "ss", "foo"
	re := regexp.MustCompile(namespace + `_` + subsystem + `_` + name + `{alpha="alpha-value",beta="beta-value"} ([0-9\.]+)`)

	// 数据注册进去将会被收集
	counter := NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      "This is the help string.",
	}, []string{"alpha", "beta"}).With("beta", "beta-value", "alpha", "alpha-value") // order shouldn't matter

	value := func() float64 {
		matches := re.FindStringSubmatch(scrape())
		f, _ := strconv.ParseFloat(matches[1], 64)
		return f
	}

	if err := CounterTool(counter, value); err != nil {
		t.Fatal(err)
	}
}
