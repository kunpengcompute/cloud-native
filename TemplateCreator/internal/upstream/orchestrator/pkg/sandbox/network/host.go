// Copyright 2023 FoundryLabs, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package network

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/utils"
)

// Host loopback interface name
const loopbackInterface = "lo"

// Host default gateway name
var defaultGateway = utils.Must(getDefaultGateway(context.Background()))

//	func getDefaultGateway() (string, error) {
//		route, err := exec.Command(
//			"sh",
//			"-c",
//			"ip route show default | awk '{print $5}'",
//		).Output()
//		if err != nil {
//			return "", fmt.Errorf("error fetching default gateway: %w", err)
//		}
//
//		return string(route), nil
//	}
func getDefaultGateway(ctx context.Context) (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return "", fmt.Errorf("error fetching routes: %w", err)
	}

	for _, route := range routes {
		// 0.0.0.0/0
		if route.Dst.String() == "0.0.0.0/0" && route.Gw != nil {
			logger.L().Info(ctx, "default gateway", zap.String("gateway", route.Gw.String()))

			link, linkErr := netlink.LinkByIndex(route.LinkIndex)

			if linkErr != nil {
				return "", fmt.Errorf("error fetching interface for default gateway: %w", linkErr)
			}

			return link.Attrs().Name, nil
		}
	}

	return "", fmt.Errorf("cannot find default gateway")
}
