package tcp

import (
	"errors"
	"sync"
)

var errNoServersInPool = errors.New("no servers in the pool")

type server struct {
	Handler

	name  string
	wight int
}

// WRRLoadBalancer is a native RoundRobin load balancer for TCP services.
type WRRLoadBalancer struct {
	// serverMu 用于保护 servers 和 status
	serverMu sync.Mutex
	servers  []server

	// status is a record of which child services of the Balancer are healthy, keyed
	// by name of child service. A service is initially added to the map when it is
	// created via Add, and it is later removed or added to the map as needed,
	// through the SetStatus method.
	status map[string]struct{}

	// updaters is the list of hooks that are run (to update the Balancer parent(s)), whenever the Balancer status changes.
	// No mutex is needed, as it is modified only during the configuration build.
	updaters []func(bool)

	index            int
	currentWeight    int
	wantsHealthCheck bool
}

func NewWRRLoadBalancer(wantsHealthCheck bool) *WRRLoadBalancer {
	return &WRRLoadBalancer{
		status:           make(map[string]struct{}),
		index:            -1,
		wantsHealthCheck: wantsHealthCheck,
	}
}

func (w *WRRLoadBalancer) nextServer() (Handler, error) {
	w.serverMu.Lock()
	defer w.serverMu.Unlock()

	if len(w.servers) == 0 || len(w.status) == 0 {
		return nil, errNoServersInPool
	}

	// The algo below may look messy, but it's actually very simple.
	// it calculates th GCD and subtracts it on every iteration, what interleaves servers.
	// and allows us not to build an iterator every time we readjust weights

	// Maximum weight across all enabled servers
	maximum := w.maxWeight()
	if maximum == 0 {
		return nil, errors.New("all servers have weight 0")
	}

	// GCD across all enabled servers
	divisor := w.weightGcd()

}

func (w *WRRLoadBalancer) maxWeight() int {
	maximum := -1
	for _, s := range w.servers {
		if s.wight > maximum {
			maximum = s.wight
		}
	}
	return maximum
}

// weightGcd 计算所有服务器权重
/*
示例：servers = [{name: "A", weight: 6}, {name: "B", weight: 4}, {name: "C", weight: 2}]

divisor 初始 = -1
  遍历 A: divisor == -1 → divisor = 6
  遍历 B: divisor != -1 → divisor = gcd(6, 4) = 2
  遍历 C: divisor != -1 → divisor = gcd(2, 2) = 2

最终 divisor = 2

┌─────────────────────────────────────────────────────────┐
│  权重: A=6, B=4, C=2                                    │
│  maximum = 6,  divisor = gcd(6,4,2) = 2                │
│                                                         │
│  currentWeight 从 6 开始，每次减 divisor=2：             │
│                                                         │
│  currentWeight=6: 选 weight≥6 的 → A (新请求分配)          │
│  currentWeight=4: 选 weight≥4 的 → A 或 B                │
│  currentWeight=2: 选 weight≥2 的 → A, B 或 C            │
│  currentWeight=0: 重置为 6                               │
│                                                         │
│  currentWeight 递减到 0 后重置回 max，循环往复             │
└─────────────────────────────────────────────────────────┘

朴素轮询：每个服务器连续分配完自己的份额
  A A A A A A  B B B B  C C  ← C 排在最后，长时间无请求
  流量不均衡，Burst 集中

GCD 平滑轮询：服务器交替出现
  A B A C B A  A B A C B A  ← A/B/C 均匀穿插
  每轮 6 次选择中 C 被选 1 次，分布均匀

核心效果：

GCD 决定了粒度：权重 6/4/2 → GCD=2 → 等价于 A=3份、B=2份、C=1份的细粒度交替
避免 consecutive burst：不会出现同一个后端连续被命中多次
不需要预建迭代器：每次选择都是 O(n) 的即时计算，权重动态变化后无需重建数据结构

*/
func (w *WRRLoadBalancer) weightGcd() int {
	divisor := -1
	for _, s := range w.servers {
		if divisor == -1 {
			divisor = s.wight
		} else {
			divisor = gcd(divisor, s.wight)
		}
	}
	return divisor
}

// gcd 计算 a 和 b 的最大公约数
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
