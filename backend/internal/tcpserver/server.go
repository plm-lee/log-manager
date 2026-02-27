package tcpserver

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"log-manager/internal/config"
	"log-manager/internal/handler"
)

const defaultPayloadCap = 256 * 1024 // 256KB

var payloadPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, defaultPayloadCap)
		return &b
	},
}

// LogBatchProcessor 批量处理日志的接口，由 LogHandler 实现
type LogBatchProcessor interface {
	ProcessLogBatch(logs []handler.ReceiveLogRequest) (successCount, failedCount int, ids []uint, err error)
}

// Server TCP 日志接收服务
type Server struct {
	cfg       config.TCPConfig
	processor LogBatchProcessor
	listener  net.Listener
	ch        chan handler.ReceiveLogRequest
	stopChan  chan struct{}
	wg        sync.WaitGroup
	flushDur  time.Duration
}

const maxFrameSize = 4 * 1024 * 1024 // 4MB

// Start 启动 TCP 服务
func Start(cfg *config.TCPConfig, processor LogBatchProcessor) (*Server, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	listener, err := net.Listen("tcp", addr)
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
		listener:  listener,
		ch:        make(chan handler.ReceiveLogRequest, cfg.BufferSize),
		stopChan:  make(chan struct{}),
		flushDur:  flushDur,
	}
	s.wg.Add(2)
	go s.acceptLoop()
	go s.consumeLoop()
	log.Printf("[tcp] 日志接收已启动，监听 %s\n", listener.Addr())
	return s, nil
}

// Stop 停止 TCP 服务
func (s *Server) Stop() {
	close(s.stopChan)
	s.listener.Close()
	s.wg.Wait()
	log.Println("[tcp] 日志接收已停止")
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

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				log.Printf("[tcp] Accept 失败: %v\n", err)
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true) // 禁用 Nagle，低延迟
	}
	br := bufio.NewReaderSize(conn, 64*1024) // 64KB 读缓冲
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	for {
		select {
		case <-s.stopChan:
			return
		default:
		}
		// 读取 4 字节长度（大端）
		var lenBuf [4]byte
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			if err != io.EOF {
				log.Printf("[tcp] 读取长度失败: %v\n", err)
			}
			return
		}
		payloadLen := binary.BigEndian.Uint32(lenBuf[:])
		if payloadLen == 0 || payloadLen > maxFrameSize {
			log.Printf("[tcp] 非法帧长度: %d\n", payloadLen)
			return
		}
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		payloadPtr := payloadPool.Get().(*[]byte)
		payload := *payloadPtr
		usedPooled := cap(payload) >= int(payloadLen)
		if usedPooled {
			payload = payload[:payloadLen]
		} else {
			payload = make([]byte, payloadLen)
		}
		if _, err := io.ReadFull(br, payload); err != nil {
			if usedPooled {
				*payloadPtr = (*payloadPtr)[:0]
			}
			payloadPool.Put(payloadPtr)
			log.Printf("[tcp] 读取载荷失败: %v\n", err)
			return
		}

		// 解析 JSON：支持单条或 {"logs": [...]}
		var logs []handler.ReceiveLogRequest
		var single handler.ReceiveLogRequest
		if err := json.Unmarshal(payload, &single); err == nil && single.LogLine != "" && single.Timestamp != 0 {
			logs = []handler.ReceiveLogRequest{single}
		} else {
			var batch struct {
				Logs []handler.ReceiveLogRequest `json:"logs"`
			}
			if err := json.Unmarshal(payload, &batch); err == nil && len(batch.Logs) > 0 {
				logs = batch.Logs
			}
		}
		if usedPooled {
			*payloadPtr = (*payloadPtr)[:0]
		}
		payloadPool.Put(payloadPtr)
		if len(logs) == 0 {
			continue
		}
		for _, req := range logs {
			if req.Timestamp == 0 || req.LogLine == "" {
				continue
			}
			if !s.checkSecret(req) {
				continue
			}
			req.Transport = "tcp"
			select {
			case s.ch <- req:
			case <-s.stopChan:
				return
			}
			// 缓冲满时阻塞等待，避免丢包（TCP 可靠传输）
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
			log.Printf("[tcp] 批量写入失败: %v\n", err)
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
