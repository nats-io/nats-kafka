/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package core

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	"github.com/nats-io/nats-kafka/server/logging"
	nats "github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
)

var version = "0.0-dev"

// NATSKafkaBridge is the core structure for the server.
type NATSKafkaBridge struct {
	sync.Mutex
	running bool

	startTime time.Time

	logger logging.Logger
	config conf.NATSKafkaBridgeConfig

	natsLock sync.Mutex
	nats     *nats.Conn
	stan     stan.Conn

	connectors []Connector

	reconnectLock  sync.Mutex
	reconnect      map[string]Connector
	reconnectTimer *reconnectTimer

	statsLock     sync.Mutex
	httpReqStats  map[string]int64
	listener      net.Listener
	http          *http.Server
	httpHandler   *http.ServeMux
	monitoringURL string
}

// NewNATSKafkaBridge creates a new account server with a default logger
func NewNATSKafkaBridge() *NATSKafkaBridge {
	return &NATSKafkaBridge{
		logger: logging.NewNATSLogger(logging.Config{
			Colors: true,
			Time:   true,
			Debug:  true,
			Trace:  true,
		}),
	}
}

// Logger hosts a shared logger
func (server *NATSKafkaBridge) Logger() logging.Logger {
	return server.logger
}

func (server *NATSKafkaBridge) checkRunning() bool {
	server.Lock()
	defer server.Unlock()
	return server.running
}

// InitializeFromFlags is called from main to configure the server, the server
// will decide what needs to happen based on the flags. On reload the same flags are
// passed
func (server *NATSKafkaBridge) InitializeFromFlags(flags Flags) error {
	server.config = conf.DefaultBridgeConfig()

	// Always try to apply a config file, we can't run without one
	err := server.ApplyConfigFile(flags.ConfigFile)

	if err != nil {
		return err
	}

	if flags.Debug || flags.DebugAndVerbose {
		server.config.Logging.Debug = true
	}

	if flags.Verbose || flags.DebugAndVerbose {
		server.config.Logging.Trace = true
	}

	return nil
}

// ApplyConfigFile applies the config file to the server's config
func (server *NATSKafkaBridge) ApplyConfigFile(configFile string) error {
	if configFile == "" {
		configFile = os.Getenv("NATS_KAFKA_BRIDGE_CONFIG")
		if configFile != "" {
			server.logger.Noticef("using config specified in $NATS_KAFKA_BRIDGE_CONFIG %q", configFile)
		}
	} else {
		server.logger.Noticef("loading configuration from %q", configFile)
	}

	if configFile == "" {
		return fmt.Errorf("no config file specified")
	}

	if err := conf.LoadConfigFromFile(configFile, &server.config, false); err != nil {
		return err
	}

	return nil
}

// InitializeFromConfig initialize the server's configuration to an existing config object, useful for tests
// Does not change the config at all, use DefaultServerConfig() to create a default config
func (server *NATSKafkaBridge) InitializeFromConfig(config conf.NATSKafkaBridgeConfig) error {
	server.config = config
	return nil
}

// Start the server, will lock the server, assumes the config is loaded
func (server *NATSKafkaBridge) Start() error {
	server.Lock()
	defer server.Unlock()

	if server.logger != nil {
		server.logger.Close()
	}

	server.running = true
	server.startTime = time.Now()
	server.logger = logging.NewNATSLogger(server.config.Logging)
	server.connectors = []Connector{}
	server.reconnect = map[string]Connector{}

	server.logger.Noticef("starting NATS-Kafka Bridge, version %s", version)
	server.logger.Noticef("server time is %s", server.startTime.Format(time.UnixDate))

	if err := server.connectToNATS(); err != nil {
		return err
	}

	if err := server.connectToSTAN(); err != nil {
		return err
	}

	if err := server.initializeConnectors(); err != nil {
		return err
	}

	if err := server.startConnectors(); err != nil {
		return err
	}

	if err := server.startMonitoring(); err != nil {
		return err
	}

	return nil
}

// Stop the account server
func (server *NATSKafkaBridge) Stop() {
	server.Lock()
	defer server.Unlock()

	if !server.running {
		return // already stopped
	}

	server.logger.Noticef("stopping bridge")

	server.running = false
	server.stopReconnectTimer()
	server.reconnect = map[string]Connector{} // clear the map

	for _, c := range server.connectors {
		err := c.Shutdown()

		if err != nil {
			server.logger.Noticef("error shutting down connector %s", err.Error())
		}
	}

	// Shutdown STAN connection first, since it needs NATS connection to send
	// the close protocol.
	if server.stan != nil {
		server.stan.Close()
		server.logger.Noticef("disconnected from NATS streaming")
	}

	if server.nats != nil {
		if err := server.nats.Drain(); err != nil {
			server.logger.Noticef("error draining NATS connection %s", err.Error())
		}
		server.logger.Noticef("disconnected from NATS")
	}

	err := server.StopMonitoring()
	if err != nil {
		server.logger.Noticef("error shutting down monitoring server %s", err.Error())
	}
}

// assumes the lock is held by the caller
func (server *NATSKafkaBridge) initializeConnectors() error {
	connectorConfigs := server.config.Connect

	for _, c := range connectorConfigs {
		connector, err := CreateConnector(c, server)

		if err != nil {
			return err
		}

		server.connectors = append(server.connectors, connector)
	}
	return nil
}

// assumes the lock is held by the caller
func (server *NATSKafkaBridge) startConnectors() error {
	for _, c := range server.connectors {
		if err := c.Start(); err != nil {
			server.logger.Noticef("error starting %s, %s", c.String(), err.Error())
			return err
		}
	}
	return nil
}

// FatalError stops the server, prints the messages and exits
func (server *NATSKafkaBridge) FatalError(format string, args ...interface{}) {
	server.Stop()
	log.Fatalf(format, args...)
	os.Exit(-1)
}

// NATS hosts a shared nats connection for the connectors
func (server *NATSKafkaBridge) NATS() *nats.Conn {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()
	return server.nats
}

// Stan hosts a shared streaming connection for the connectors
func (server *NATSKafkaBridge) Stan() stan.Conn {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()
	return server.stan
}

// CheckNATS returns true if the bridge is connected to nats
func (server *NATSKafkaBridge) CheckNATS() bool {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	if server.nats != nil {
		return server.nats.ConnectedUrl() != ""
	}

	return false
}

// CheckStan returns true if the bridge is connected to stan
func (server *NATSKafkaBridge) CheckStan() bool {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	if server.nats != nil {
		ok := server.nats.ConnectedUrl() != ""

		if !ok {
			return false
		}
	}

	return server.stan != nil
}

// ConnectorError is called by a connector if it has a failure that requires a reconnect
func (server *NATSKafkaBridge) ConnectorError(connector Connector, err error) {
	if !server.checkRunning() {
		return
	}

	server.reconnectLock.Lock()
	defer server.reconnectLock.Unlock()

	_, check := server.reconnect[connector.ID()]

	if check {
		return // we already have that connector, no need to stop or pring any messages
	}

	description := connector.String()
	server.logger.Errorf("a connector error has occurred, bridge will try to restart %s, %s", description, err.Error())

	err = connector.Shutdown()

	if err != nil {
		server.logger.Warnf("error shutting down connector %s, bridge will try to restart, %s", description, err.Error())
	}

	server.reconnect[connector.ID()] = connector

	server.ensureReconnectTimer()
}

// checkConnections loops over the connections and has them each check check their requirements
func (server *NATSKafkaBridge) checkConnections() {
	server.logger.Warnf("checking connector requirements and will restart as needed.")

	if !server.checkRunning() {
		return
	}

	server.reconnectLock.Lock()
	defer server.reconnectLock.Unlock()

	for _, connector := range server.connectors {
		_, check := server.reconnect[connector.ID()]

		if check {
			continue // we already have that connector, no need to stop or pring any messages
		}

		err := connector.CheckConnections()

		if err == nil {
			continue // connector is happy
		}

		description := connector.String()
		server.logger.Errorf("a connector error has occurred, trying to restart %s, %s", description, err.Error())

		err = connector.Shutdown()

		if err != nil {
			server.logger.Warnf("error shutting down connector %s, trying to restart, %s", description, err.Error())
		}

		server.reconnect[connector.ID()] = connector
	}

	server.ensureReconnectTimer()
}

// requires the reconnect lock be held by the caller
// spawns a go routine that will acquire the lock for handling reconnect tasks
func (server *NATSKafkaBridge) ensureReconnectTimer() {
	if server.reconnectTimer != nil {
		return
	}

	timer := newReconnectTimer()
	server.reconnectTimer = timer

	go func() {
		var doReconnect bool
		interval := server.config.ReconnectInterval

		doReconnect = <-timer.After(time.Duration(interval) * time.Millisecond)
		if !doReconnect {
			return
		}

		server.reconnectLock.Lock()
		defer server.reconnectLock.Unlock()

		// Wait for nats to be reconnected
		if !server.CheckNATS() {
			server.logger.Noticef("nats connection is down, will try reconnecting to NATS and restarting connectors in %d milliseconds", interval)
			server.reconnectTimer = nil
			server.ensureReconnectTimer()
			// Until we get a NATS connection, no point in continuing.
			return
		}

		// Make sure stan is up, if it should be
		if server.stan == nil {
			server.logger.Noticef("trying to reconnect to nats streaming")
			err := server.connectToSTAN() // this may be a no-op if server.stan == nil was true but is not true once we get the lock in the connect

			if err != nil {
				server.logger.Noticef("error restarting streaming connection, will retry in %d milliseconds: %v", interval, err.Error())
				server.reconnectTimer = nil
				server.ensureReconnectTimer()
				// Until we get a STAN connection, no point in continuing.
				return
			}
		}

		// Do all the reconnects
		for id, connector := range server.reconnect {
			server.logger.Noticef("trying to restart connector %s", connector.String())
			err := connector.Start()

			if err != nil {
				server.logger.Noticef("error restarting connector %s, will retry in %d milliseconds, %s", connector.String(), interval, err.Error())
				continue
			}

			delete(server.reconnect, id)
		}

		server.reconnectTimer = nil

		if len(server.reconnect) > 0 {
			server.ensureReconnectTimer()
		}
	}()
}

// locks the reconnect lock
func (server *NATSKafkaBridge) stopReconnectTimer() {
	server.reconnectLock.Lock()
	defer server.reconnectLock.Unlock()

	if server.reconnectTimer != nil {
		server.reconnectTimer.Cancel()
	}

	server.reconnectTimer = nil
}

/*
func (server *NATSKafkaBridge) checkReconnecting() bool {
	server.reconnectLock.Lock()
	reconnecting := len(server.reconnect) > 0
	server.reconnectLock.Unlock()
	return reconnecting
}
*/
