/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	ProxyCreateContainerRequest = "CreateContainer"
	ProxyStartContainerRequest  = "StartContainer"
	ProxyUpdateContainerRequest = "UpdateContainer"
	ProxyStopContainerRequest   = "StopContainer"
	ProxyRemoveContainerRequest = "RemoveContainer"
)

var (
	ProxyHealthz = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tap_proxy_up",
		Help: "Health status of the proxy (1 = UP, 0 = DOWN).",
	})

	ProxyRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tap_proxy_request_duration_seconds",
			Help:    "Duration of requests in proxy in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"request"},
	)

	HookRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tap_hook_request_duration_seconds",
			Help:    "Duration of requests in hook in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"hook"},
	)
)
