// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

type Server struct {
	// Root is the full path name to the directory that Server will read
	// files from and write files to.
	Root string

	// Addr is the address the server will listen on for new Read and Write
	// requests. The default value is ":tftp".
	Addr string

	// ErrorLog specifies an optional logger for errors setting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger // Go 1.3

	// requestReader listens on Addr for new Read and Write requests.
	requestReader *Conn

	// numActiveConnections is used to wait for connections to finish
	// during graceful shutdown. It is incremented upon receiving a
	// new request packet, and decremented after a client connection
	// closes.
	numActiveConns int
}

func NewServer(root, addr string, errorLog *log.Logger) *Server {
	srv := &Server{
		Root:           root,
		Addr:           addr,
		ErrorLog:       errorLog,
		numActiveConns: 0,
	}
	return srv
}

// setup prepares a new server value for use by:
// - changing its Root directory
// - setting up a connection to listen for requests on its address
// - writing an initial log statement.
func (srv *Server) setup() error { // setup() is an instance of Sequential coupling...
	err := srv.changeDir()
	if err != nil {
		return err
	}

	err = srv.setupRequestReader()
	if err != nil {
		return err
	}

	srv.initializeLogger()

	return nil
}

func (srv *Server) changeDir() error {
	return os.Chdir(srv.Root)
}

func (srv *Server) setupRequestReader() error {
	conn, err := NewConn(srv.Addr)
	if err != nil {
		return err
	}
	srv.requestReader = conn
	return nil
}

func (srv *Server) initializeLogger() {
	srv.logf("tftp: starting server...\n\tRoot:\t%v\n\tRoot:\t%v", srv.Root, srv.Addr)
}

func (srv *Server) Serve(cancelChan <-chan CancelType) <-chan error {
	// create a channel to send errors back to caller (so that this routine can be cancelled)
	done := make(chan error)
	go func() {
		err := srv.setup()
		if err != nil {
			done <- err
			return
		}
		ctxSrv := context.WithValue(context.Background(), LoggerContextKey, srv.ErrorLog)
		ctxSrv, cancelServer := context.WithCancel(ctxSrv)
		requests := srv.requestReader.ReadContinuously(ctxSrv)
		connDone := make(chan error)
		for {
			select {
			case request := <-requests:
				srv.logf("tftp: new request received:\n\tfrom: %v\n\tdata: %v\n", request.from, request.data)
				srv.numActiveConns++
				go HandleRequest(ctxSrv, request, connDone)
			case err := <-connDone:
				if err != nil {
					srv.logf("tftp: connection finished with error - %v\n", err)
				}
				srv.numActiveConns--
			case cancelType := <-cancelChan:
				cancelServer()
				done <- srv.cancel(cancelType)
				return
			}
		}
	}()
	return done
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type CloseType error

var (
	ShutdownGracefully  CloseType = errors.New("ShutdownGracefully")
	ShutdownWithTimeout CloseType = errors.New("ShutdownWithTimeout")
	ShutdownImmediately CloseType = errors.New("ShutdownImmediately")
)

type CancelType struct {
	CloseType
	time.Duration
}

func Cancellation(closeType CloseType, duration time.Duration) CancelType {
	return CancelType{
		CloseType: closeType,
		Duration:  duration,
	}
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

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

var (
	// ServerContextKey is a context key. It can be used in TFTP
	// handlers with context.WithValue to access the server's
	// logger. The associated value will be of
	// type *log.Logger.
	LoggerContextKey = &contextKey{"tftp-logger"}
)
