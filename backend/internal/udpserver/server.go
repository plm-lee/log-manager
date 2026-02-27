package udpserver

import (
	"encoding/json"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"log-manager/internal/config"
	"log-manager/internal/handler"
)

// LogBatchProcessor 批量处理日志的接口，由 LogHandler 实现
type LogBatchProcessor interface {
	ProcessLogBatch(logs []handler.ReceiveLogRequest) (successCount, failedCount int, ids []uint, err error)
}

// Server UDP 日志接收服务
type Server struct {
	cfg       config.UDPConfig
	processor LogBatchProcessor
	conn      *net.UDPConn
	ch        chan handler.ReceiveLogRequest
	stopChan  chan struct{}
	wg        sync.WaitGroup
	flushDur  time.Duration
}

// Start 启动 UDP 服务
func Start(cfg *config.UDPConfig, processor LogBatchProcessor) (*Server, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	flushDur, _ := time.ParseDuration(cfg.FlushInterval)
	if flushDur <= 0 {
		flushDur = 100 * time.Millisecond
	}
	s := &Server{
		cfg:       *cfg,
		processor: processor,
		conn:      conn,
		ch:        make(chan handler.ReceiveLogRequest, cfg.BufferSize),
		stopChan:  make(chan struct{}),
		flushDur:  flushDur,
	}
	s.wg.Add(2)
	go s.recvLoop()
	go s.consumeLoop()
	log.Printf("[udp] 日志接收已启动，监听 %s\n", conn.LocalAddr())
	return s, nil
}

// Stop 停止 UDP 服务
func (s *Server) Stop() {
	close(s.stopChan)
	s.conn.Close()
	s.wg.Wait()
	log.Println("[udp] 日志接收已停止")
}

func (s *Server) checkSecret(req handler.ReceiveLogRequest) bool {
	if s.cfg.Secret == "" {
		return true
	}
	secret := req.Secret
	if secret == "" {
		secret = req.APIKey
	}
	return secret == s.cfg.Secret
}

func (s *Server) recvLoop() {
	defer s.wg.Done()
	buf := make([]byte, 65535)
	for {
		select {
		case <-s.stopChan:
			return
		default:
		}
		s.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.stopChan:
				return
			default:
				continue
			}
		}
		if n <= 0 {
			continue
		}
		var req handler.ReceiveLogRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			continue
		}
		if req.Timestamp == 0 || req.LogLine == "" {
			continue
		}
		if !s.checkSecret(req) {
			continue
		}
		req.Transport = "udp"
		select {
		case s.ch <- req:
		case <-s.stopChan:
			return
		default:
			// 缓冲满，丢弃
		}
	}
}

func (s *Server) consumeLoop() {
	defer s.wg.Done()
	batch := make([]handler.ReceiveLogRequest, 0, s.cfg.FlushSize)
	ticker := time.NewTicker(s.flushDur)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toSend := batch
		batch = make([]handler.ReceiveLogRequest, 0, s.cfg.FlushSize)
		_, _, _, err := s.processor.ProcessLogBatch(toSend)
		if err != nil {
			log.Printf("[udp] 批量写入失败: %v\n", err)
		}
	}

	for {
		select {
		case <-s.stopChan:
			flush()
			return
		case req := <-s.ch:
			batch = append(batch, req)
			if len(batch) >= s.cfg.FlushSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
