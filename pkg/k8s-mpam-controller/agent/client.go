/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"time"

	"k8s.io/klog"
)

func startClient(server, caFile, certFile, keyFile, serverName string) error {
	tlsCfg, err := createTLSConfig(caFile, certFile, keyFile, serverName)
	if err != nil {
		return err
	}

	conn, err := establishConnection(server, tlsCfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	return handleClientCommunication(conn)
}

func createTLSConfig(caFile, certFile, keyFile, serverName string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair: %v", err)
	}

	caCert, err := ioutil.ReadFile(path.Clean(caFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read root certificate file: %v", err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("failed to add certificate from ca.crt")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func establishConnection(server string, tlsCfg *tls.Config) (*tls.Conn, error) {
	klog.Info("Connecting to server: " + server)
	conn, err := tls.Dial("tcp", server, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("%v", err.Error())
	}
	klog.Info("Connected to ", conn.RemoteAddr())

	if _, err = conn.Write([]byte(nodeName)); err != nil {
		return nil, fmt.Errorf("failed to send node name: %v", err)
	}

	return conn, nil
}

func handleClientCommunication(conn *tls.Conn) error {
	ticker := time.NewTicker(1 * time.Second * 30)
	defer ticker.Stop()

	buf := make([]byte, 128)
	pos := 0

	for {
		select {
		case <-ticker.C:
			if err := sendHeartbeat(conn); err != nil {
				return err
			}
		default:
			if err := processIncomingData(conn, buf, &pos); err != nil {
				return err
			}
		}
	}
}

func sendHeartbeat(conn *tls.Conn) error {
	if _, err := conn.Write([]byte("hello.")); err != nil {
		return fmt.Errorf("heartbeat failed: %v", err)
	}
	return nil
}

func processIncomingData(conn *tls.Conn, buf []byte, pos *int) error {
	size, err := conn.Read(buf[*pos:])
	if err != nil {
		klog.Errorf("failed to read: %v", err)
		return err
	}

	size += *pos
	*pos = size

	if size <= 8 {
		return nil
	}

	*pos = 0
	return processBufferedData(buf, size, pos)
}

func processBufferedData(buf []byte, size int, pos *int) error {
	for {
		if size <= 8 {
			*pos = size
			return nil
		}

		jsize, err := strconv.Atoi(string(buf[*pos : *pos+8]))
		if err != nil {
			return err
		}

		if size < 8+jsize {
			copy(buf, buf[*pos:*pos+size])
			*pos = size
			return nil
		}

		if err := processSingleRecord(buf[*pos+8 : *pos+8+jsize]); err != nil {
			return err
		}

		*pos += 8 + jsize
		size -= 8 + jsize
	}
}

func processSingleRecord(data []byte) error {
	m := make(map[string]string)
	klog.Infof("Received: %s", data)

	if err := json.Unmarshal(data, &m); err != nil {
		klog.Errorf("failed to parse data: %v", data)
		return nil // 保持原逻辑，即使解析失败也不返回错误
	}

	uid, ok := m["UID"]
	rcgroup, ok2 := m["rcgroup"]
	if !ok || !ok2 {
		return fmt.Errorf("wrong data: missing required fields")
	}

	assignControlGroup(uid, rcgroup)
	return nil
}
