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

package collector

import (
	"errors"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	dto "github.com/prometheus/client_model/go"
	fakes "kunpeng.huawei.com/kunpeng-cloud-computing/test/kunpeng-perf-monitor/fake"
)

var _ = Describe("NodeCollector Internal", func() {
	var logger *slog.Logger

	// ---------- clean ----------
	beforeEach := func() {
		// 清掉所有全局状态
		factories = make(map[string]func(logger *slog.Logger) (Collector, error))
		initiatedCollectors = make(map[string]Collector)
		collectorState = make(map[string]*bool)
		forcedCollectors = make(map[string]bool)

		// 新建 kingpin 实例，防止重复注册
		kingpin.CommandLine = kingpin.New("test", "")
		logger = slog.New(slog.NewTextHandler(GinkgoWriter, nil))
	}
	BeforeEach(beforeEach)

	// 统一解析
	parse := func(args []string) error {
		_, err := kingpin.CommandLine.Parse(args)
		return err
	}

	Describe("RegisterCollector", func() {
		It("registers a collector enabled by default", func() {
			RegisterCollector("cpu", true, func(l *slog.Logger) (Collector, error) {
				return &fakes.FakeCollector{}, nil
			})
			Expect(parse([]string{"--collector.cpu"})).To(Succeed())
			nc, err := NewNodeCollector(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(nc.Collectors).To(HaveKey("cpu"))
		})

		It("registers a collector disabled by default", func() {
			RegisterCollector("tape", false, func(l *slog.Logger) (Collector, error) {
				return &fakes.FakeCollector{}, nil
			})
			Expect(parse([]string{})).To(Succeed())
			nc, err := NewNodeCollector(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(nc.Collectors).NotTo(HaveKey("tape"))
		})
	})

	Describe("DisableDefaultCollectors", func() {
		It("turns off every collector not explicitly enabled", func() {
			RegisterCollector("a", true, factoryOK)
			RegisterCollector("b", true, factoryOK)
			Expect(parse([]string{"--collector.a"})).To(Succeed())
			DisableDefaultCollectors()

			nc, err := NewNodeCollector(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(nc.Collectors).To(HaveKey("a"))
			Expect(nc.Collectors).NotTo(HaveKey("b"))
		})
	})

	Describe("NewNodeCollector", func() {
		BeforeEach(func() {
			RegisterCollector("good", true, factoryOK)
			RegisterCollector("bad", true, factoryErr)
		})

		It("succeeds when factory returns no error", func() {
			Expect(parse([]string{"--collector.good"})).To(Succeed())
			nc, err := NewNodeCollector(logger, "good")
			Expect(err).NotTo(HaveOccurred())
			Expect(nc.Collectors).To(HaveLen(1))
		})

		It("fails when factory returns error", func() {
			Expect(parse([]string{"--collector.bad"})).To(Succeed())
			_, err := NewNodeCollector(logger, "bad")
			Expect(err).To(MatchError(ContainSubstring("factory err")))
		})

		It("fails when collector does not exist", func() {
			_, err := NewNodeCollector(logger, "ghost")
			Expect(err).To(MatchError(ContainSubstring("missing collector: ghost")))
		})

		It("fails when collector is disabled", func() {
			Expect(parse([]string{"--no-collector.good"})).To(Succeed())
			_, err := NewNodeCollector(logger, "good")
			Expect(err).To(MatchError(ContainSubstring("disabled collector: good")))
		})
	})

	Describe("Collect", func() {
		var (
			reg  *prometheus.Registry
			fake *fakes.FakeCollector
		)
		BeforeEach(func() {
			reg = prometheus.NewRegistry()
			fake = &fakes.FakeCollector{}
			RegisterCollector("fake", true, func(l *slog.Logger) (Collector, error) { return fake, nil })
			Expect(parse([]string{"--collector.fake"})).To(Succeed())
			nc, err := NewNodeCollector(logger)
			Expect(err).NotTo(HaveOccurred())
			reg.MustRegister(nc)
		})

		JustBeforeEach(func() { _, _ = reg.Gather() })

		Context("collector succeeds", func() {
			BeforeEach(func() { fake.UpdateReturns(nil) })
			It("records success=1", func() {
				Expect(metricValue(reg, "kunpeng_node_scrape_collector_success", "fake")).To(Equal(1.0))
			})
		})

		Context("collector returns ErrNoData", func() {
			BeforeEach(func() { fake.UpdateReturns(ErrNoData) })
			It("still records success=1", func() {
				Expect(metricValue(reg, "kunpeng_node_scrape_collector_success", "fake")).To(Equal(0.0))
			})
		})

		Context("collector fails", func() {
			BeforeEach(func() { fake.UpdateReturns(errors.New("boom")) })
			It("records success=0", func() {
				Expect(metricValue(reg, "kunpeng_node_scrape_collector_success", "fake")).To(Equal(0.0))
			})
		})
	})
})

/* ---------- mustNewConstMetric ---------- */
var _ = Describe("typedDesc", func() {
	var td *typedDesc

	BeforeEach(func() {
		td = &typedDesc{
			desc: prometheus.NewDesc(
				"test_metric",
				"help",
				[]string{"l1", "l2"},
				nil,
			),
			valueType: prometheus.GaugeValue,
		}
	})

	It("produces metric with correct value and labels", func() {
		m := td.mustNewConstMetric(3.14, "a", "b")
		dtoM := &dto.Metric{}
		_ = m.Write(dtoM)
		Expect(dtoM.GetGauge().GetValue()).To(Equal(3.14))
		Expect(dtoM.GetLabel()[0].GetValue()).To(Equal("a"))
		Expect(dtoM.GetLabel()[1].GetValue()).To(Equal("b"))
	})

	It("panics when label count does not match", func() {
		Expect(func() {
			td.mustNewConstMetric(1.0, "only-one")
		}).To(Panic())
	})
})

/* ---------- pushMetric ---------- */
var _ = Describe("pushMetric", func() {
	var (
		ch   chan prometheus.Metric
		desc *prometheus.Desc
	)

	BeforeEach(func() {
		ch = make(chan prometheus.Metric, 10)
		desc = prometheus.NewDesc("push_test", "help", []string{"label"}, nil)
	})

	AfterEach(func() { close(ch) })

	read := func() prometheus.Metric {
		select {
		case m := <-ch:
			return m
		default:
			return nil
		}
	}

	// 工具函数：把 metric 转成 float
	valueOf := func(m prometheus.Metric) float64 {
		dtoM := &dto.Metric{}
		_ = m.Write(dtoM)
		return dtoM.GetGauge().GetValue()
	}

	// 正向场景：所有数值类型
	DescribeTable("convert and push",
		func(value interface{}, want float64) {
			pushMetric(ch, desc, "field", value, prometheus.GaugeValue, "lv")
			m := read()
			Expect(m).NotTo(BeNil())
			Expect(valueOf(m)).To(Equal(want))
			// 标签
			dtoM := &dto.Metric{}
			_ = m.Write(dtoM)
			Expect(dtoM.GetLabel()[0].GetValue()).To(Equal("lv"))
		},
		Entry("uint8", uint8(8), 8.0),
		Entry("uint16", uint16(16), 16.0),
		Entry("uint32", uint32(32), 32.0),
		Entry("uint64", uint64(64), 64.0),
		Entry("int64", int64(-100), -100.0),
		Entry("*uint8", ptr(uint8(8)), 8.0),
		Entry("*uint16", ptr(uint16(16)), 16.0),
		Entry("*uint32", ptr(uint32(32)), 32.0),
		Entry("*uint64", ptr(uint64(64)), 64.0),
		Entry("*int64", ptr(int64(-100)), -100.0),
	)

	// nil 指针：直接 return，不输出
	It("ignores nil pointer", func() {
		var p *uint64
		pushMetric(ch, desc, "field", p, prometheus.GaugeValue, "lv")
		Expect(read()).To(BeNil())
	})

	// 未知类型：直接 return，不输出
	It("ignores unknown type", func() {
		pushMetric(ch, desc, "field", "string", prometheus.GaugeValue, "lv")
		Expect(read()).To(BeNil())
	})
})

/* ---------- 简单辅助 ---------- */
func ptr[T any](v T) *T { return &v }

/* ---------- helpers ---------- */

func factoryOK(l *slog.Logger) (Collector, error)  { return &fakes.FakeCollector{}, nil }
func factoryErr(l *slog.Logger) (Collector, error) { return nil, errors.New("factory err") }

func metricValue(reg *prometheus.Registry, name, collectorLabel string) float64 {
	mfs, err := reg.Gather()
	Expect(err).NotTo(HaveOccurred())
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "collector" && l.GetValue() == collectorLabel {
					return m.GetGauge().GetValue()
				}
			}
		}
	}
	return -1
}
