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

package docker_test

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	interval           = 1 * time.Second
	timeout            = 10 * time.Second
	utSocketPathPrefix = "/tmp/tap-test"
)

func TestDocker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker Suite")
}

var _ = BeforeSuite(func() {
	_, err := os.Stat(utSocketPathPrefix)
	if err == nil {
		//已经存在，清除
		err := os.RemoveAll(utSocketPathPrefix)
		Expect(err).To(BeNil())
	}

	err = os.Mkdir(utSocketPathPrefix, 0755)
	Expect(err).To(BeNil())

})

var _ = AfterSuite(func() {
	os.RemoveAll(utSocketPathPrefix)
})
