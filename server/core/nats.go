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
	"os"
	"strings"
	"time"

	nats "github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
)

func (server *NATSKafkaBridge) natsError(nc *nats.Conn, sub *nats.Subscription, err error) {
	server.logger.Warnf("nats error %s", err.Error())
}

func (server *NATSKafkaBridge) stanConnectionLost(sc stan.Conn, err error) {
	if !server.checkRunning() {
		return
	}
	server.logger.Warnf("nats streaming disconnected")

	server.natsLock.Lock()
	server.stan = nil // we lost stan
	server.natsLock.Unlock()

	server.checkConnections()
}

func (server *NATSKafkaBridge) natsDisconnected(nc *nats.Conn) {
	if !server.checkRunning() {
		return
	}
	server.logger.Warnf("nats disconnected")
	server.checkConnections()
}

func (server *NATSKafkaBridge) natsReconnected(nc *nats.Conn) {
	server.logger.Warnf("nats reconnected")
}

func (server *NATSKafkaBridge) natsClosed(nc *nats.Conn) {
	if server.checkRunning() {
		server.logger.Errorf("nats connection closed, shutting down bridge")
		go func() {
			// When NATS connection is really marked as closed, the bridge cannot
			// do anything else, so stop the bridge and exit the process with an
			// error so that system/docker can restart (if applicable).
			server.Stop()
			os.Exit(2)
		}()
	}
}

func (server *NATSKafkaBridge) natsDiscoveredServers(nc *nats.Conn) {
	server.logger.Debugf("discovered servers: %v\n", nc.DiscoveredServers())
	server.logger.Debugf("known servers: %v\n", nc.Servers())
}

// assumes the lock is held by the caller
func (server *NATSKafkaBridge) connectToNATS() error {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	if !server.running {
		return nil // already stopped
	}

	server.logger.Noticef("connecting to NATS core")

	config := server.config.NATS
	options := []nats.Option{
		nats.MaxReconnects(config.MaxReconnects),
		nats.ReconnectWait(time.Duration(config.ReconnectWait) * time.Millisecond),
		nats.Timeout(time.Duration(config.ConnectTimeout) * time.Millisecond),
		nats.ErrorHandler(server.natsError),
		nats.DiscoveredServersHandler(server.natsDiscoveredServers),
		nats.DisconnectHandler(server.natsDisconnected),
		nats.ReconnectHandler(server.natsReconnected),
		nats.ClosedHandler(server.natsClosed),
		nats.NoCallbacksAfterClientClose(),
	}

	if config.TLS.Root != "" {
		options = append(options, nats.RootCAs(config.TLS.Root))
	}

	if config.TLS.Cert != "" {
		options = append(options, nats.ClientCert(config.TLS.Cert, config.TLS.Key))
	}

	if config.UserCredentials != "" {
		options = append(options, nats.UserCredentials(config.UserCredentials))
	}

	nc, err := nats.Connect(strings.Join(config.Servers, ","),
		options...,
	)

	if err != nil {
		return err
	}

	server.nats = nc
	return nil
}

// assumes the lock is held by the caller
func (server *NATSKafkaBridge) connectToSTAN() error {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	if server.stan != nil {
		return nil // already connected
	}

	if server.config.STAN.ClusterID == "" {
		server.logger.Noticef("skipping NATS streaming connection, not configured")
		return nil
	}

	server.logger.Noticef("connecting to NATS streaming")
	config := server.config.STAN
	if config.DiscoverPrefix == "" {
		config.DiscoverPrefix = stan.DefaultDiscoverPrefix
	}

	sc, err := stan.Connect(config.ClusterID, config.ClientID,
		stan.NatsConn(server.nats),
		stan.PubAckWait(time.Duration(config.PubAckWait)*time.Millisecond),
		stan.MaxPubAcksInflight(config.MaxPubAcksInflight),
		stan.ConnectWait(time.Duration(config.ConnectWait)*time.Millisecond),
		stan.SetConnectionLostHandler(server.stanConnectionLost),
		func(o *stan.Options) error {
			o.DiscoverPrefix = config.DiscoverPrefix
			return nil
		})
	if err != nil {
		return err
	}
	server.stan = sc

	return nil
}

func (server *NATSKafkaBridge) connectToJetStream() error {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	if server.js != nil {
		return nil // already connected
	}

	var hasJetStream bool
	for _, c := range server.config.Connect {
		if strings.Contains(c.Type, "JetStream") {
			hasJetStream = true
			break
		}
	}
	if !hasJetStream {
		server.logger.Noticef("skipping JetStream connection, not configured")
		return nil
	}

	server.logger.Noticef("connecting to JetStream")

	var opts []nats.JSOpt
	c := server.config.JetStream
	if c.MaxWait > 0 {
		opts = append(opts, nats.MaxWait(time.Duration(c.MaxWait)*time.Millisecond))
	}
	if c.PublishAsyncMaxPending > 0 {
		opts = append(opts, nats.PublishAsyncMaxPending(c.PublishAsyncMaxPending))
	}

	js, err := server.nats.JetStream(opts...)
	if err != nil {
		return err
	}
	server.js = js

	return nil
}
