package tftp

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Server struct {
	// ErrorLog specifies an optional logger for errors setting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger // Go 1.3

	// numActiveConnections is used to wait for connections to finish
	// during graceful shutdown.
	numActiveConns int
}

func (srv *Server) Serve(cancelChan <-chan CancelType) <-chan error {
	// create a channel to send errors back to caller (so that this routine can be cancelled)
	done := make(chan error)
	go func() {
		ctxSrv, cancelRequestReader := context.WithCancel(context.Background())
		requestReader, err := NewConn(":tftp")
		if err != nil {
			done <- err
			return
		}
		requests := requestReader.ReadContinuously(ctxSrv)
		connDone := make(chan error)
		for {
			select {
			case request := <-requests:
				go HandleRequest(ctxSrv, request, connDone)
				srv.numActiveConns++
			case err := <-connDone:
				if err != nil {
					srv.logf("tftp: connection finished with error - %v\n", err)
				}
				srv.numActiveConns--
			case cancelType := <-cancelChan:
				cancelRequestReader()
				done <- srv.cancel(cancelType)
				return
			}
		}
	}()
	return done
}

func (srv *Server) cancel(cancel CancelType) error {
	switch cancel.CloseType {
	case ShutdownGracefully:
		return srv.shutdown()
	case ShutdownWithTimeout:
		return srv.shutdown(cancel.Duration)
	case ShutdownImmediately:
		return srv.close()
	default:
		return fmt.Errorf("tftp: Server shutdown with unexpected CancelType: %T\t%v\n", cancel, cancel)
	}
}

func (srv *Server) shutdown(timeout ...time.Duration) error {
	/* TODO Implement */
	return nil
}

func (srv *Server) close() error {
	/* TODO Implement */
	return nil
}

func (srv *Server) logf(format string, args ...interface{}) {
	if srv.ErrorLog != nil {
		srv.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}
