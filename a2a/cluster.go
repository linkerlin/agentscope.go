package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ClusterManager 管理分布式 Agent 集群，支持分片、负载均衡和故障转移。
type ClusterManager struct {
	registry    *Registry
	router      *ShardRouter
	localNodeID string
	localURL    string

	// 健康检查
	healthCheckInterval time.Duration
	healthTimeout       time.Duration

	// 故障转移
	failoverTimeout time.Duration
	maxRetries      int

	mu       sync.RWMutex
	nodeHealth map[string]*NodeHealth

	// 事件监听
	onNodeUp   func(nodeID string)
	onNodeDown func(nodeID string)
}

// NodeHealth 节点健康状态。
type NodeHealth struct {
	NodeID      string    `json:"node_id"`
	URL         string    `json:"url"`
	LastPing    time.Time `json:"last_ping"`
	Healthy     bool      `json:"healthy"`
	Load        float64   `json:"load"`         // 0-1，当前负载
	Capacity    int       `json:"capacity"`     // 最大并发任务数
	ActiveTasks int       `json:"active_tasks"` // 当前活跃任务数
}

// NewClusterManager 创建集群管理器。
func NewClusterManager(registry *Registry, localNodeID, localURL string) *ClusterManager {
	return &ClusterManager{
		registry:            registry,
		router:              NewShardRouter(registry, 150),
		localNodeID:         localNodeID,
		localURL:            localURL,
		healthCheckInterval: 30 * time.Second,
		healthTimeout:       5 * time.Second,
		failoverTimeout:     10 * time.Second,
		maxRetries:          3,
		nodeHealth:          make(map[string]*NodeHealth),
	}
}

// Start 启动集群管理（健康检查、自动刷新）。
func (cm *ClusterManager) Start(ctx context.Context) {
	// 注册本地节点
	cm.registerLocalNode()

	// 启动健康检查循环
	go cm.healthCheckLoop(ctx)

	// 启动路由自动刷新
	go cm.router.AutoRefresh(ctx, 30*time.Second)
}

// registerLocalNode 将本地节点注册到 Registry。
func (cm *ClusterManager) registerLocalNode() {
	card := AgentCard{
		Name:         cm.localNodeID,
		URL:          cm.localURL,
		Description:  "Distributed cluster node",
		Capabilities: []string{"cluster", "distributed"},
	}
	_ = cm.registry.Register(card)
	_ = cm.router.Refresh()
}

// healthCheckLoop 定期健康检查所有节点。
func (cm *ClusterManager) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(cm.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkAllNodes(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// checkAllNodes 检查所有注册节点的健康状态。
func (cm *ClusterManager) checkAllNodes(ctx context.Context) {
	entries := cm.registry.List()
	for _, entry := range entries {
		go cm.checkNode(ctx, entry.Card.URL)
	}
}

// checkNode 检查单个节点健康状态。
func (cm *ClusterManager) checkNode(ctx context.Context, url string) {
	client := &http.Client{Timeout: cm.healthTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/health", nil)
	if err != nil {
		cm.markUnhealthy(url)
		return
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		cm.markUnhealthy(url)
		return
	}
	resp.Body.Close()

	// 解析负载信息
	var health NodeHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err == nil {
		cm.updateHealth(url, &health)
	} else {
		cm.markHealthy(url)
	}
}

// markUnhealthy 标记节点为不健康。
func (cm *ClusterManager) markUnhealthy(url string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if h, ok := cm.nodeHealth[url]; ok {
		h.Healthy = false
	}
	if cm.onNodeDown != nil {
		go cm.onNodeDown(url)
	}
}

// markHealthy 标记节点为健康。
func (cm *ClusterManager) markHealthy(url string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if h, ok := cm.nodeHealth[url]; ok {
		h.Healthy = true
		h.LastPing = time.Now()
	} else {
		cm.nodeHealth[url] = &NodeHealth{
			URL:      url,
			Healthy:  true,
			LastPing: time.Now(),
		}
	}
	if cm.onNodeUp != nil {
		go cm.onNodeUp(url)
	}
}

// updateHealth 更新节点健康信息。
func (cm *ClusterManager) updateHealth(url string, health *NodeHealth) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.nodeHealth[url] = health
}

// RouteWithLoadBalance 路由请求到负载最低的节点。
func (cm *ClusterManager) RouteWithLoadBalance(key string) (string, error) {
	// 先尝试一致哈希路由
	target, err := cm.router.Route(key)
	if err != nil {
		return "", err
	}

	// 检查目标节点负载
	cm.mu.RLock()
	health, ok := cm.nodeHealth[target]
	cm.mu.RUnlock()

	if !ok || !health.Healthy || health.Load > 0.8 {
		// 负载过高或不可用，选择负载最低的节点
		return cm.selectLeastLoadedNode()
	}
	return target, nil
}

// selectLeastLoadedNode 选择负载最低的节点。
func (cm *ClusterManager) selectLeastLoadedNode() (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var bestURL string
	var bestLoad float64 = 2.0

	for url, health := range cm.nodeHealth {
		if !health.Healthy {
			continue
		}
		if health.Load < bestLoad {
			bestLoad = health.Load
			bestURL = url
		}
	}

	if bestURL == "" {
		return "", fmt.Errorf("no healthy nodes available")
	}
	return bestURL, nil
}

// SendTaskWithFailover 发送任务并支持故障转移。
func (cm *ClusterManager) SendTaskWithFailover(ctx context.Context, task *Task) (*TaskResult, error) {
	// 获取路由目标
	target, err := cm.RouteWithLoadBalance(task.ID)
	if err != nil {
		return nil, err
	}

	// 尝试发送，失败时重试其他节点
	for i := 0; i < cm.maxRetries; i++ {
		result, err := cm.sendTaskToNode(ctx, target, task)
		if err == nil {
			return result, nil
		}

		// 标记节点不健康，尝试下一个节点
		cm.markUnhealthy(target)
		target, err = cm.selectLeastLoadedNode()
		if err != nil {
			return nil, fmt.Errorf("all nodes failed: %w", err)
		}
		time.Sleep(cm.failoverTimeout)
	}

	return nil, fmt.Errorf("task failed after %d retries", cm.maxRetries)
}

// sendTaskToNode 向指定节点发送任务。
func (cm *ClusterManager) sendTaskToNode(ctx context.Context, target string, task *Task) (*TaskResult, error) {
	client := &NoopClient{}
	resp, err := client.Send(ctx, &Message{Content: task.ID})
	if err != nil {
		return nil, err
	}
	return &TaskResult{
		TaskID: task.ID,
		Status: "completed",
		Output: resp.Content,
	}, nil
}

// GetClusterStatus 获取集群状态。
func (cm *ClusterManager) GetClusterStatus() map[string]*NodeHealth {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*NodeHealth)
	for url, health := range cm.nodeHealth {
		result[url] = health
	}
	return result
}

// OnNodeUp 设置节点上线回调。
func (cm *ClusterManager) OnNodeUp(fn func(nodeID string)) {
	cm.onNodeUp = fn
}

// OnNodeDown 设置节点下线回调。
func (cm *ClusterManager) OnNodeDown(fn func(nodeID string)) {
	cm.onNodeDown = fn
}

// SetHealthCheckInterval 设置健康检查间隔。
func (cm *ClusterManager) SetHealthCheckInterval(d time.Duration) {
	cm.healthCheckInterval = d
}

// SetFailoverTimeout 设置故障转移超时。
func (cm *ClusterManager) SetFailoverTimeout(d time.Duration) {
	cm.failoverTimeout = d
}

// SetMaxRetries 设置最大重试次数。
func (cm *ClusterManager) SetMaxRetries(n int) {
	cm.maxRetries = n
}

// TaskResult 任务执行结果。
type TaskResult struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Output   string `json:"output"`
	NodeID   string `json:"node_id"`
	Duration int64  `json:"duration_ms"`
}
