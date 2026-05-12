"""BDD scenarios for MPAM controller core lifecycle."""

import pytest
from pytest_bdd import scenarios

from step_definitions.common_steps import cleanup_test_resources, collect_failure_diagnostics
from step_definitions.common_steps import *  # noqa: F401,F403

scenarios("features/qos_controller.feature")


@pytest.fixture(scope="function", autouse=True)
def cleanup_after_test(request):
    """Always cleanup; dump diagnostics when scenario fails."""
    yield

    rep = getattr(request.node, "rep_call", None)
    if rep is not None and rep.failed:
        collect_failure_diagnostics()

    cleanup_test_resources()
