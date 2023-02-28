package tnet

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/timzzx/tnet/types"
)

type Server struct {
	Name        string
	Port        string
	Connections map[string]types.Connection
	Handlers    map[int]types.Handler

	mu sync.Mutex
}

func NewServer() *Server {
	return &Server{
		Name:        "tcp",
		Port:        "9999",
		Connections: make(map[string]types.Connection),
		Handlers:    make(map[int]types.Handler),
	}
}

func (s *Server) Start() {
	listen, err := net.Listen("tcp", "0.0.0.0:"+s.Port)
	if err != nil {
		fmt.Println("监听失败：", err)
		return
	}

	fmt.Println("TCP服务启动成功...")

	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("建立连接失败", err)
			continue
		}
		fmt.Println("连接建立成功")

		// 获取用户id
		uid := conn.RemoteAddr().String()

		ctx, cancel := context.WithCancel(context.Background())
		agent := NewConnection(uid, conn, ctx, cancel)
		// 连接加入全局
		s.addConnections(uid, agent)

		// 逻辑控制
		go s.proceess(agent)

		// 心跳
		go s.heartbeat(agent)
		// 监听连接关闭
		select {
		case <-ctx.Done():
			s.claer(agent)
		}

	}
}

// 逻辑控制
func (s *Server) proceess(agent types.Connection) {
	defer agent.Cancel()
	for {
		// 获取消息
		routerID, data, err := Unpack(agent.GetConn())
		if err != nil {
			fmt.Println("消息解析失败：", err)
			return
		}

		// 处理心跳
		if routerID != 0 {
			// 根据路由id调用处理逻辑
			go s.doHandler(routerID, data, agent)
		}

	}
}

// 心跳
func (s *Server) heartbeat(agent types.Connection) {

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			msg, _ := Pack(0, nil)
			_, err := agent.Send(msg)
			if err != nil {
				s.claer(agent)
				return
			}
		}
	}
}

// 清除
func (s *Server) claer(agent types.Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent.GetConn().Close()
	delete(s.Connections, agent.GetUid())
}

// 根据路由调用处理逻辑
func (s *Server) doHandler(id int, data []byte, agent types.Connection) {
	s.Handlers[id].Do(data, agent)
}

// 连接加入全局
func (s *Server) addConnections(uid string, agent types.Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Connections[uid] = agent

}

// 添加handler
func (s *Server) AddHandlers(id int, handler types.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Handlers[id] = handler
}

func (s *Server) Stop() {
	for _, agent := range s.Connections {
		s.claer(agent)
	}
	fmt.Println("关闭服务器...")
}
