package redox

import (
	"context"
	"crypto/rsa"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	// The period of time before the token expiration when we should refresh it
	expirationDelta = 1 * time.Minute
)

type Client struct {
	config      ClientConfig
	restyClient *resty.Client
	logger      *zap.SugaredLogger
	privateKey  *rsa.PrivateKey

	token *Token
	mu    *sync.RWMutex
}

type ClientConfig struct {
	ClientId      string `envconfig:"TIDEPOOL_REDOX_CLIENT_ID" required:"true"`
	KeyId         string `envconfig:"TIDEPOOL_REDOX_KEY_ID" required:"true"`
	PrivateKeyPem string `envconfig:"TIDEPOOL_REDOX_PRIVATE_KEY" required:"true"`
	SourceId      string `envconfig:"TIDEPOOL_REDOX_SOURCE_ID" required:"true"`
	SourceName    string `envconfig:"TIDEPOOL_REDOX_SOURCE_NAME" required:"true"`
	TestMode      bool   `envconfig:"TIDEPOOL_REDOX_TEST_MODE"`
}

func NewClient(moduleConfig ModuleConfig, logger *zap.SugaredLogger) (*Client, error) {
	client := &Client{
		logger: logger,
		mu:     &sync.RWMutex{},
	}

	if moduleConfig.Enabled {
		config := ClientConfig{}
		err := envconfig.Process("", &config)
		if err != nil {
			return nil, err
		}

		client.restyClient = resty.New()
		client.config = config
		client.privateKey, err = jwt.ParseRSAPrivateKeyFromPEM([]byte(config.PrivateKeyPem))
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (c *Client) GetSource() (source struct {
	ID   *string `json:"ID"`
	Name *string `json:"Name"`
}) {
	source.ID = &c.config.SourceId
	source.Name = &c.config.SourceName
	return
}

func (c *Client) Send(ctx context.Context, payload interface{}) error {
	req, err := c.getRequestWithFreshToken(ctx)
	if err != nil {
		return err
	}

	httpErr := &ErrorResponse{}
	resp, err := req.
		SetBody(payload).
		SetHeader("Content-Type", "application/json").
		SetError(httpErr).
		Post("https://api.redoxengine.com/endpoint")

	if err != nil {
		return fmt.Errorf("error sending payload to redox: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("received error response when sending payload to redox: %w", httpErr)
	}
	return nil
}

func (c *Client) getRequest(ctx context.Context) *resty.Request {
	return c.restyClient.R().SetContext(ctx)
}

func (c *Client) getRequestWithFreshToken(ctx context.Context) (*resty.Request, error) {
	if c.shouldRefreshToken() {
		if err := c.obtainFreshToken(ctx); err != nil {
			return nil, err
		}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getRequest(ctx).SetAuthToken(c.token.AccessToken), nil
}

func (c *Client) shouldRefreshToken() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token == nil || c.token.IsExpired(expirationDelta)
}

func (c *Client) obtainFreshToken(ctx context.Context) error {
	assertion, err := c.getSignedAssertion()
	if err != nil {
		return err
	}

	token := &Token{}
	authErr := &AuthError{}

	data := map[string]string{
		"grant_type":            "client_credentials",
		"client_assertion_type": "urn:ietf:params:oauth:client-assertion-type:jwt-bearer",
		"client_assertion":      assertion,
	}

	c.logger.Debugw("obtaining token", "data", data)
	resp, err := c.getRequest(ctx).
		SetFormData(data).
		SetResult(token).
		SetError(authErr).
		Post("https://api.redoxengine.com/v2/auth/token")

	if err != nil {
		return fmt.Errorf("error obtaining token: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("error response when obtaining token: %w", authErr)
	}

	token.SetExpirationTime()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = token
	return nil
}

func (c *Client) getSignedAssertion() (string, error) {
	now := time.Now()
	nonce, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	assertion := jwt.New(jwt.SigningMethodRS384)
	assertion.Header = map[string]interface{}{
		"alg": "RS384",
		"kid": c.config.KeyId,
		"typ": "JWT",
	}
	assertion.Claims = jwt.MapClaims{
		"iss": c.config.ClientId,
		"sub": c.config.ClientId,
		"iat": now.Unix(),
		"exp": now.Add(time.Minute * 5).Unix(),
		"jti": nonce.String(),
	}

	return assertion.SignedString(c.privateKey)
}

type Token struct {
	AccessToken    string `json:"access_token"`
	ExpiresIn      int    `json:"expires_in"`
	ExpirationTime time.Time
}

func (c *Token) SetExpirationTime() {
	c.ExpirationTime = time.Now().Add(time.Duration(c.ExpiresIn) * time.Second)
}

func (c *Token) IsExpired(delta time.Duration) bool {
	return time.Now().After(c.ExpirationTime.Add(-delta))
}

type AuthError struct {
	Err              string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorUri         string `json:"error_uri"`
}

func (a AuthError) Error() string {
	return fmt.Sprintf("%v: %v. URI: %v", a.Err, a.ErrorDescription, a.ErrorUri)
}

type ErrorResponse struct {
	ErrorDetail string `json:"errorDetail"`
}

func (e ErrorResponse) Error() string {
	return e.ErrorDetail
}
