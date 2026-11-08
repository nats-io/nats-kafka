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
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats-kafka/server/conf"
	nats "github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
)

// natsCell wraps the connection state for a single NATS cluster (cell).
// The bridge holds one cell for the default nats block (config.Name "") and
// one per entry in the natsclusters config list. The nc and js fields are
// guarded by the bridge's natsLock.
type natsCell struct {
	config conf.NATSConfig
	nc     *nats.Conn
	js     nats.JetStreamContext
}

// displayName is the cell name used in logs and monitoring
func (cell *natsCell) displayName() string {
	if cell.config.Name == "" {
		return "default"
	}
	return cell.config.Name
}

// isConnected reports whether the cell's NATS connection is established,
// assumes the bridge's natsLock is held by the caller
func (cell *natsCell) isConnected() bool {
	return cell.nc != nil && cell.nc.ConnectedUrl() != ""
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

func (server *NATSKafkaBridge) natsDisconnected(cell *natsCell) {
	if !server.checkRunning() {
		return
	}
	server.logger.Warnf("nats disconnected, cluster %s", cell.displayName())
	server.checkConnections()
}

func (server *NATSKafkaBridge) natsReconnected(cell *natsCell) {
	server.logger.Warnf("nats reconnected, cluster %s", cell.displayName())
}

func (server *NATSKafkaBridge) natsClosed(cell *natsCell) {
	if !server.checkRunning() {
		return
	}

	if cell.config.Name == "" {
		server.logger.Errorf("nats connection closed for the default cluster, shutting down bridge")
		go func() {
			// When the default NATS connection is really marked as closed, the
			// bridge cannot do anything else, so stop the bridge and exit the
			// process with an error so that system/docker can restart (if applicable).
			server.Stop()
			os.Exit(2)
		}()
		return
	}

	// A named cell going away should not take down connectors on the other
	// cells, queue its connectors for restart and let the reconnect timer
	// re-dial the cluster.
	server.logger.Errorf("nats connection closed for cluster %s, its connectors will restart when the cluster is reachable", cell.displayName())
	server.checkConnections()
}

func (server *NATSKafkaBridge) natsDiscoveredServers(nc *nats.Conn) {
	server.logger.Debugf("discovered servers: %v\n", nc.DiscoveredServers())
	server.logger.Debugf("known servers: %v\n", nc.Servers())
}

// dialCell opens a NATS connection for the cell's config. It does not touch
// the cell's shared fields, so it may run without the natsLock; the caller
// stores the returned connection under the lock.
func (server *NATSKafkaBridge) dialCell(cell *natsCell) (*nats.Conn, error) {
	server.logger.Noticef("connecting to NATS core, cluster %s", cell.displayName())

	config := cell.config

	clientName := config.ClientName
	if config.Name != "" {
		clientName = fmt.Sprintf("%s [%s]", clientName, config.Name)
	}

	options := []nats.Option{
		nats.Name(clientName),
		nats.MaxReconnects(config.MaxReconnects),
		nats.ReconnectWait(time.Duration(config.ReconnectWait) * time.Millisecond),
		nats.Timeout(time.Duration(config.ConnectTimeout) * time.Millisecond),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			server.logger.Warnf("nats error on cluster %s, %s", cell.displayName(), err.Error())
		}),
		nats.DiscoveredServersHandler(server.natsDiscoveredServers),
		nats.DisconnectHandler(func(nc *nats.Conn) {
			server.natsDisconnected(cell)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			server.natsReconnected(cell)
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			server.natsClosed(cell)
		}),
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

	if config.UserNKEY != "" {
		opt, err := nats.NkeyOptionFromSeed(config.UserNKEY)
		if err != nil {
			return nil, err
		}
		options = append(options, opt)
	}

	return nats.Connect(strings.Join(config.Servers, ","),
		options...,
	)
}

// connectToNATS builds the cell map from the config and connects every cell.
// The default cluster must be reachable; a named cluster that is not gets a
// warning and is left for the reconnect timer, so one unreachable cell does
// not keep the whole bridge down.
// Assumes the lock is held by the caller.
func (server *NATSKafkaBridge) connectToNATS() error {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	if !server.running {
		return nil // already stopped
	}

	server.cells = map[string]*natsCell{
		"": {config: server.config.NATS},
	}
	for _, c := range server.config.NATSClusters {
		server.cells[c.Name] = &natsCell{config: c}
	}

	for _, cell := range server.cells {
		nc, err := server.dialCell(cell)
		if err != nil {
			if cell.config.Name == "" {
				return err
			}
			server.logger.Warnf("error connecting to NATS cluster %s, its connectors will start when the cluster is reachable, %s", cell.displayName(), err.Error())
			continue
		}
		cell.nc = nc
	}
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

	cell := server.cells[""]
	if cell == nil || cell.nc == nil {
		return fmt.Errorf("default NATS connection is not available for NATS streaming")
	}

	server.logger.Noticef("connecting to NATS streaming")
	config := server.config.STAN
	if config.DiscoverPrefix == "" {
		config.DiscoverPrefix = stan.DefaultDiscoverPrefix
	}

	sc, err := stan.Connect(config.ClusterID, config.ClientID,
		stan.NatsConn(cell.nc),
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

// cellHasJetStream reports whether any configured connector needs JetStream
// on the named cell
func (server *NATSKafkaBridge) cellHasJetStream(name string) bool {
	for _, c := range server.config.Connect {
		if c.NATSConnection == name && strings.Contains(c.Type, "JetStream") {
			return true
		}
	}
	return false
}

// connectCellJetStream derives the JetStream context from the cell's
// current connection. Assumes the natsLock is held by the caller.
func (server *NATSKafkaBridge) connectCellJetStream(cell *natsCell) error {
	server.logger.Noticef("connecting to JetStream, cluster %s", cell.displayName())

	var opts []nats.JSOpt
	c := server.config.JetStream
	if c.MaxWait > 0 {
		opts = append(opts, nats.MaxWait(time.Duration(c.MaxWait)*time.Millisecond))
	}
	if c.PublishAsyncMaxPending > 0 {
		opts = append(opts, nats.PublishAsyncMaxPending(c.PublishAsyncMaxPending))
	}

	js, err := cell.nc.JetStream(opts...)
	if err != nil {
		return err
	}
	cell.js = js
	return nil
}

func (server *NATSKafkaBridge) connectToJetStream() error {
	server.natsLock.Lock()
	defer server.natsLock.Unlock()

	needed := false
	for _, cell := range server.cells {
		if !server.cellHasJetStream(cell.config.Name) {
			continue
		}
		needed = true
		if cell.js != nil || cell.nc == nil {
			// a cell that was unreachable at startup gets its JetStream
			// context from redialCell once the cluster is reachable
			continue
		}
		if err := server.connectCellJetStream(cell); err != nil {
			return err
		}
	}

	if !needed {
		server.logger.Noticef("skipping JetStream connection, not configured")
	}
	return nil
}

// redialCell re-establishes the connection for a cell whose NATS client is
// missing or reached the Closed state. A closed client never reconnects on
// its own, so the reconnect timer uses this before restarting the cell's
// connectors. A no-op when the client is alive and handling its own
// reconnects and the JetStream context (when needed) is in place.
func (server *NATSKafkaBridge) redialCell(name string) error {
	server.natsLock.Lock()
	cell, ok := server.cells[name]
	if !ok {
		server.natsLock.Unlock()
		return fmt.Errorf("unknown nats cluster %q", name)
	}
	needDial := cell.nc == nil || cell.nc.IsClosed()
	if needDial {
		// make sure nobody publishes through the old context while we dial
		cell.js = nil
	}
	server.natsLock.Unlock()

	if needDial {
		// dial outside the natsLock so a slow dial of a dead cluster cannot
		// stall message flow on the healthy cells
		nc, err := server.dialCell(cell)
		if err != nil {
			return err
		}

		server.natsLock.Lock()
		if cell.nc != nil && !cell.nc.IsClosed() {
			// someone else re-established the connection while we dialed
			server.natsLock.Unlock()
			nc.Close()
			return nil
		}
		cell.nc = nc
		server.natsLock.Unlock()
	}

	server.natsLock.Lock()
	defer server.natsLock.Unlock()
	if cell.js == nil && server.cellHasJetStream(cell.config.Name) {
		return server.connectCellJetStream(cell)
	}
	return nil
}
