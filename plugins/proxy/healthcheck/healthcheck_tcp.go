package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/andrewbytecoder/nmq/internal/config/dynamic"
	"github.com/andrewbytecoder/nmq/internal/config/runtimecfg"
	"github.com/andrewbytecoder/nmq/pkg/options"
	"github.com/andrewbytecoder/nmq/plugins/proxy/tcp"
	"go.uber.org/zap"
)

// maxPayloadSize is the maximum payload size that can be sent during health checks.
const maxPayloadSize = 65535

type TCPHealthCheckTarget struct {
	Address string
	TLS     bool
	Dialer  tcp.Dialer
}

type ServiceTCPHealthChecker struct {
	log     *zap.Logger
	targets []TCPHealthCheckTarget

	balancer StatusSetter
	info     *runtimecfg.TCPServiceInfo

	config            *dynamic.TCPServerHealthCheck
	interval          time.Duration
	unhealthyInterval time.Duration
	timeout           time.Duration

	healthyTargets   chan *TCPHealthCheckTarget
	unhealthyTargets chan *TCPHealthCheckTarget

	serviceName string
}

func SetTCPLog(log *zap.Logger) options.Option {
	return func(o interface{}) {
		if hc, ok := o.(*ServiceTCPHealthChecker); ok {
			hc.log = log
		}
	}
}

func SetTCPConfig(config *dynamic.TCPServerHealthCheck) options.Option {
	return func(o interface{}) {
		if hc, ok := o.(*ServiceTCPHealthChecker); ok {
			hc.config = config
		}
	}
}

func SetTCPStatusSetter(balancer StatusSetter) options.Option {
	return func(o interface{}) {
		if hc, ok := o.(*ServiceTCPHealthChecker); ok {
			hc.balancer = balancer
		}
	}
}

func SetTCPRuntimeInfo(info *runtimecfg.TCPServiceInfo) options.Option {
	return func(o interface{}) {
		if hc, ok := o.(*ServiceTCPHealthChecker); ok {
			hc.info = info
		}
	}
}

func SetTCPHealthCheckTargets(targets []TCPHealthCheckTarget) options.Option {
	return func(o interface{}) {
		if hc, ok := o.(*ServiceTCPHealthChecker); ok {
			hc.targets = targets
		}
	}
}

func SetTCPHealthCheckServiceName(serviceName string) options.Option {
	return func(o interface{}) {
		if hc, ok := o.(*ServiceTCPHealthChecker); ok {
			hc.serviceName = serviceName
		}
	}
}

func NewServiceTCPHandlerChecker(ctx context.Context, options ...options.Option) *ServiceTCPHealthChecker {
	thc := &ServiceTCPHealthChecker{}
	for _, option := range options {
		option(thc)
	}
	interval := time.Duration(thc.config.Interval)
	if interval <= 0 {
		thc.log.Error("Health check interval smaller tha zero, default value will be used instead.")
		interval = time.Duration(dynamic.DefaultHealthCheckInterval)
	}

	// If the unhealthyInterval option i snot set, we use the interval option value
	//  to check the unhealthy targets as often as the healthy ones.\
	var unhealthyInterval time.Duration
	if thc.config.UnhealthyInterval != nil {
		unhealthyInterval = interval
	} else {
		unhealthyInterval = time.Duration(*thc.config.UnhealthyInterval)
		if unhealthyInterval <= 0 {
			thc.log.Error("Unhealthy health check interval smaller tha zero, default value will be used instead.")
			unhealthyInterval = time.Duration(dynamic.DefaultHealthCheckInterval)
		}
	}

	timeout := time.Duration(thc.config.Timeout)
	if timeout <= 0 {
		thc.log.Error("Health check timeout smaller tha zero, default value will be used instead.")
		timeout = time.Duration(dynamic.DefaultHealthCheckTimeout)
	}

	if thc.config.Send != "" && len(thc.config.Send) > maxPayloadSize {
		thc.log.Error("Health check send payload size is too large, default value will be used instead.", zap.Int("maxPayloadSize", maxPayloadSize))
		thc.config.Send = ""
	}

	thc.healthyTargets = make(chan *TCPHealthCheckTarget, len(thc.targets))
	for _, target := range thc.targets {
		thc.healthyTargets <- &target
	}
	thc.unhealthyTargets = make(chan *TCPHealthCheckTarget, len(thc.targets))

	return thc
}

func (thc *ServiceTCPHealthChecker) Launch(ctx context.Context) {
	thc.log.Info("starting unhealth check")
	go thc.healthCheck(ctx, thc.unhealthyTargets, thc.unhealthyInterval)
	thc.log.Info("starting health check")
	thc.healthCheck(ctx, thc.healthyTargets, thc.interval)
}

func (thc *ServiceTCPHealthChecker) healthCheck(ctx context.Context, targets chan *TCPHealthCheckTarget, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// We collect the targets to check once for all,
			// to avoid rechecking a target that has been moved during the health check.
			var targetsToCheck []*TCPHealthCheckTarget
			hasMoreTargets := true
			for hasMoreTargets {
				select {
				case <-ctx.Done():
					return
				case target := <-targets:
					targetsToCheck = append(targetsToCheck, target)
				default:
					hasMoreTargets = false
				}
			}
			// Now we can check the targets.
			for _, target := range targetsToCheck {
				select {
				case <-ctx.Done():
					return
				default:
				}

				up := true
				if err := thc.executeHealthCheck(ctx, thc.config, target); err != nil {
					// The context is canceled when the dynamic configuration is updated
					if errors.Is(err, context.Canceled) {
						return
					}
					thc.log.Warn("health check failed", zap.Error(err))
					up = false
				}
				// 能走到这里说明对应的target 是健康的
				thc.balancer.SetStatus(ctx, target.Address, up)

				var statusStr string
				if up {
					statusStr = runtimecfg.StatusUp
					thc.healthyTargets <- target
				} else {
					statusStr = runtimecfg.StatusDown
					thc.unhealthyTargets <- target
				}

				thc.info.UpdateServerStatus(target.Address, statusStr)

			}
		}

	}

}

// executeHealthCheck 执行一次健康检查
// 向指定的 target 发送 Send 内容，并验证返回的 Expect 内容是否匹配
func (thc *ServiceTCPHealthChecker) executeHealthCheck(ctx context.Context, config *dynamic.TCPServerHealthCheck, target *TCPHealthCheckTarget) error {
	addr := target.Address
	if config.Port != 0 {
		host, _, err := net.SplitHostPort(target.Address)
		if err != nil {
			return fmt.Errorf("invalid address %s: %w", target.Address, err)
		}
		// 如果指定的 Port 为 0，则使用配置的 Port
		addr = net.JoinHostPort(host, strconv.Itoa(config.Port))
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(config.Timeout)))
	defer cancel()

	conn, err := target.Dialer.DialContext(ctx, "tcp", addr, nil)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", addr, err)
	}
	defer conn.Close()

	if err = conn.SetDeadline(time.Now().Add(thc.timeout)); err != nil {
		return fmt.Errorf("setting timeout to %s: %w", thc.timeout, err)
	}

	// 如果配置了 Send，则发送 Send 内容
	if config.Send != "" {
		if _, err = conn.Write([]byte(config.Send)); err != nil {
			return fmt.Errorf("sending to %s: %w", addr, err)
		}
	}

	if config.Expect != "" {
		buf := make([]byte, len(config.Expect))
		if _, err := conn.Read(buf); err != nil {
			return fmt.Errorf("reading from %s: %w", addr, err)
		}

		if string(buf) != config.Expect {
			return fmt.Errorf("expected %s, but got %s", config.Expect, string(buf))
		}
	}

	return nil
}
