/*
 * Copyright 2021 OpsMx, Inc.
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

package serviceconfig

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/skandragon/grpc-datacon/internal/jwtutil"
	"github.com/skandragon/grpc-datacon/internal/logging"
	"github.com/skandragon/grpc-datacon/internal/secrets"
	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"gopkg.in/yaml.v3"
)

type genericEndpointCredentials struct {
	Type       string `yaml:"type,omitempty"`
	Username   string `yaml:"username,omitempty"`
	Password   string `yaml:"password,omitempty"`
	Token      string `yaml:"token,omitempty"`
	SecretName string `yaml:"secretName,omitempty"`

	rawUsername string `yaml:"-"`
	rawPassword string `yaml:"-"`
	rawToken    string `yaml:"-"`
}

type genericEndpointConfig struct {
	URL         string                     `yaml:"url,omitempty"`
	Insecure    bool                       `yaml:"insecure,omitempty"`
	Credentials genericEndpointCredentials `yaml:"credentials,omitempty"`
}

// GenericEndpoint defines the state (config and credentials) for a generic HTTP
// endpoint.
type GenericEndpoint struct {
	endpointType string
	endpointName string
	config       genericEndpointConfig
}

func (ep *GenericEndpoint) loadSecrets(secretsLoader secrets.SecretLoader) error {
	if ep.config.Credentials.SecretName == "" {
		return ep.loadBase64Secrets()
	}
	return ep.loadKubernetesSecrets(secretsLoader)
}

func (ep *GenericEndpoint) loadBase64Secrets() error {
	token := ep.config.Credentials.Token
	username := ep.config.Credentials.Username
	password := ep.config.Credentials.Password

	switch ep.config.Credentials.Type {
	case "none", "":
		ep.config.Credentials.Type = "none"
		return nil
	case "basic":
		if username == "" || password == "" {
			return fmt.Errorf("username or password missing for credential type 'basic'")
		}
		rawUsername, err := base64.StdEncoding.DecodeString(username)
		if err != nil {
			return err
		}
		rawPassword, err := base64.StdEncoding.DecodeString(password)
		if err != nil {
			return err
		}
		ep.config.Credentials.rawUsername = string(rawUsername)
		ep.config.Credentials.rawPassword = string(rawPassword)
		return nil
	case "bearer", "token":
		if token == "" {
			return fmt.Errorf("token missing for credential type '%s'", ep.config.Credentials.Type)
		}
		rawToken, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return err
		}
		ep.config.Credentials.rawToken = string(rawToken)
		return nil
	default:
		return fmt.Errorf("unknown credential type %s", ep.config.Credentials.Type)
	}
}

func getItem(m *map[string][]byte, key string) ([]byte, bool) {
	s, found := (*m)[key]
	return s, found && len(s) > 0
}

func (ep *GenericEndpoint) loadKubernetesSecrets(secretsLoader secrets.SecretLoader) error {
	if secretsLoader == nil {
		return fmt.Errorf("cannot load Kubernetes secrets from outside the cluster")
	}

	if ep.config.Credentials.Type == "none" || ep.config.Credentials.Type == "" {
		return fmt.Errorf("none: secretName should not be set")
	}

	secret, err := secretsLoader.GetSecret(ep.config.Credentials.SecretName)
	if err != nil {
		return err
	}

	token, hasToken := getItem(secret, "token")
	username, hasUsername := getItem(secret, "username")
	password, hasPassword := getItem(secret, "password")

	switch ep.config.Credentials.Type {
	case "basic":
		if !hasUsername {
			return fmt.Errorf("basic: username missing in secret")
		}
		if !hasPassword {
			return fmt.Errorf("basic: password missing in secret")
		}
		ep.config.Credentials.rawUsername = string(username)
		ep.config.Credentials.rawPassword = string(password)
		return nil
	case "bearer", "token":
		at := ep.config.Credentials.Type
		if !hasToken {
			return fmt.Errorf("%s: token missing in secret", at)
		}
		ep.config.Credentials.rawToken = string(token)
		return nil
	default:
		return fmt.Errorf("unknown or unsupported credential type %s", ep.config.Credentials.Type)
	}
}

// MakeGenericEndpoint returns a generic HTTP endpoint which allows calling a HTTP service.
func MakeGenericEndpoint(ctx context.Context, endpointType string, endpointName string, configBytes []byte, secretsLoader secrets.SecretLoader) (*GenericEndpoint, bool, error) {
	logger := logging.WithContext(ctx).Sugar()
	ep := &GenericEndpoint{
		endpointType: endpointType,
		endpointName: endpointName,
	}

	var config genericEndpointConfig
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, false, err
	}
	ep.config = config

	err = ep.loadSecrets(secretsLoader)
	if err != nil {
		logger.Errorf("Unable to load secret: %v", err)
		return nil, false, nil
	}

	if ep.config.URL == "" {
		logger.Errorf("url not set for %s/%s", endpointType, endpointName)
		return nil, false, nil
	}

	newURL := strings.TrimSuffix(ep.config.URL, "/")
	if newURL != ep.config.URL {
		ep.config.URL = newURL
	}

	return ep, true, nil
}

func (ep *GenericEndpoint) unmutateURI(typ string, method string, uri string, clock jwt.Clock) (unmutatedURI string, err error) {
	if typ != "fiat" {
		return uri, nil
	}
	if method != http.MethodGet {
		return uri, nil
	}
	if !jwtutil.MutationIsRegistered() {
		return uri, nil
	}
	parts := strings.Split(uri, "/")
	if len(parts) >= 3 && parts[1] == "authorize" {
		if parts[2], err = jwtutil.UnmutateHeader([]byte(parts[2]), clock); err != nil {
			return "", err
		}
		return strings.Join(parts, "/"), nil
	}
	return uri, nil
}

func sendErrorHeaders(ctx context.Context, c pb.TunnelServiceClient, streamID string) {
	logger := logging.WithContext(ctx).Sugar()
	ctx, cancel := getHeaderContext(ctx, session.rpcTimeout)
	defer cancel()
	_, err := c.SendHeaders(ctx, &pb.TunnelHeaders{
		StreamId:      streamID,
		ContentLength: 0,
		Status:        http.StatusBadRequest,
	})
	if err != nil {
		logger.Errorw("unable to SendHeaders", "error", err)
	}
}

func sendHeaders(ctx context.Context, c pb.TunnelServiceClient, streamID string, resp *http.Response) error {
	ctx, cancel := getHeaderContext(ctx, session.rpcTimeout)
	defer cancel()

	headers := []*pb.HttpHeader{}
	for k, v := range resp.Header {
		headers = append(headers, &pb.HttpHeader{Name: k, Values: v})
	}
	_, err := c.SendHeaders(ctx, &pb.TunnelHeaders{
		StreamId:      streamID,
		Status:        int32(resp.StatusCode),
		Headers:       headers,
		ContentLength: resp.ContentLength,
	})
	return err
}

func sendBody(ctx context.Context, c pb.TunnelService_SendDataClient, streamID string, data []byte) error {
	return c.Send(&pb.Data{
		StreamId: streamID,
		Data:     data,
	})
}

func dispatchRequest(ctx context.Context, c pb.TunnelServiceClient, req *pb.TunnelRequest) {
	logger := logging.WithContext(ctx).Sugar()
	hr, err := http.NewRequestWithContext(ctx, req.Method, req.URI, bytes.NewReader(req.Body))
	if err != nil {
		logger.Warnw("dispatchRequest NewRequestWithContext", "error", err)
		sendErrorHeaders(ctx, c, req.StreamId)
		return
	}

	resp, err := http.DefaultClient.Do(hr)
	if err != nil {
		logger.Warnw("dispatchRequest NewRequestWithContext", "error", err)
		sendErrorHeaders(ctx, c, req.StreamId)
		return
	}
	defer resp.Body.Close()

	err = sendHeaders(ctx, c, req.StreamId, resp)
	if err != nil {
		logger.Errorw("dispatchRequest sendHeaders failed", "error", err)
		return
	}

	stream, err := c.SendData(ctx)
	if err != nil {
		logger.Errorw("SendData()", "error", err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if err := sendBody(ctx, stream, req.StreamId, []byte{}); err != nil {
			logger.Errorw("sendBody(EOF)", "error", err)
		}
		return
	}
	logger.Infof("read body: %d bytes", len(body))
	if err := sendBody(ctx, stream, req.StreamId, body); err != nil {
		logger.Errorw("sendBody(with body)", "error", err)
	}
	if err := sendBody(ctx, stream, req.StreamId, []byte{}); err != nil {
		logger.Errorw("sendBody(EOF)", "error", err)
	}
}

type HTTPEcho interface {
	// Headers is called once to send the appropriate headers.
	Headers(ctx context.Context, h *pb.TunnelHeaders) error
	// Data is called one or more times to send data.
	Data(ctx context.Context, d *pb.Data) error
	// Fail can be called to indicate no more calls will be made.  This may happen
	// without calling Headers() or Data(), or after calling one or both.  If
	// headers have been sent, this should send a EOF Data frame.
	Fail(ctx context.Context, httpCode int, err error) error
	// Done indicates the session ended.  If headers have not been sent,
	// this is an error.  Data may not be called, and Done should send
	// an EOF Data frame.
	Done(ctx context.Context)
}

func containsFolded(l []string, t string) bool {
	for i := 0; i < len(l); i++ {
		if strings.EqualFold(l[i], t) {
			return true
		}
	}
	return false
}

var mutatedHeaders = []string{"X-Spinnaker-User"}

func CopyHeaders(headers []*pb.HttpHeader, out *http.Header) error {
	for _, header := range headers {
		if jwtutil.MutationIsRegistered() && containsFolded(mutatedHeaders, header.Name) {
			// only handle the first value here as well
			value := header.Values[0]
			unmutated, err := jwtutil.UnmutateHeader([]byte(value), nil)
			if err != nil {
				return err
			}
			out.Add(header.Name, unmutated)
		} else {
			for _, value := range header.Values {
				out.Add(header.Name, value)
			}
		}
	}
	return nil
}

// ExecuteHTTPRequest does the actual call to connect to HTTP, and will send the data back over the
// tunnel.
func (ep *GenericEndpoint) ExecuteHTTPRequest(ctx context.Context, agentName string, echo HTTPEcho, req *pb.TunnelRequest) error {
	logger := logging.WithContext(ctx).Sugar()
	logger.Debugf("Running request %v", req)
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    tlsConfig,
	}
	if ep.config.Insecure {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	client := &http.Client{
		Transport: tr,
	}

	uri, err := ep.unmutateURI(req.Type, req.Method, req.URI, nil)
	if err != nil {
		err = fmt.Errorf("Failed to unmutate URI %s to %s: %v", req.Method, ep.config.URL+req.URI, err)
		logger.Error(err)
		return echo.Fail(ctx, http.StatusBadGateway, err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, req.Method, ep.config.URL+uri, bytes.NewBuffer(req.Body))
	if err != nil {
		err = fmt.Errorf("Failed to build request for %s to %s: %v", req.Method, ep.config.URL+uri, err)
		logger.Error(err)
		return echo.Fail(ctx, http.StatusBadGateway, err)
	}

	err = CopyHeaders(req.Headers, &httpRequest.Header)
	if err != nil {
		err = fmt.Errorf("failed to copy headers: %v", err)
		logger.Error(err)
		return echo.Fail(ctx, http.StatusBadGateway, err)
	}

	if agentName != "" {
		httpRequest.Header.Set("x-opsmx-agent-name", agentName)
	}

	creds := ep.config.Credentials
	switch creds.Type {
	case "basic":
		u := strings.TrimSpace(creds.rawUsername)
		if u != creds.rawUsername {
			logger.Infof("warning: trimming whitespace from username for %s/%s", ep.endpointType, ep.endpointName)
		}
		p := strings.TrimSpace(creds.rawPassword)
		if p != creds.rawPassword {
			logger.Infof("warning: trimming whitespace from password for %s/%s", ep.endpointType, ep.endpointName)
		}
		httpRequest.SetBasicAuth(u, p)
	case "bearer":
		t := strings.TrimSpace(creds.rawToken)
		if t != creds.rawToken {
			logger.Infof("warning: trimming whitespace from token for %s/%s", ep.endpointType, ep.endpointName)
		}
		httpRequest.Header.Set("Authorization", "Bearer "+creds.rawToken)
	case "token":
		t := strings.TrimSpace(creds.rawToken)
		if t != creds.rawToken {
			logger.Infof("warning: trimming whitespace from token for %s/%s", ep.endpointType, ep.endpointName)
		}
		httpRequest.Header.Set("Authorization", "Token "+creds.rawToken)
	}

	return tunnel.RunHTTPRequest(client, req, httpRequest, echo, ep.config.URL)
}
