// Package tcp 提供 TCP 服务的加权轮询（Weighted Round Robin）负载均衡器。
//
// WRRLoadBalancer 基于平滑加权轮询算法（GCD 平滑变体），实现 TCP 连接的加权分配。
// 与传统加权轮询不同，该算法通过最大公约数（GCD）作为递减步长，使高权重服务器
// 和低权重服务器在时间上均匀穿插，避免"连续分配"导致的流量 Burst。
//
// 同时支持健康检查（Health Check）：通过 SetStatus 接口标记后端健康状态，
// 只有状态为健康的服务器才会参与负载均衡。当所有服务器都不健康时，
// 通过 RegisterStatusUpdater 注册的回调向上级通知整个负载均衡器的状态变化。
package tcp

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
)

// errNoServersInPool 表示当前没有任何可用服务器。
// 两种情况会触发此错误：
//  1. servers 列表为空（Add 从未被调用）
//  2. status 为空（虽然有服务器，但所有服务器都不健康）
var errNoServersInPool = errors.New("no servers in the pool")

// server 表示一个后端服务器的内部结构。
// 嵌入 Handler 接口（通常是一个反向代理 TCP handler），
// 保存服务器的逻辑名称和权重值。
type server struct {
	Handler // 嵌入 Handler 接口，实现多态：ServeTCP 时直接调用该服务器的实际处理逻辑

	name  string // 服务器逻辑名称，用于健康检查状态映射（status map 的 key）
	wight int    // 权重值，值越大分配的请求越多（wight 为 typo，实际含义为 weight）
}

// WRRLoadBalancer 平滑加权轮询 TCP 负载均衡器。
//
// 核心数据结构说明：
//   - servers: 数组存储所有已注册的后端服务器（包括健康和不健康的）
//   - status:  map 记录当前健康的后端（key=name, value=struct{} 零内存占用标记）
//     只有 status 中存在的服务器才参与轮询选择
//   - index:  环形数组的当前指针位置，每次选择后递增（取模取环），保证轮询连续性
//   - currentWeight: 平滑算法的核心变量，从 maxWeight 递减到 0，控制哪些服务器可被选中
//   - updaters: 状态变化通知回调列表，当 WRRLoadBalancer 作为一个整体（作为其他
//     负载均衡器的子节点）状态变化时，通知父节点
//
// 并发安全：
//   - serverMu 保护 servers 切片和 status map（Add / SetStatus / nextServer 均加锁）
//   - updaters 只在配置构建阶段修改，运行时只读，不需要锁保护
type WRRLoadBalancer struct {
	log *zap.Logger // 日志记录器

	// serverMu 保护 servers 切片和 status map 的并发安全。
	// 所有对 servers 和 status 的读写操作（Add/SetStatus/nextServer）都需要获取此锁。
	serverMu sync.Mutex
	servers  []server // 所有已注册的服务器列表（含健康和不健康），按添加顺序排列

	// status 记录当前哪些服务器是健康的。
	// key: 服务器的 name（通过 Add 注册时的名称）
	// value: struct{}（仅作标记，不占额外内存）
	//
	// 生命周期：
	//   - Add 时：服务器刚注册，初始为健康 → 加入 status
	//   - SetStatus(false) 时：服务器变故障 → 从 status 删除
	//   - SetStatus(true)  时：服务器恢复健康 → 重新加入 status
	//
	// nextServer 只从 status 中存在的服务器里选择，因此：
	//   如果 status 为空 → 返回 errNoServersInPool（即使 servers 不为空）
	//   如果 status 不为空 → 在 servers 中遍历，跳过 status 中不存在的
	status map[string]struct{}

	// updaters 状态变化回调函数列表。
	// 当 WRRLoadBalancer 作为某个上级负载均衡器的子节点时，
	// 上级通过 RegisterStatusUpdater 注册回调，监听本负载均衡器的整体状态变化。
	//
	// 触发时机（SetStatus 中）：
	//   当 status 从空变非空（所有服务器故障→至少有一个健康）或
	//   从非空变空（至少有一个健康→所有服务器故障）时，遍历调用所有回调。
	//
	// 入参 up: true=至少有一个健康的服务器, false=所有服务器都不健康
	//
	// 并发安全：只在配置构建阶段追加，运行时只读，不需要锁保护。
	updaters []func(bool)

	index            int  // 环形数组遍历指针，记录上次被选中的服务器在 servers 中的位置，初始=-1
	currentWeight    int  // 当前权重阈值，只有 weight > currentWeight 的服务器才能被选中，从 maxWeight 递减到 0
	wantsHealthCheck bool // 是否启用了健康检查，决定 RegisterStatusUpdater 是否允许注册
}

// NewWRRLoadBalancer 创建一个新的 WRRLoadBalancer 实例。
//
// 参数：
//
//	log: 日志记录器（传值而非指针，内部取地址保存）
//	wantsHealthCheck: 是否启用健康检查，影响 RegisterStatusUpdater 的行为
//
// 初始化：
//   - status map 初始化为空 map（因为后续通过 Add 逐个加入）
//   - index 初始为 -1，第一次 nextServer 时 index+1=0，指向第一个服务器
//   - currentWeight 初始为 0，第一次遍历完整一圈后会被重置为 maximum
func NewWRRLoadBalancer(log *zap.Logger, wantsHealthCheck bool) *WRRLoadBalancer {
	return &WRRLoadBalancer{
		log:              log,
		status:           make(map[string]struct{}),
		index:            -1,
		wantsHealthCheck: wantsHealthCheck,
	}
}

// ServeTCP 处理一个 TCP 连接，将连接转发到加权轮询选出的后端服务器。
//
// 执行流程：
//  1. 调用 nextServer() 按加权轮询算法选出下一个可用服务器
//  2. 如果所有服务器都不可用（errNoServersInPool），关闭连接并返回
//  3. 如果选出了服务器，调用该服务器的 ServeTCP 处理连接
//
// 错误处理策略：
//   - errNoServersInPool（无可用服务器）：静默关闭连接，不记录错误日志
//     （这是预期内的情况，所有服务器故障时应优雅降级而非告警刷屏）
//   - 其他错误（如所有服务器 weight=0）：记录错误日志后关闭连接
func (w *WRRLoadBalancer) ServeTCP(conn WriteCloser) {
	next, err := w.nextServer()
	if err != nil {
		// if err 不为 nil → 获取服务器失败
		//
		// if !errors.Is(err, errNoServersInPool) 为 true：
		//   错误不是"无可用服务器"，而是其他异常（如 weight 全为 0）
		//   → 需要记录错误日志，方便排查非预期问题
		//
		// if !errors.Is(err, errNoServersInPool) 为 false（即 err == errNoServersInPool）：
		//   所有服务器都不可用，这是正常场景（所有后端故障了）
		//   → 不记录错误日志，避免告警风暴
		if !errors.Is(err, errNoServersInPool) {
			w.log.Error("WRRLoadBalancer: no servers in pool", zap.Error(err))
		}
		_ = conn.Close() // 无论哪种错误，都无法处理连接，关闭连接
		return
	}

	// err == nil：成功选出一个后端服务器，将连接转发给它处理
	next.ServeTCP(conn)
}

// Add 向负载均衡器注册一个后端服务器。
//
// 参数：
//
//	name: 服务器逻辑名称，作为健康检查状态映射的 key
//	handler: 实际处理 TCP 连接的 Handler（通常是一个反向代理）
//	weight: 权重指针，nil 表示使用默认权重 1；非 nil 取其所指的值
//
// 副作用：
//   - 在 servers 末尾追加新服务器
//   - 在 status 中标记该服务器为健康（新注册的服务器默认为健康状态）
//
// 并发安全：通过 serverMu 加锁保护。
//
// 注意：weight 默认为 1 意味着所有服务器不加权配置时退化为普通轮询。
func (w *WRRLoadBalancer) Add(name string, handler Handler, weight *int) {
	// 解析权重：
	//   if weight == nil：用户未配置权重 → 使用默认值 1（所有服务器平等对待）
	//   if weight != nil：使用用户配置的权重值
	we := 1
	if weight != nil {
		we = *weight
	}

	w.serverMu.Lock()
	defer w.serverMu.Unlock()

	// 在 servers 切片末尾追加新服务器，保持注册顺序
	w.servers = append(w.servers, server{
		Handler: handler,
		name:    name,
		wight:   we,
	})

	// 新注册的服务器默认为健康状态，加入 status map
	// struct{} 是空结构体，零内存占用，仅作为"存在"标记
	w.status[name] = struct{}{}
}

// SetStatus 实现健康检查的 StatusSetter 接口，更新指定服务器的健康状态。
//
// 参数：
//
//	ctx: 上下文（当前仅用于接口约定，内部未使用）
//	childName: 要更新状态的服务器的 name（必须与 Add 时注册的 name 一致）
//	up: true=健康可用, false=故障不可用
//
// 核心逻辑（两次快照对比）：
//
//	第1步：记录更新前的整体状态 → upBefore
//	      - upBefore=true:  变更前至少有一个健康服务器（整体可用）
//	      - upBefore=false: 变更前所有服务器都不健康（整体不可用）
//
//	第2步：更新指定服务器的状态（加入或从 status 中移除）
//
//	第3步：记录更新后的整体状态 → upAfter
//	      - upAfter=true:  变更后至少有一个健康服务器（整体可用）
//	      - upAfter=false: 变更后所有服务器都不健康（整体不可用）
//
//	第4步：比较 upBefore 和 upAfter
//	      - 相同：整体状态未变化（例如：3个健康变2个健康，仍然是"整体可用"）
//	        → 不通知父节点，避免不必要的抖动
//	      - 不同：整体状态发生变化（从可用→不可用，或不可用→可用）
//	        → 遍历 upaters 回调列表，通知所有父节点
//
// 设计意图：
//
//	负载均衡器作为父节点的子节点时，父节点不需要关心每个具体后端的状态，
//	只需知道"这个负载均衡器作为整体是否可用"。SetStatus 只在整体状态
//	发生质变（有↔无）时才通知，减少不必要的状态传播。
func (w *WRRLoadBalancer) SetStatus(ctx context.Context, childName string, up bool) {
	w.serverMu.Lock()
	defer w.serverMu.Unlock()

	// ── 第1步：快照 → 变更前是否有健康服务器 ──
	// len(w.status) > 0 为 true：至少有一个健康的服务器
	// len(w.status) > 0 为 false：没有任何健康的服务器
	upBefore := len(w.status) > 0

	// 构建日志用的状态字符串
	status := "DOWN"
	if up {
		status = "UP"
	}

	w.log.Info("WRRLoadBalancer: server status changed", zap.String("server", childName), zap.String("status", status))

	// ── 第2步：更新指定服务器的状态 ──
	// if up 为 true：服务器恢复健康 → 重新加入 status map 参与轮询
	// if up 为 false：服务器故障 → 从 status map 中删除，不再参与轮询
	if up {
		w.status[childName] = struct{}{}
	} else {
		delete(w.status, childName)
	}

	// ── 第3步：快照 → 变更后是否有健康服务器 ──
	upAfter := len(w.status) > 0

	// 构建变更后的日志用状态字符串
	status = "DOWN"
	if upAfter {
		status = "UP"
	}

	// ── 第4步：判断整体状态是否发生质变 ──
	// if upBefore == upAfter 为 true：
	//   整体状态没有质变。
	//   例如：之前有 2 个健康的服务器，变更为有 1 个健康的服务器
	//   虽然健康数量减少了，但"整体可用"这个布尔状态没变。
	//   → 不需要通知父节点，记录日志后直接返回
	//
	// if upBefore == upAfter 为 false：
	//   整体状态发生了质变（0→1 或 1→0）。
	//   例如：最后一个健康服务器故障（upBefore=true, upAfter=false）
	//   或：第一个服务器从故障恢复（upBefore=false, upAfter=true）
	//   → 需要通知所有注册的父节点
	if upBefore == upAfter {
		w.log.Info("WRRLoadBalancer: server status unchanged",
			zap.String("server", childName), zap.String("status", status))
		return
	}

	// ── 整体状态发生质变，通知所有父节点 ──
	// 遍历 updaters 回调列表，每个回调用 upAfter 作为参数：
	//   upAfter=true:  通知父节点"我现在可用了"
	//   upAfter=false: 通知父节点"我完全不可用了"
	for _, f := range w.updaters {
		f(upAfter)
	}
}

// RegisterStatusUpdater 实现 StatusUpdater 接口，注册状态变化回调。
//
// 当本负载均衡器的整体健康状态发生变化时（从有健康服务器变为全故障，
// 或从全故障变为有健康服务器），会调用所有注册的回调函数。
//
// 参数 f: 回调函数，入参 bool 表示整体是否可用
//
// 返回值：
//   - nil: 注册成功
//   - error: wantsHealthCheck=false 时拒绝注册（未启用健康检查的负载均衡器无需状态传播）
//
// 并发安全：updaters 只在配置构建阶段修改，运行时只读，不需要锁保护。
func (w *WRRLoadBalancer) RegisterStatusUpdater(f func(bool)) error {
	// if !w.wantsHealthCheck 为 true：
	//   负载均衡器未启用健康检查。
	//   既然不检查健康，就谈不上"整体状态变化"，拒绝注册回调。
	//   这是一个防御性检查，防止不合理的配置组合。
	if !w.wantsHealthCheck {
		return errors.New("healthCheck not enabled in config for this weighted service")
	}

	// 追加回调到列表末尾
	w.updaters = append(w.updaters, f)

	return nil
}

// nextServer 使用平滑加权轮询算法（GCD Smooth Weighted Round Robin）选出下一个可用服务器。
//
// 算法核心思想：
//
//	传统加权轮询会导致高权重服务器被连续选中多次（如 A权重6→连续6次选A），
//	造成流量 Burst。平滑加权轮询通过 GCD 作为递减步长，让不同权重的服务器
//	均匀穿插，避免"扎堆"分配。
//
// 算法步骤（以 servers=[A:6, B:4, C:2], GCD=2 为例）：
//
//	currentWeight 从 maximum(6) 开始：
//	┌──────────────────────────────────────────────────────┐
//	│ currentWeight=6: 选 weight>6 的 → 没有 → 跳过         │
//	│ currentWeight=6: 选 weight>6 的 → A(weight=6) 不满足 → 跳过 │
//	│                 选 weight>6 的 → B(weight=4) 不满足 → 跳过 │
//	│                 选 weight>6 的 → C(weight=2) 不满足 → 跳过 │
//	│                 一圈走完，index 回到 0 → currentWeight=6-GCD=4 │
//	│                 选 weight>4 的 → A(weight=6) 满足 → 返回 A  ✓│
//	│ currentWeight=4: 选 weight>4 的 → B(weight=4) 不满足 → 跳过 │
//	│                 选 weight>4 的 → C(weight=2) 不满足 → 跳过 │
//	│                 一圈走完 → currentWeight=4-GCD=2            │
//	│                 选 weight>2 的 → A(weight=6) 满足 → 返回 A  ✓│
//	│ currentWeight=2: 选 weight>2 的 → B(weight=4) 满足 → 返回 B  ✓│
//	│                 选 weight>2 的 → C(weight=2) 不满足 → 跳过 │
//	│                 一圈走完 → currentWeight=2-GCD=0 → 重置为 6 │
//	│                 选 weight>0 的 → A(weight=6) 满足 → 返回 A ✓ │
//	└──────────────────────────────────────────────────────┘
//
//	输出序列：A B A C B A  A B A C B A ...
//	每 6 次选择中：A被选3次、B被选2次、C被选1次，比例恰好 6:4:2
//
// 为什么需要 for 循环：
//
//	不是所有服务器都满足 weight > currentWeight 的条件，且不健康的服务器
//	也要被跳过。for 循环从 index 处继续遍历环形数组，直到找到符合
//	条件的服务器。由于 totalWeight > 0 且至少有一个健康服务器，
//	循环一定会在有限步数内找到并返回。
func (w *WRRLoadBalancer) nextServer() (Handler, error) {
	w.serverMu.Lock()
	defer w.serverMu.Unlock()

	// ── 前置检查：是否有可用服务器 ──
	// if len(w.servers) == 0：没有任何被注册的后端服务器（Add 从未被调用）
	//   → 返回 errNoServersInPool，让调用方优雅处理（关闭连接）
	//
	// if len(w.status) == 0：有服务器被注册，但全部不健康
	//   → 也返回 errNoServersInPool，因为健康检查要求只转发给健康的服务器
	//
	// 这两个条件用 || 连接：任一满足即表示无服务器可用
	if len(w.servers) == 0 || len(w.status) == 0 {
		return nil, errNoServersInPool
	}

	// ── 计算最大权重 ──
	// maxWeight 遍历所有 servers（包括不健康的），取最大权重值。
	// 注意：这里不区分健康状态，因为算法需要的 maximum 是全局的权重上限。
	maximum := w.maxWeight()

	// if maximum == 0 为 true：所有服务器的权重均为 0
	//   权重为 0 表示该服务器不参与分配，如果所有权重都为 0，
	//   无论怎么选择都选不到，无法继续。返回非 errNoServersInPool 的错误，
	//   让调用方记录错误日志（见 ServeTCP 中的 if !errors.Is 判断）。
	if maximum == 0 {
		return nil, errors.New("all servers have weight 0")
	}

	// ── 计算 GCD ──
	// 遍历所有 servers，计算所有权重的最大公约数，作为 currentWeight 的递减步长。
	// GCD 越大，步长越大，服务器交替越粗糙（接近朴素轮询）；
	// GCD 越小，步长越小，服务器交替越细腻（更平滑）。
	gcd := w.weightGcd()

	// ── 平滑加权轮询主循环 ──
	// index 是结构体字段，记录上次选择的位置，保证每次选择从上一次的位置继续，
	// 而不是每次都从 0 开始。这确保了轮询的连续性。
	for {
		// 移动到环形数组的下一个位置
		// w.index = (w.index + 1) % len(w.servers)
		// 初始 index=-1 时：(0) % n = 0 → 从第一个服务器开始
		// 后续每次递增，到达末尾时 n%n=0 → 回到开头，形成环形遍历
		w.index = (w.index + 1) % len(w.servers)

		// if w.index == 0 为 true：刚完成一整圈的遍历，回到了起始位置
		//   此时需要调整 currentWeight 阈值，让更多（权重更低的）服务器能被选中。
		//
		//   调整逻辑：
		//     currentWeight -= gcd：将阈值降低一个步长
		//       例如 currentWeight=6, gcd=2 → currentWeight=4
		//       此时满足 weight>4 的服务器变多（原本只有 weight>6 的服务器才被选中）
		//
		//     if currentWeight <= 0 为 true：
		//       阈值已降到 0 或以下，说明本轮所有服务器都有机会被选过一次了
		//       重置 currentWeight=maximum，开始新一轮
		//
		//     if currentWeight <= 0 为 false：
		//       阈值还在正数范围内，继续用当前的 currentWeight 进行下一圈遍历
		if w.index == 0 {
			w.currentWeight -= gcd
			if w.currentWeight <= 0 {
				w.currentWeight = maximum
			}
		}

		// 获取 index 位置的服务器
		svr := w.servers[w.index]

		// ── 选择条件判断 ──
		// 条件1：_, ok := w.status[svr.name]；
		//        ok==true  → 该服务器在 status map 中，是健康的 → 继续检查条件2
		//        ok==false → 该服务器不在 status map 中，不健康 → 整体为 false，跳过
		//
		// 条件2：svr.wight > w.currentWeight；
		//        权重 > 当前阈值 → 该服务器有资格在当前这轮被选中
		//        权重 <= 当前阈值 → 该服务器在当前这轮"权重不足"，跳过
		//
		// 两个条件用 && 连接：必须同时满足（健康 + 权重足够）才会被选中。
		// 如果任一条件不满足 → 回到 for 开头继续检查下一个服务器。
		//
		// 为什么用 > 而不是 >=？
		//   算法设计：currentWeight 从 maximum 递减，递减时 weight==currentWeight
		//   的服务器会在下一圈被选中，形成自然的交替顺序，避免 weight==currentWeight
		//   的服务器在递减临界点被"抢占"选中，破坏平滑性。
		if _, ok := w.status[svr.name]; ok && svr.wight > w.currentWeight {
			return svr, nil
		}
		// 条件不满足 → 继续 for 循环，检查环形数组的下一个服务器
	}
}

// maxWeight 遍历所有服务器（包括不健康的），返回最大权重。
//
// 为什么包含不健康的服务器？
//
//	算法需要的是"绝对"的最大权重值，用于确定 currentWeight 的起始和重置值。
//	如果只考虑健康的服务器，当高权重服务器故障后，
//	maximum 会变小，影响 currentWeight 的重置和整体分配比例。
//	保持 maximum 不变可以确保权重比例的稳定。
//
// 初始值 -1 的作用：
//
//	确保任意 weight >= 0 的值都能被正确选出。
//	如果初始为 0，当所有服务器 weight=0 时，maxWeight 返回 0，
//	nextServer 中的 if maximum==0 会被正确触发。
//	如果初始为 -1 且所有服务器 weight=0，返回 -1，
//	nextServer 中 if maximum==0 为 false，会继续执行但 gcd 会出问题。
//
//	实际上这里用的是 -1 而不是 0，是因为代码设计上假设至少有一个服务器的 weight>0。
//	如果确实全部 weight=0，将在 nextServer 中被 maximum==0 检查捕获。
func (w *WRRLoadBalancer) maxWeight() int {
	maximum := -1 // 初始设为 -1，确保任何 weight>=0 都能覆盖
	for _, s := range w.servers {
		// if s.wight > maximum 为 true：
		//   当前服务器的权重比已知的最大值更大
		//   → 更新 maximum 为这个更大的值
		// 如果 equal → 不更新，保持第一个遇到的最大值
		if s.wight > maximum {
			maximum = s.wight
		}
	}
	// 遍历完毕后，maximum 是所有服务器中的最大权重
	return maximum
}

// weightGcd 计算所有服务器权重的最大公约数（GCD）。
//
// divisor 初始为 -1 作为"未初始化"标志：
//   - 遇到第一个服务器：divisor == -1 → 直接取第一个服务器的权重
//   - 后续服务器：divisor != -1 → 与当前权重做 GCD
//
// 为什么需要 GCD？
//
//	GCD 决定了 currentWeight 的递减步长。
//	步长 = GCD 能保证每次递减后，权重能被 GCD 整除的服务器分布规律保持不变，
//	从而实现比例精确的平滑交替。
//	如果不用 GCD，某些权重组合会导致分配比例偏差。
//
// 示例：servers = [{name: "A", weight: 6}, {name: "B", weight: 4}, {name: "C", weight: 2}]
//
//	divisor 初始 = -1
//	  遍历 A: divisor == -1  → divisor = 6
//	  遍历 B: divisor != -1  → divisor = gcd(6, 4) = 2
//	  遍历 C: divisor != -1  → divisor = gcd(2, 2) = 2
//
//	最终 divisor = 2
//
//	朴素轮询（无 GCD）：每个服务器连续分配完自己的份额
//	  A A A A A A  B B B B  C C  ← C 排在最后，长时间无请求，流量不均衡
//
//	GCD 平滑轮询（步长=2）：服务器交替出现
//	  A B A C B A  A B A C B A  ← A/B/C 均匀穿插，每轮 6 次选 C 被选 1 次
//
// 核心效果：
//   - GCD 决定了粒度：权重 6/4/2 → GCD=2 → 等价于 A=3份、B=2份、C=1份的细粒度交替
//   - 避免 consecutive burst：不会出现同一个后端连续被命中多次
//   - 不需要预建迭代器：每次选择都是 O(n) 的即时计算，权重动态变化后无需重建数据结构
func (w *WRRLoadBalancer) weightGcd() int {
	divisor := -1 // 初始化为 -1 作为"未初始化"标记
	for _, s := range w.servers {
		// if divisor == -1 为 true：
		//   首次遍历，还没有建立公约数 → 直接用第一个服务器的权重作为初始公约数
		//   （gcd(x, x) = x，所以这样做是安全的）
		//
		// if divisor == -1 为 false（即 divisor >= 0）：
		//   已经初始化过公约数 → 计算当前公约数与当前服务器权重的最大公约数
		if divisor == -1 {
			divisor = s.wight
		} else {
			divisor = gcd(divisor, s.wight)
		}
	}
	return divisor
}

// gcd 使用欧几里得算法（辗转相除法）计算 a 和 b 的最大公约数。
//
// 算法原理：gcd(a, b) = gcd(b, a mod b)，直到 b 为 0，此时 a 即为最大公约数。
//
// 时间复杂度：O(log(min(a, b)))，非常高效。
//
// 举例：
//
//	gcd(6, 4):  6%4=2 → (4, 2) → 4%2=0 → (2, 0) → 返回 2
//	gcd(12, 8): 12%8=4 → (8, 4) → 8%4=0 → (4, 0) → 返回 4
//	gcd(7, 5):  7%5=2 → (5, 2) → 5%2=1 → (2, 1) → 2%1=0 → (1, 0) → 返回 1（互质）
func gcd(a, b int) int {
	// for b != 0：只要 b 不为 0 就继续计算
	//   循环结束条件 b == 0，此时 a 即为最大公约数
	for b != 0 {
		// Go 的多元赋值使得辗转相除特别简洁：
		//   a ← b (将 b 赋给 a)
		//   b ← a%b (将 a 对 b 取余的结果赋给 b)
		// 注意：这里的 a%b 使用的是旧值 a，因为赋值是从右到左先求值的
		a, b = b, a%b
	}
	// b == 0 时，a 就是最大公约数
	return a
}
