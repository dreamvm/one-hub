package requester

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"one-api/common/logger"
	"one-api/types"
)

type WSProxy struct {
	userConn       *websocket.Conn
	supplierConn   *websocket.Conn
	timeout        time.Duration
	handler        MessageHandler
	usageHandler   UsageHandler
	workers        sync.WaitGroup
	closeOnce      sync.Once
	userClosed     chan struct{}
	supplierClosed chan struct{}
}

type MessageSource int

const (
	UserMessage MessageSource = iota
	SupplierMessage
)

type MessageHandler func(source MessageSource, messageType int, message []byte) (bool, *types.UsageEvent, []byte, error)
type UsageHandler func(usage *types.UsageEvent) error

func NewWSProxy(userConn, supplierConn *websocket.Conn, timeout time.Duration, handler MessageHandler, usageHandler UsageHandler) *WSProxy {
	return &WSProxy{
		userConn:       userConn,
		supplierConn:   supplierConn,
		timeout:        timeout,
		handler:        handler,
		usageHandler:   usageHandler,
		userClosed:     make(chan struct{}),
		supplierClosed: make(chan struct{}),
	}
}

func (p *WSProxy) Start() {
	p.workers.Add(2)
	go p.transfer(p.userConn, p.supplierConn, UserMessage, p.userClosed)
	go p.transfer(p.supplierConn, p.userConn, SupplierMessage, p.supplierClosed)
}

func (p *WSProxy) Wait() {
	p.workers.Wait()
}

func (p *WSProxy) Close() {
	p.closeOnce.Do(func() {
		p.userConn.Close()
		p.supplierConn.Close()
	})
}

func (p *WSProxy) UserClosed() <-chan struct{} {
	return p.userClosed
}

func (p *WSProxy) SupplierClosed() <-chan struct{} {
	return p.supplierClosed
}

func (p *WSProxy) transfer(src, dst *websocket.Conn, source MessageSource, closed chan<- struct{}) {
	defer func() {
		close(closed)
		p.Close()
		p.workers.Done()
	}()

	for {
		src.SetReadDeadline(time.Now().Add(p.timeout))

		messageType, message, err := src.ReadMessage()
		if err != nil {
			logger.SysError(fmt.Sprintf("source: %d, ReadMessage error: %s", source, err.Error()))
			return
		}

		dst.SetWriteDeadline(time.Now().Add(p.timeout))
		if p.handler != nil {
			shouldContinue, usage, newMessage, err := p.handler(source, messageType, message)
			// A rejected completion can still prove that accounting is unknown.
			// Preserve its error, but retain the reservation before handling it.
			if usage != nil && (usage.MissingUsage || (usage.ResponseStarted && err != nil)) && p.usageHandler != nil {
				usageErr := p.usageHandler(usage)
				usage = nil
				if usageErr != nil {
					if err == nil {
						err = usageErr
					}
				}
				if err != nil {
					p.supplierConn.Close()
				}
			}
			if err != nil {
				errMsg := []byte(err.Error())
				dst.WriteMessage(websocket.TextMessage, errMsg)
				logger.SysError(fmt.Sprintf("source: %d, handler error: %s", source, err.Error()))
				return
			}

			if !shouldContinue {
				return
			}

			if newMessage != nil {
				message = newMessage
			}

			if usage != nil && p.usageHandler != nil {
				err := p.usageHandler(usage)
				if err != nil {
					// Stop further upstream work immediately; client delivery
					// may block on backpressure until its write deadline.
					p.supplierConn.Close()
					dst.WriteMessage(websocket.TextMessage, message)
					errMsg := []byte(err.Error())
					dst.WriteMessage(websocket.TextMessage, errMsg)
					logger.SysError(fmt.Sprintf("source: %d, usageHandler error: %s", source, err.Error()))
					return
				}
			}
		}

		err = dst.WriteMessage(messageType, message)
		if err != nil {
			logger.SysError(fmt.Sprintf("source: %d, WriteMessage error: %s", source, err.Error()))
			return
		}
	}
}
