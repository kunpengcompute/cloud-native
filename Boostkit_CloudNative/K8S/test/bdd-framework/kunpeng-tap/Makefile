# kunpeng-tap BDD 测试 Makefile

# 变量定义
PYTHON := python3
PIP := pip3
PYTEST := pytest
TEST_DIR := .
FRAMEWORK_DIR := ..
REPORTS_DIR := test-reports

# 默认目标
.PHONY: help
help:
	@echo "kunpeng-tap BDD 测试命令:"
	@echo "  setup              - 安装依赖和设置环境"
	@echo "  test               - 运行所有测试"
	@echo "  test-topology      - 运行拓扑感知测试"
	@echo "  test-containerd    - 运行 containerd 集成测试"
	@echo "  test-smoke         - 运行冒烟测试"
	@echo "  test-e2e           - 运行端到端测试"
	@echo "  test-debug         - 运行调试模式测试"
	@echo "  report             - 生成 HTML 测试报告"
	@echo "  clean              - 清理测试环境"
	@echo "  lint               - 代码检查"
	@echo "  format             - 代码格式化"

# 环境设置
.PHONY: setup
setup:
	@echo "设置 kunpeng-tap BDD 测试环境..."
	$(PIP) install -r $(FRAMEWORK_DIR)/requirements.txt
	mkdir -p $(REPORTS_DIR)
	@echo "环境设置完成"

# 测试目标
.PHONY: test
test: setup
	@echo "运行所有 kunpeng-tap BDD 测试..."
	$(PYTEST) $(TEST_DIR) -v --html=$(REPORTS_DIR)/report.html --self-contained-html

.PHONY: test-topology
test-topology: setup
	@echo "运行拓扑感知测试..."
	$(PYTEST) test_topology_aware_deployment.py -v

.PHONY: test-containerd
test-containerd: setup
	@echo "运行 containerd 集成测试..."
	$(PYTEST) test_containerd_integration.py -v

.PHONY: test-smoke
test-smoke: setup
	@echo "运行冒烟测试..."
	$(PYTEST) test_smoke.py -v

.PHONY: test-e2e
test-e2e: setup
	@echo "运行端到端测试..."
	$(PYTEST) -m e2e -v --html=$(REPORTS_DIR)/e2e-report.html --self-contained-html

.PHONY: test-debug
test-debug: setup
	@echo "运行调试模式测试..."
	LOG_LEVEL=DEBUG CLEANUP_ON_FAILURE=false $(PYTEST) $(TEST_DIR) -v -s

# 报告生成
.PHONY: report
report: setup
	@echo "生成测试报告..."
	$(PYTEST) $(TEST_DIR) --html=$(REPORTS_DIR)/kunpeng-tap-report.html --self-contained-html --cov=support --cov-report=html:$(REPORTS_DIR)/coverage

# 清理
.PHONY: clean
clean:
	@echo "清理测试环境..."
	rm -rf $(REPORTS_DIR)
	rm -rf __pycache__
	rm -rf .pytest_cache
	rm -rf .coverage
	find . -name "*.pyc" -delete
	find . -name "*.pyo" -delete
	@echo "清理完成"

# 代码质量
.PHONY: lint
lint:
	@echo "运行代码检查..."
	flake8 step_definitions/ support/ --max-line-length=120
	pylint step_definitions/ support/ --disable=C0114,C0115,C0116

.PHONY: format
format:
	@echo "格式化代码..."
	black step_definitions/ support/ --line-length=120
	isort step_definitions/ support/

# 特定场景测试
.PHONY: test-cpu-intensive
test-cpu-intensive: setup
	@echo "运行 CPU 密集型容器测试..."
	$(PYTEST) -k "CPU-intensive" -v

.PHONY: test-numa-affinity
test-numa-affinity: setup
	@echo "运行 NUMA 亲和性测试..."
	$(PYTEST) -k "NUMA" -v

.PHONY: test-nri-plugin
test-nri-plugin: setup
	@echo "运行 NRI 插件测试..."
	$(PYTEST) -k "NRI" -v

# 环境验证
.PHONY: verify-env
verify-env:
	@echo "验证测试环境..."
	@echo "检查 Kubernetes 连接..."
	kubectl cluster-info
	@echo "检查 kunpeng-tap 服务..."
	systemctl is-active kunpeng-tap || echo "kunpeng-tap 服务未运行"
	@echo "检查容器运行时..."
	kubectl get nodes -o wide
	@echo "环境验证完成"

# 并行测试
.PHONY: test-parallel
test-parallel: setup
	@echo "运行并行测试..."
	$(PYTEST) $(TEST_DIR) -v -n auto --html=$(REPORTS_DIR)/parallel-report.html --self-contained-html

# 性能测试
.PHONY: test-performance
test-performance: setup
	@echo "运行性能测试..."
	$(PYTEST) -m performance -v --html=$(REPORTS_DIR)/performance-report.html --self-contained-html

# 回归测试
.PHONY: test-regression
test-regression: setup
	@echo "运行回归测试..."
	$(PYTEST) -m regression -v --html=$(REPORTS_DIR)/regression-report.html --self-contained-html
