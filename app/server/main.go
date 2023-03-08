/*
 * Copyright 2023 OpsMx, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/OpsMx/go-app-base/tracer"
	"github.com/OpsMx/go-app-base/util"
	"github.com/OpsMx/go-app-base/version"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/skandragon/grpc-datacon/internal/ca"
	"github.com/skandragon/grpc-datacon/internal/jwtutil"
	"github.com/skandragon/grpc-datacon/internal/secrets"
	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"github.com/skandragon/grpc-datacon/internal/ulid"
	"google.golang.org/grpc/keepalive"
)

const (
	appName = "agent-server"
)

var (
	configFile = flag.String("configFile", "/app/config/config.yaml", "The file with the controller config")

	// eg, http://localhost:14268/api/traces
	jaegerEndpoint = flag.String("jaeger-endpoint", "", "Jaeger collector endpoint")
	traceToStdout  = flag.Bool("traceToStdout", false, "log traces to stdout")
	traceRatio     = flag.Float64("traceRatio", 0.01, "ratio of traces to create, if incoming request is not traced")
	showversion    = flag.Bool("version", false, "show the version and exit")

	tracerProvider *tracer.TracerProvider

	jwtKeyset     = jwk.NewSet()
	jwtCurrentKey string
	config        *ControllerConfig
	secretsLoader secrets.SecretLoader
	authority     *ca.CA
	//endpoints     []serviceconfig.ConfiguredEndpoint
)

var kaep = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second,
	PermitWithoutStream: true,
}

var kasp = keepalive.ServerParameters{
	MaxConnectionIdle: 20 * time.Minute,
	Time:              10 * time.Second,
	Timeout:           10 * time.Second,
}

// get a random session
func (s *server) getRandomSession() *agentContext {
	s.Lock()
	defer s.Unlock()
	for _, session := range s.agents {
		return session
	}
	return nil
}

// fake a http request to some random agent
func (s *server) randomRequest(ctx context.Context) {
	session := s.getRandomSession()
	if session == nil {
		return
	}
	session.out <- &pb.TunnelRequest{
		StreamId: ulid.GlobalContext.Ulid(),
		Name:     "bobService",
		Type:     "bobServiceType",
		Method:   "GET",
		URI:      "http://blog.flame.org/",
	}
}

func (s *server) requestOnTimer(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.randomRequest(ctx)
		}
	}
}

func healthcheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(200)
	n, err := w.Write([]byte("{}"))
	if err != nil {
		log.Printf("Error writing healthcheck response: %v", err)
		return
	}
	if n != 2 {
		log.Printf("Failed to write 2 bytes: %d written", n)
	}
}

func runPrometheusHTTPServer(port uint16) {
	log.Printf("Running HTTP listener for Prometheus on port %d", port)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", healthcheck)
	mux.HandleFunc("/health", healthcheck)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	log.Fatal(server.ListenAndServe())
}

func loadKeyset() {
	if config.ServiceAuth.CurrentKeyName == "" {
		log.Fatalf("No primary serviceAuth key name provided")
	}

	err := filepath.WalkDir(config.ServiceAuth.SecretsPath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip not refular files
		if !info.Type().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		key, err := jwk.New(content)
		if err != nil {
			return err
		}
		err = key.Set(jwk.KeyIDKey, info.Name())
		if err != nil {
			return err
		}
		err = key.Set(jwk.AlgorithmKey, jwa.HS256)
		if err != nil {
			return err
		}
		jwtKeyset.Add(key)
		log.Printf("Loaded service key name %s, length %d", info.Name(), len(content))
		return nil
	})
	if err != nil {
		log.Fatalf("cannot load key serviceAuth keys: %v", err)
	}

	jwtCurrentKey = config.ServiceAuth.CurrentKeyName
	if len(jwtCurrentKey) == 0 {
		log.Fatal("serviceAuth.currentKeyName is not set")
	}
	if _, found := jwtKeyset.LookupKeyID(jwtCurrentKey); !found {
		log.Fatal("serviceAuth.currentKeyName is not in the loaded list of keys")
	}

	if len(config.ServiceAuth.HeaderMutationKeyName) == 0 {
		log.Fatal("serviceAuth.headerMutationKeyName is not set")
	}

	log.Printf("Loaded %d serviceKeys", jwtKeyset.Len())
}

func parseConfig(filename string) (*ControllerConfig, error) {
	f, err := os.Open(*configFile)
	if err != nil {
		return nil, fmt.Errorf("while opening configfile: %w", err)
	}

	c, err := LoadConfig(f)
	if err != nil {
		return nil, fmt.Errorf("while loading config: %w", err)
	}

	return c, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, logger := loggerFromContext(ctx)

	logger.Infof("%s", version.VersionString())
	flag.Parse()
	if *showversion {
		os.Exit(0)
	}

	logger.Infow("controller starting",
		"version", version.VersionString(),
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"cores", runtime.NumCPU(),
	)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT)

	if *jaegerEndpoint != "" {
		*jaegerEndpoint = util.GetEnvar("JAEGER_TRACE_URL", "")
	}

	var err error
	tracerProvider, err = tracer.NewTracerProvider(*jaegerEndpoint, *traceToStdout, version.GitHash(), appName, *traceRatio)
	util.Check(err)
	defer tracerProvider.Shutdown(ctx)

	config, err = parseConfig(*configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}
	config.Dump()

	namespace, ok := os.LookupEnv("POD_NAMESPACE")
	if ok {
		secretsLoader, err = secrets.MakeKubernetesSecretLoader(namespace)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("POD_NAMESPACE not set.  Disabling Kubeernetes secret handling.")
	}

	loadKeyset()

	// Create registry entries to sign and validate JWTs for service authentication,
	// and protect x-spinnaker-user header.
	if err = jwtutil.RegisterServiceKeyset(jwtKeyset, config.ServiceAuth.CurrentKeyName); err != nil {
		log.Fatal(err)
	}
	if err = jwtutil.RegisterMutationKeyset(jwtKeyset, config.ServiceAuth.HeaderMutationKeyName); err != nil {
		log.Fatal(err)
	}

	//
	// Make a new CA, for our use to generate server and other certificates.
	//
	caLocal, err := ca.LoadCAFromFile(config.CAConfig)
	if err != nil {
		log.Fatalf("Cannot create authority: %v", err)
	}
	authority = caLocal

	//
	// Make a server certificate.
	//
	log.Println("Generating a server certificate...")
	serverCert, err := authority.MakeServerCert(config.ServerNames)
	if err != nil {
		log.Fatalf("Cannot make server certificate: %v", err)
	}

	//endpoints = serviceconfig.ConfigureEndpoints(secretsLoader, &config.ServiceConfig)

	//	cnc := cncserver.MakeCNCServer(config, authority, routes, version.GitBranch())
	//	go cnc.RunServer(*serverCert)

	go runAgentGRPCServer(ctx, config.AgentUseTLS, serverCert)

	// Always listen on our well-known port, and always use HTTPS for this one.
	//	go serviceconfig.RunHTTPSServer(routes, authority, *serverCert, serviceconfig.IncomingServiceConfig{
	//		Name: "_services",
	//		Port: config.ServiceListenPort,
	//	})

	// Now, add all the others defined by our config.
	//	for _, service := range config.ServiceConfig.IncomingServices {
	//		if service.UseHTTP {
	//			go serviceconfig.RunHTTPServer(routes, service)
	//		} else {
	//			go serviceconfig.RunHTTPSServer(routes, authority, *serverCert, service)
	//		}
	//	}

	go runPrometheusHTTPServer(config.PrometheusListenPort)

	<-sigchan
	log.Printf("Exiting Cleanly")
}
