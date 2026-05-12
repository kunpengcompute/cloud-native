"""k8s-mpam-controller BDD pytest configuration."""

import os
import sys
from pathlib import Path

import pytest

# Add bdd framework root (test/bdd-framework) into import path.
framework_path = Path(__file__).parent.parent
sys.path.insert(0, str(framework_path))

from conftest import *  # noqa: F401,F403


@pytest.fixture(scope="session")
def test_config():
    """Base test config for this suite.

    Keep a local definition to avoid relying on cross-conftest import behavior.
    """
    return {
        "cluster_config": os.getenv("KUBECONFIG", "~/.kube/config"),
        "namespace": os.getenv("TEST_NAMESPACE", "bdd-test"),
        "test_timeout": int(os.getenv("TEST_TIMEOUT", "30")),
        "cleanup_on_failure": os.getenv("CLEANUP_ON_FAILURE", "true").lower() == "true",
        "metrics_endpoint": os.getenv("METRICS_ENDPOINT", "http://localhost:8080/metrics"),
        "log_level": os.getenv("LOG_LEVEL", "INFO"),
    }


@pytest.fixture(scope="session")
def mpam_test_config(test_config):
    """MPAM suite specific runtime configuration."""
    cfg = dict(test_config)
    cfg.update(
        {
            "namespace": os.getenv("MPAM_E2E_NAMESPACE", "mpam-e2e"),
            "operator_namespace": os.getenv("MPAM_OPERATOR_NAMESPACE", "qos-system"),
            "operator_image": os.getenv("MPAM_OPERATOR_IMAGE", "k8s-mpam-controller:0.1.0"),
            "pod_image": os.getenv("MPAM_E2E_POD_IMAGE", "busybox:1.36"),
            "node_selector": os.getenv("MPAM_E2E_NODE_SELECTOR", ""),
            "reconcile_timeout_seconds": int(os.getenv("MPAM_E2E_TIMEOUT", "180")),
            "poll_interval_seconds": int(os.getenv("MPAM_E2E_POLL_INTERVAL", "3")),
        }
    )
    return cfg


@pytest.hookimpl(hookwrapper=True, tryfirst=True)
def pytest_runtest_makereport(item, call):
    """Expose test stage report to fixtures for failure diagnostics."""
    outcome = yield
    rep = outcome.get_result()
    setattr(item, f"rep_{rep.when}", rep)
