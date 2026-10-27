// Copyright (c) 2023-2026 e2b Devs Inc.
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

package pool

import "context"

type destinationContextKey struct{}

func WithDestination(ctx context.Context, d *Destination) context.Context {
	return context.WithValue(ctx, destinationContextKey{}, d)
}

func getDestination(ctx context.Context) (*Destination, bool) {
	v := ctx.Value(destinationContextKey{})
	if v == nil {
		return nil, false
	}

	d, ok := v.(*Destination)
	if !ok {
		return nil, false
	}

	return d, true
}
