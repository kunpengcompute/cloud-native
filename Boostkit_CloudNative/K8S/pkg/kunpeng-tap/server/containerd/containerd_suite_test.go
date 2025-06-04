package containerd_test

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

func TestContainerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Containerd Suite")
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
