package wpserv

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/quic-go/quic-go/http3"
)

type Server struct {
	httpServer      *http.Server
	httpsServer     *http.Server
	http3Server     *http3.Server
	PortHTTP        int
	PortHTTPS       int
	PortHTTP3       int
	ListenAddress   string
	patterns        map[string]map[string][]http.Handler
	patternsMTLS    map[string]map[string][]http.Handler
	init            bool
	initMTLS        bool
	mutex           sync.Mutex
	CaCertPath      string
	CertPath        string
	KeyPath         string
	ShutdownTimeout int
	EnableMTLS      bool
	ServerMux       *http.ServeMux
	ServerMuxMTLS   *http.ServeMux
}

func NewServer(server *Server) *Server {

	return &Server{
		PortHTTP:        server.PortHTTP,
		PortHTTPS:       server.PortHTTPS,
		PortHTTP3:       server.PortHTTP3,
		ListenAddress:   server.ListenAddress,
		patterns:        make(map[string]map[string][]http.Handler),
		patternsMTLS:    make(map[string]map[string][]http.Handler),
		mutex:           sync.Mutex{},
		CaCertPath:      server.CaCertPath,
		CertPath:        server.CertPath,
		KeyPath:         server.KeyPath,
		ShutdownTimeout: server.ShutdownTimeout,
		EnableMTLS:      server.EnableMTLS,
		ServerMux:       http.NewServeMux(),
		ServerMuxMTLS:   http.NewServeMux(),
	}
}

func (s *Server) Start() error {
	s.initRouter()
	var err error

	mtlsOpt := &tls.Config{}
	if s.EnableMTLS {
		caCertFile, err := os.ReadFile(s.CaCertPath)
		if err != nil {
			log.Fatalf("error reading CA certificate: %v", err)
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCertFile)

		var allowedChipers []uint16
		cs := tls.CipherSuites()
		for _, val := range cs {
			allowedChipers = append(allowedChipers, val.ID)
		}

		mtlsOpt.ClientAuth = tls.RequireAndVerifyClientCert
		mtlsOpt.MinVersion = tls.VersionTLS12
		mtlsOpt.ClientCAs = certPool
		mtlsOpt.CipherSuites = allowedChipers

	}

	if s.PortHTTP3 > 0 {
		s.http3Server = &http3.Server{
			Addr:      fmt.Sprintf("%s:%d", s.ListenAddress, s.PortHTTP3),
			Handler:   s.ServerMux,
			TLSConfig: mtlsOpt,
		}
		go func() {
			err = s.ListenAndServeHttp3(s.CertPath, s.KeyPath)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	if s.PortHTTP > 0 {
		s.httpServer = &http.Server{
			Addr:      fmt.Sprintf("%s:%d", s.ListenAddress, s.PortHTTP),
			TLSConfig: mtlsOpt,
			Handler:   s.ServerMux,
		}

		if s.http3Server != nil {
			s.httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err = s.http3Server.SetQuicHeaders(w.Header())
				if err != nil {
					log.Fatal(err)
				}
				s.ServerMux.ServeHTTP(w, r)
			})
		}

		go func() {
			err = s.httpServer.ListenAndServe()
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	if s.PortHTTPS > 0 {
		s.httpsServer = &http.Server{
			Addr:      fmt.Sprintf("%s:%d", s.ListenAddress, s.PortHTTPS),
			TLSConfig: mtlsOpt,
		}

		s.httpsServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.http3Server != nil {
				err = s.http3Server.SetQuicHeaders(w.Header())
				if err != nil {
					log.Fatal(err)
				}
			}
			if s.EnableMTLS {
				if len(r.TLS.PeerCertificates) > 0 {
					r.Header.Set("user", r.TLS.PeerCertificates[0].Subject.CommonName)
				}
				s.ServerMuxMTLS.ServeHTTP(w, r)
			} else {
				s.ServerMux.ServeHTTP(w, r)
			}
		})

		go func() {
			err = s.httpsServer.ListenAndServeTLS(s.CertPath, s.KeyPath)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	return err
}

func (s *Server) Stop() error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.ShutdownTimeout)*time.Second)
	defer cancel()

	var err error

	if s.httpServer != nil {
		err = s.httpServer.Shutdown(ctx)
	}

	if s.httpsServer != nil {
		err = s.httpsServer.Shutdown(ctx)
	}

	if s.http3Server != nil {
		err = s.http3Server.CloseGracefully(time.Duration(s.ShutdownTimeout) * time.Second)
	}

	return err
}

func (s *Server) initRouter() {

	s.mutex.Lock()
	if s.init {
		return
	}
	s.init = true

	if _, ok := s.patterns["/"]; !ok {
		s.AddRouter(http.MethodOptions, "/", notFound)
	}

	for k := range s.patterns {
		s.ServerMux.Handle(k, s.getHandlers(false))
	}

	if s.EnableMTLS {
		if _, ok := s.patternsMTLS["/"]; !ok {
			s.AddRouter(http.MethodOptions, "/", true, notFound)
		}
		for k := range s.patternsMTLS {
			s.ServerMuxMTLS.Handle(k, s.getHandlers(true))
		}
	}

	s.mutex.Unlock()
}

/*
AddRouter method accepts pattern, request type and chain handlers.
When specifying a pattern, you must ensure that there are no duplicates, otherwise the service will not start.
server.AddRouter
/test/:id/*file and /test/:name/*path are considered duplicates.
*/
func (s *Server) AddRouter(method, pattern string, handlers ...interface{}) {

	hs := make([]http.Handler, 0)
	var mtls bool

	for _, handler := range handlers {
		switch t := handler.(type) {
		case http.Handler:
			hs = append(hs, t)
		case func(http.ResponseWriter, *http.Request):
			hs = append(hs, http.HandlerFunc(t))
		case bool:
			if handler == true && s.EnableMTLS {
				mtls = true
			}
		default:
			panic(fmt.Errorf("error handler type %v\n", t))
		}
	}

	if err := s.checkPattern(method, pattern, mtls); err != nil {
		log.Fatal(err)
	}

	if mtls {
		_, ok := s.patternsMTLS[pattern]
		if !ok {
			m := make(map[string][]http.Handler)
			m[method] = hs
			s.patternsMTLS[pattern] = m
			return
		}

		s.patternsMTLS[pattern][method] = hs
		return
	}

	_, ok := s.patterns[pattern]
	if !ok {
		m := make(map[string][]http.Handler)
		m[method] = hs
		s.patterns[pattern] = m
		return
	}

	s.patterns[pattern][method] = hs
}

func (s *Server) getHandlers(mtls bool) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		patterns := s.patterns

		if mtls {
			patterns = s.patternsMTLS
		}

		BreakConn(r, false)

		if vh, ok := patterns[r.URL.Path][r.Method]; ok {
			for _, h := range vh {
				if checkBreak(r) {
					return
				}
				h.ServeHTTP(w, r)
			}
			return
		}

		p := s.searchPattern(r.URL.Path, mtls)
		if p == "" {
			SendMsg(w, "not found", http.StatusNotFound)
			return
		}

		params := getParseUrl(r.URL.Path)
		for i, elem := range getParseUrl(p) {
			if elem == "" {
				continue
			}

			if elem[0] == ':' {
				setParam(r, elem[1:], params[i])
				continue
			}

			if elem[0] == '*' {
				setParam(r, elem[1:], filepath.Join(params[i:]...))
				break
			}
		}

		setParam(r, "pattern", p)

		handlers, ok := patterns[p][r.Method]
		if !ok {
			SendMsg(w, "not found", http.StatusNotFound)
			return
		}

		for _, h := range handlers {
			if checkBreak(r) {
				return
			}
			h.ServeHTTP(w, r)
		}
	}
}

func setParam(r *http.Request, key, value string) {
	*r = *r.WithContext(context.WithValue(r.Context(), key, value))
}

func GetParam(r *http.Request, key string) string {
	v, ok := r.Context().Value(key).(string)
	if !ok {
		return ""
	}
	return v
}

func SendMsg(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

func BreakConn(r *http.Request, interrupt bool) {
	*r = *r.WithContext(context.WithValue(r.Context(), "break", interrupt))
}

func checkBreak(r *http.Request) bool {
	flag, ok := r.Context().Value("break").(bool)
	if !ok {
		return false
	}
	return flag
}

func (s *Server) ReloadTLS() error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	for range c {
		cert, err := tls.LoadX509KeyPair(s.CertPath, s.KeyPath)
		if err != nil {
			return err
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if s.httpsServer != nil {
			s.httpsServer.TLSConfig = tlsConfig
		}

		if s.http3Server != nil {
			s.httpsServer.TLSConfig = tlsConfig
		}

	}
	return nil
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) ListenAndServeHttp3(certFile, keyFile string) error {

	var err error
	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	//s.http3Server.TLSConfig.Certificates = certs

	udpAddr, err := net.ResolveUDPAddr("udp", s.http3Server.Addr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpConn.Close()

	s.http3Server.Handler = s.ServerMux
	hErr := make(chan error)
	qErr := make(chan error)
	go func() {
		hErr <- http.ListenAndServeTLS(s.http3Server.Addr, certFile, keyFile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := s.http3Server.SetQuicHeaders(w.Header()); err != nil {
				log.Fatal(err)
			}
			s.http3Server.Handler.ServeHTTP(w, r)
		}))
	}()
	go func() {
		qErr <- s.http3Server.Serve(udpConn)
	}()

	select {
	case err := <-hErr:
		s.http3Server.Close()
		return err
	case err := <-qErr:
		return err
	}
}

func test() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	}

}
