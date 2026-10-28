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

package tracking

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/google/uuid"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/smap"
)

type Connection struct {
	net.Conn

	counter *atomic.Int64
	key     string

	m *smap.Map[*Connection]
}

func NewConnection(conn net.Conn, counter *atomic.Int64, m *smap.Map[*Connection]) *Connection {
	counter.Add(1)

	c := &Connection{
		Conn:    conn,
		counter: counter,
		m:       m,
	}

	if m != nil {
		c.key = uuid.New().String()

		m.Insert(c.key, c)
	}

	return c
}

func (c *Connection) Reset() error {
	var errs []error

	// This forces the connection to close with RST.
	err := c.Conn.(*net.TCPConn).SetLinger(0)
	if err != nil {
		errs = append(errs, err)
	}

	err = c.Close()
	if err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (c *Connection) Close() error {
	err := c.Conn.Close()
	if err != nil {
		return err
	}

	c.counter.Add(-1)

	if c.m != nil {
		c.m.Remove(c.key)
	}

	return nil
}
