package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketUpgrader WebSocket 升级器（默认允许所有来源，生产环境请调用 SetWebSocketAllowedOrigins 限制）
var WebSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     defaultCheckOrigin,
}

// defaultAllowedOrigins 为 nil 时允许所有来源（开发模式）。
var defaultAllowedOrigins []string

// defaultCheckOrigin 默认来源检查。
// 若设置了 AllowedOrigins，只允许列表中的来源；否则允许所有来源（向后兼容）。
func defaultCheckOrigin(r *http.Request) bool {
	if len(defaultAllowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // 非浏览器客户端无 Origin 头
	}
	for _, allowed := range defaultAllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

// SetWebSocketAllowedOrigins 设置全局 WebSocket 允许的来源列表。
// 设置后，WebSocketUpgrader 将只接受来自这些来源的连接。
// 传空切片恢复默认行为（允许所有来源）。
func SetWebSocketAllowedOrigins(origins []string) {
	defaultAllowedOrigins = origins
}

// NewWebSocketUpgrader 创建带来源限制的 WebSocket 升级器。
func NewWebSocketUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}
}

// WebSocketClient WebSocket 客户端连接
type WebSocketClient struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan []byte
	Server   *WebSocketServer
	UserName string
	mu       sync.RWMutex
}

// WebSocketMessage WebSocket 消息格式
type WebSocketMessage struct {
	Type      string    `json:"type"`
	TaskID    string    `json:"task_id,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// WebSocketServer WebSocket 服务器
type WebSocketServer struct {
	clients    map[string]*WebSocketClient
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	broadcast  chan WebSocketMessage
	mu         sync.RWMutex
	upgrader   websocket.Upgrader
	server     *Server
	// 任务状态推送映射
	taskClients map[string]map[string]bool // taskID -> clientIDs
}

// NewWebSocketServer 创建 WebSocket 服务器
func NewWebSocketServer(a2aServer *Server) *WebSocketServer {
	ws := &WebSocketServer{
		clients:     make(map[string]*WebSocketClient),
		register:    make(chan *WebSocketClient),
		unregister:  make(chan *WebSocketClient),
		broadcast:   make(chan WebSocketMessage, 256),
		upgrader:    WebSocketUpgrader,
		server:      a2aServer,
		taskClients: make(map[string]map[string]bool),
	}
	go ws.run()
	return ws
}

// run 主循环
func (ws *WebSocketServer) run() {
	for {
		select {
		case client := <-ws.register:
			ws.mu.Lock()
			ws.clients[client.ID] = client
			ws.mu.Unlock()
			fmt.Printf("[A2A WS] Client connected: %s\n", client.ID)

		case client := <-ws.unregister:
			ws.mu.Lock()
			if _, ok := ws.clients[client.ID]; ok {
				delete(ws.clients, client.ID)
				close(client.Send)
				// 从所有任务订阅中移除
				for taskID, clients := range ws.taskClients {
					delete(clients, client.ID)
					if len(clients) == 0 {
						delete(ws.taskClients, taskID)
					}
				}
			}
			ws.mu.Unlock()
			fmt.Printf("[A2A WS] Client disconnected: %s\n", client.ID)

		case message := <-ws.broadcast:
			ws.mu.RLock()
			clients := make([]*WebSocketClient, 0, len(ws.clients))
			for _, client := range ws.clients {
				clients = append(clients, client)
			}
			ws.mu.RUnlock()

			data, _ := json.Marshal(message)
			for _, client := range clients {
				select {
				case client.Send <- data:
				default:
					// 客户端发送缓冲满，关闭连接
					ws.unregister <- client
				}
			}
		}
	}
}

// HandleWebSocket 处理 WebSocket 连接升级
func (ws *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("[A2A WS] Upgrade failed: %v\n", err)
		return
	}

	client := &WebSocketClient{
		ID:     fmt.Sprintf("ws-%d", time.Now().UnixNano()),
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Server: ws,
	}

	ws.register <- client

	// 启动读写 goroutine
	go client.writePump()
	go client.readPump()
}

// readPump 读取客户端消息
func (c *WebSocketClient) readPump() {
	defer func() {
		c.Server.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024) // 512KB
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("[A2A WS] Read error: %v\n", err)
			}
			break
		}

		// 解析消息
		var wsMsg WebSocketMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			c.sendError("invalid_message", err.Error())
			continue
		}

		// 处理消息
		c.handleMessage(wsMsg)
	}
}

// writePump 向客户端发送消息
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端消息
func (c *WebSocketClient) handleMessage(msg WebSocketMessage) {
	switch msg.Type {
	case "subscribe_task":
		// 订阅任务更新
		if taskID, ok := msg.Data.(string); ok && taskID != "" {
			c.Server.mu.Lock()
			if c.Server.taskClients[taskID] == nil {
				c.Server.taskClients[taskID] = make(map[string]bool)
			}
			c.Server.taskClients[taskID][c.ID] = true
			c.Server.mu.Unlock()
			c.sendAck("subscribed", taskID)
		}

	case "unsubscribe_task":
		// 取消订阅
		if taskID, ok := msg.Data.(string); ok && taskID != "" {
			c.Server.mu.Lock()
			if clients, ok := c.Server.taskClients[taskID]; ok {
				delete(clients, c.ID)
			}
			c.Server.mu.Unlock()
			c.sendAck("unsubscribed", taskID)
		}

	case "send_task":
		// 通过 WebSocket 发送任务
		if taskData, ok := msg.Data.(map[string]any); ok {
			c.handleTaskSend(taskData)
		}

	case "ping":
		c.sendAck("pong", nil)

	default:
		c.sendError("unknown_type", fmt.Sprintf("unknown message type: %s", msg.Type))
	}
}

// handleTaskSend 处理任务发送
func (c *WebSocketClient) handleTaskSend(taskData map[string]any) {
	// 构造 TaskUpdateRequest
	req := TaskUpdateRequest{
		ID: fmt.Sprintf("%v", taskData["id"]),
	}
	if msgData, ok := taskData["message"].(map[string]any); ok {
		req.Message = &Message{
			Role:    fmt.Sprintf("%v", msgData["role"]),
			Content: fmt.Sprintf("%v", msgData["content"]),
		}
	}

	// 创建任务
	task := &Task{
		ID:        req.ID,
		Status:    TaskStatusSubmitted,
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if req.Message != nil {
		task.Messages = append(task.Messages, *req.Message)
	}
	task.Status = TaskStatusWorking

	// 保存任务
	if c.Server.server != nil && c.Server.server.store != nil {
		_ = c.Server.server.store.Save(task)
	}

	// 订阅此任务
	c.Server.mu.Lock()
	if c.Server.taskClients[task.ID] == nil {
		c.Server.taskClients[task.ID] = make(map[string]bool)
	}
	c.Server.taskClients[task.ID][c.ID] = true
	c.Server.mu.Unlock()

	// 推送任务状态
	c.Server.BroadcastTaskUpdate(task.ID, TaskStatusWorking, nil)

	// 异步执行任务
	go func() {
		if c.Server.server != nil && c.Server.server.runner != nil && req.Message != nil {
			resp, err := c.Server.server.runner.Run(context.Background(), req.Message)
			if err != nil {
				c.Server.BroadcastTaskUpdate(task.ID, TaskStatusFailed, map[string]any{
					"error": err.Error(),
				})
			} else if resp != nil {
				c.Server.BroadcastTaskUpdate(task.ID, TaskStatusCompleted, map[string]any{
					"response": resp.Content,
				})
			} else {
				c.Server.BroadcastTaskUpdate(task.ID, TaskStatusCompleted, nil)
			}
		}
	}()
}

// sendAck 发送确认消息
func (c *WebSocketClient) sendAck(ackType string, data any) {
	msg := WebSocketMessage{
		Type:      ackType,
		Data:      data,
		Timestamp: time.Now(),
	}
	dataBytes, _ := json.Marshal(msg)
	select {
	case c.Send <- dataBytes:
	default:
	}
}

// sendError 发送错误消息
func (c *WebSocketClient) sendError(errType, errMsg string) {
	msg := WebSocketMessage{
		Type:      "error",
		Error:     fmt.Sprintf("%s: %s", errType, errMsg),
		Timestamp: time.Now(),
	}
	dataBytes, _ := json.Marshal(msg)
	select {
	case c.Send <- dataBytes:
	default:
	}
}

// BroadcastTaskUpdate 广播任务状态更新
func (ws *WebSocketServer) BroadcastTaskUpdate(taskID string, status TaskStatus, data map[string]any) {
	ws.mu.RLock()
	clientIDs, ok := ws.taskClients[taskID]
	ws.mu.RUnlock()

	if !ok {
		return
	}

	msg := WebSocketMessage{
		Type:      "task_update",
		TaskID:    taskID,
		Data:      map[string]any{"status": string(status), "data": data},
		Timestamp: time.Now(),
	}
	msgBytes, _ := json.Marshal(msg)

	ws.mu.RLock()
	for clientID := range clientIDs {
		if client, ok := ws.clients[clientID]; ok {
			select {
			case client.Send <- msgBytes:
			default:
			}
		}
	}
	ws.mu.RUnlock()
}

// Broadcast 广播消息给所有客户端
func (ws *WebSocketServer) Broadcast(msg WebSocketMessage) {
	ws.broadcast <- msg
}

// GetClientCount 获取客户端数量
func (ws *WebSocketServer) GetClientCount() int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return len(ws.clients)
}

// GetTaskSubscriberCount 获取任务订阅者数量
func (ws *WebSocketServer) GetTaskSubscriberCount(taskID string) int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	if clients, ok := ws.taskClients[taskID]; ok {
		return len(clients)
	}
	return 0
}

// Close 关闭 WebSocket 服务器
func (ws *WebSocketServer) Close() error {
	ws.mu.Lock()
	for _, client := range ws.clients {
		close(client.Send)
		client.Conn.Close()
	}
	ws.clients = make(map[string]*WebSocketClient)
	ws.mu.Unlock()
	return nil
}

// WebSocketEnabledServer 同时支持 HTTP 和 WebSocket 的 A2A 服务器
type WebSocketEnabledServer struct {
	*SecureServer
	wsServer *WebSocketServer
}

// NewWebSocketEnabledServer 创建支持 WebSocket 的服务器
func NewWebSocketEnabledServer(card AgentCard, runner AgentRunner, store TaskStore) *WebSocketEnabledServer {
	secure := NewSecureServer(card, runner, store)
	base := secure.Server
	ws := NewWebSocketServer(base)

	return &WebSocketEnabledServer{
		SecureServer: secure,
		wsServer:     ws,
	}
}

// ServeHTTP 同时处理 HTTP 和 WebSocket 请求
func (s *WebSocketEnabledServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// WebSocket 升级请求
	if websocket.IsWebSocketUpgrade(r) {
		s.wsServer.HandleWebSocket(w, r)
		return
	}

	// 普通 HTTP 请求
	s.SecureServer.ServeHTTP(w, r)
}

// GetWebSocketServer 获取 WebSocket 服务器
func (s *WebSocketEnabledServer) GetWebSocketServer() *WebSocketServer {
	return s.wsServer
}

// WithWebSocketAuth 配置 WebSocket 认证（通过 query param 或 header）
func (s *WebSocketEnabledServer) WithWebSocketAuth(authFunc func(r *http.Request) (string, error)) *WebSocketEnabledServer {
	// 自定义 CheckOrigin 以支持认证
	s.wsServer.upgrader.CheckOrigin = func(r *http.Request) bool {
		_, err := authFunc(r)
		return err == nil
	}
	return s
}
