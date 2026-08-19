// Package mqtttransport provides the MQTT data-plane used by Tier0 CLI.
package mqtttransport

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
)

// Connection contains the non-secret and secret fields needed to connect.
type Connection struct {
	Broker             string
	ClientID           string
	Username           string
	Password           string
	CAFile             string
	TLSServerName      string
	InsecureSkipVerify bool
	ConnectTimeout     time.Duration
	AutoReconnect      bool
}

// Message is a copied MQTT message safe to use after the Paho callback returns.
type Message struct {
	Topic     string
	Payload   []byte
	QoS       byte
	Retained  bool
	Duplicate bool
	Received  time.Time
}

// Subscription owns a live client and its message stream.
type Subscription struct {
	client   mqtt.Client
	messages chan Message
	errors   chan error
	once     sync.Once
}

// NormalizeBroker converts host-only and common MQTT URLs into Paho URLs.
func NormalizeBroker(raw string) (string, error) {
	raw = strings.TrimSpace(strings.Split(raw, ",")[0])
	if raw == "" {
		return "", errors.New("MQTT broker is empty")
	}
	if strings.HasPrefix(strings.ToLower(raw), "mqtt://") {
		raw = "tcp://" + raw[len("mqtt://"):]
	}
	if strings.HasPrefix(strings.ToLower(raw), "mqtts://") {
		raw = "ssl://" + raw[len("mqtts://"):]
	}
	if !strings.Contains(raw, "://") {
		host, port, err := net.SplitHostPort(raw)
		if err == nil {
			scheme := schemeForPort(port)
			raw = scheme + "://" + net.JoinHostPort(host, port)
		} else {
			if strings.Count(raw, ":") > 1 && !strings.HasPrefix(raw, "[") {
				raw = "ssl://[" + raw + "]:8883"
			} else {
				raw = "ssl://" + raw + ":8883"
			}
		}
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid MQTT broker: %w", err)
	}
	switch parsed.Scheme {
	case "tcp", "ssl", "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported MQTT broker scheme %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return "", errors.New("MQTT broker host is empty")
	}
	if parsed.User != nil {
		return "", errors.New("MQTT broker URL must not contain credentials; use a saved profile or MQTT environment variables")
	}
	if parsed.Port() == "" {
		port := "8883"
		switch parsed.Scheme {
		case "tcp":
			port = "1883"
		case "ws":
			port = "8083"
		case "wss":
			port = "8084"
		}
		parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid MQTT broker port %q", parsed.Port())
	}
	if (parsed.Scheme == "ws" || parsed.Scheme == "wss") && (parsed.Path == "" || parsed.Path == "/") {
		parsed.Path = "/mqtt"
	}
	return parsed.String(), nil
}

func schemeForPort(port string) string {
	switch port {
	case "1883":
		return "tcp"
	case "8083":
		return "ws"
	case "8084":
		return "wss"
	default:
		return "ssl"
	}
}

// RuntimeClientID adds a collision-resistant suffix when the server credential
// permits it, avoiding MQTT clients disconnecting each other.
func RuntimeClientID(base string, randomSuffix bool) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", errors.New("MQTT client ID is empty")
	}
	if !randomSuffix {
		return base, nil
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate MQTT client ID suffix: %w", err)
	}
	return base + "-" + hex.EncodeToString(buf), nil
}

// ValidatePublishTopic rejects MQTT wildcard filters for publish operations.
func ValidatePublishTopic(topic string) error {
	if err := validateTopicCommon(topic); err != nil {
		return err
	}
	if strings.ContainsAny(topic, "+#") {
		return errors.New("publish topic cannot contain '+' or '#' wildcards")
	}
	return nil
}

// ValidateSubscribeTopic validates MQTT + and # filter placement.
func ValidateSubscribeTopic(topic string) error {
	if err := validateTopicCommon(topic); err != nil {
		return err
	}
	levels := strings.Split(topic, "/")
	for i, level := range levels {
		if strings.Contains(level, "+") && level != "+" {
			return errors.New("'+' must occupy an entire topic level")
		}
		if strings.Contains(level, "#") && (level != "#" || i != len(levels)-1) {
			return errors.New("'#' must occupy the final topic level")
		}
	}
	return nil
}

func validateTopicCommon(topic string) error {
	if strings.TrimSpace(topic) == "" {
		return errors.New("MQTT topic is empty")
	}
	if strings.ContainsRune(topic, '\x00') {
		return errors.New("MQTT topic cannot contain a null character")
	}
	return nil
}

// Publish connects, publishes one payload, waits for acknowledgement when
// required by QoS, and disconnects cleanly.
func Publish(ctx context.Context, connection Connection, topic string, qos byte, retained bool, payload []byte) error {
	client, err := connect(ctx, connection, nil, nil, nil)
	if err != nil {
		return err
	}
	defer client.Disconnect(250)
	return waitToken(ctx, client.Publish(topic, qos, retained, payload))
}

// Subscribe connects and subscribes. Automatic reconnect re-establishes the
// subscription through OnConnect.
func Subscribe(ctx context.Context, connection Connection, topic string, qos byte) (*Subscription, error) {
	sub := &Subscription{
		messages: make(chan Message, 64),
		errors:   make(chan error, 8),
	}
	ready := make(chan error, 1)
	var initial sync.Once

	handler := func(_ mqtt.Client, message mqtt.Message) {
		payload := append([]byte(nil), message.Payload()...)
		item := Message{
			Topic:     message.Topic(),
			Payload:   payload,
			QoS:       message.Qos(),
			Retained:  message.Retained(),
			Duplicate: message.Duplicate(),
			Received:  time.Now().UTC(),
		}
		select {
		case sub.messages <- item:
		case <-ctx.Done():
		}
	}
	onConnect := func(client mqtt.Client) {
		go func() {
			err := waitToken(ctx, client.Subscribe(topic, qos, handler))
			first := false
			initial.Do(func() {
				first = true
				ready <- err
			})
			if !first && err != nil {
				sub.report(err)
			}
		}()
	}
	onLost := func(_ mqtt.Client, err error) {
		if err != nil {
			sub.report(fmt.Errorf("MQTT connection lost: %w", err))
		}
	}

	client, err := connect(ctx, connection, onConnect, onLost, handler)
	if err != nil {
		return nil, err
	}
	sub.client = client
	select {
	case err := <-ready:
		if err != nil {
			sub.Close()
			return nil, fmt.Errorf("subscribe to %q: %w", topic, err)
		}
		return sub, nil
	case <-ctx.Done():
		sub.Close()
		return nil, ctx.Err()
	}
}

// Messages returns the subscription message stream.
func (s *Subscription) Messages() <-chan Message { return s.messages }

// Errors returns recoverable connection/subscription errors. Automatic
// reconnect remains active until Close is called.
func (s *Subscription) Errors() <-chan error { return s.errors }

// Close terminates the MQTT session. It is safe to call more than once.
func (s *Subscription) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		if s.client != nil {
			s.client.Disconnect(250)
		}
	})
}

func (s *Subscription) report(err error) {
	select {
	case s.errors <- err:
	default:
	}
}

func connect(
	ctx context.Context,
	connection Connection,
	onConnect mqtt.OnConnectHandler,
	onLost mqtt.ConnectionLostHandler,
	defaultHandler mqtt.MessageHandler,
) (mqtt.Client, error) {
	broker, err := NormalizeBroker(connection.Broker)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(connection.ClientID) == "" || strings.TrimSpace(connection.Username) == "" || connection.Password == "" {
		return nil, errors.New("MQTT credential is incomplete")
	}
	tlsConfig, err := buildTLSConfig(broker, connection)
	if err != nil {
		return nil, err
	}
	connectTimeout := connection.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = 15 * time.Second
	}

	options := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(connection.ClientID).
		SetUsername(connection.Username).
		SetPassword(connection.Password).
		SetConnectTimeout(connectTimeout).
		SetWriteTimeout(connectTimeout).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(10 * time.Second).
		SetCleanSession(true).
		SetAutoReconnect(connection.AutoReconnect).
		SetOrderMatters(false)
	if tlsConfig != nil {
		options.SetTLSConfig(tlsConfig)
	}
	if onConnect != nil {
		options.SetOnConnectHandler(onConnect)
	}
	if onLost != nil {
		options.SetConnectionLostHandler(onLost)
	}
	if defaultHandler != nil {
		options.SetDefaultPublishHandler(defaultHandler)
	}

	client := mqtt.NewClient(options)
	if err := waitToken(ctx, client.Connect()); err != nil {
		client.Disconnect(0)
		return nil, fmt.Errorf("connect to MQTT broker: %w", err)
	}
	return client, nil
}

// IsAuthenticationError reports broker rejections caused by invalid or
// revoked MQTT credentials.
func IsAuthenticationError(err error) bool {
	return errors.Is(err, packets.ErrorRefusedBadUsernameOrPassword) ||
		errors.Is(err, packets.ErrorRefusedNotAuthorised)
}

func buildTLSConfig(broker string, connection Connection) (*tls.Config, error) {
	parsed, err := url.Parse(broker)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "ssl" && parsed.Scheme != "wss" {
		if connection.CAFile != "" || connection.TLSServerName != "" || connection.InsecureSkipVerify {
			return nil, errors.New("TLS options require an ssl:// or wss:// broker")
		}
		return nil, nil
	}
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         strings.TrimSpace(connection.TLSServerName),
		InsecureSkipVerify: connection.InsecureSkipVerify, // #nosec G402 -- explicit CLI opt-in only
	}
	if config.ServerName == "" {
		config.ServerName = parsed.Hostname()
	}
	if connection.CAFile != "" {
		pem, err := os.ReadFile(connection.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read MQTT CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("MQTT CA file contains no valid certificates")
		}
		config.RootCAs = pool
	}
	return config, nil
}

func waitToken(ctx context.Context, token mqtt.Token) error {
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ParseQoS validates an integer and returns its wire representation.
func ParseQoS(value int) (byte, error) {
	if value < 0 || value > 2 {
		return 0, fmt.Errorf("QoS must be 0, 1, or 2, got %s", strconv.Itoa(value))
	}
	return byte(value), nil
}
