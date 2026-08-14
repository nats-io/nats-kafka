/*
 * Copyright 2026 The NATS Authors
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
	_ "embed"
	"net/http"
)

//go:embed dashboard/index.html
var dashboardHTML []byte

// HandleDashboard serves the embedded, self-contained monitoring dashboard.
// The page polls /varz and renders traffic rates client-side.
func (server *NATSKafkaBridge) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	server.statsLock.Lock()
	server.httpReqStats[DashboardPath]++
	server.statsLock.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(dashboardHTML)
}
