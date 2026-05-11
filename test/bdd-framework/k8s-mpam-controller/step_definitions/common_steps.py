"""BDD steps for k8s-mpam-controller E2E lifecycle tests."""

from __future__ import annotations

import subprocess
import time
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import yaml
from kubernetes import client, config
from kubernetes.client.rest import ApiException
from kubernetes.stream import stream
from pytest_bdd import given, parsers, then, when

REPO_ROOT = Path(__file__).resolve().parents[4]
CRD_MANIFEST = REPO_ROOT / "config/k8s-mpam-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml"
DAEMONSET_MANIFEST = REPO_ROOT / "config/k8s-mpam-controller-config/samples/mpam-controller-daemonset-v1alpha1.yaml"

OPERATOR_LABEL_SELECTOR = "app=mpam-controller"
RUN_LABEL_KEY = "mpam-e2e-run"
GROUP_LABEL_KEY = "qos.kunpeng.huawei.com/group"
WORKLOAD_CLASS_LABEL_KEY = "qos.kunpeng.huawei.com/workload-class"
WORKLOAD_CLASS_OFFLINE = "offline"


test_context: Dict[str, object] = {
    "run_id": f"mpam-e2e-{int(time.time())}",
    "namespace": "mpam-e2e",
    "operator_namespace": "mpam-system",
    "operator_image": "k8s-mpam-controller:0.1.0",
    "pod_image": "busybox:1.36",
    "node_selector": "",
    "timeout": 180,
    "poll_interval": 3,
    "operator_installed": False,
    "target_node_name": "",
    "controller_pod_name": "",
    "cgroup_driver": "",
    "policies": set(),
    "pods": set(),
    "current_group": "",
    "schemata_before_update": "",
    "last_apply_error": "",
    "policy_specs": {},
    "node_selector_match": {},
    "node_selector_mismatch": {},
}


k8s_core_v1: Optional[client.CoreV1Api] = None
k8s_apps_v1: Optional[client.AppsV1Api] = None


def _ensure_k8s_clients() -> None:
    global k8s_core_v1, k8s_apps_v1
    if k8s_core_v1 is not None and k8s_apps_v1 is not None:
        return
    try:
        config.load_incluster_config()
    except Exception:
        config.load_kube_config()
    k8s_core_v1 = client.CoreV1Api()
    k8s_apps_v1 = client.AppsV1Api()


def _run_kubectl(args: List[str], stdin: Optional[str] = None, check: bool = True) -> subprocess.CompletedProcess:
    cmd = ["kubectl", *args]
    return subprocess.run(
        cmd,
        input=stdin,
        text=True,
        capture_output=True,
        check=check,
    )


def _apply_yaml_docs(docs: List[dict]) -> None:
    payload = yaml.safe_dump_all(docs, sort_keys=False)
    _run_kubectl(["apply", "-f", "-"], stdin=payload, check=True)


def _parse_selector(selector: str) -> Tuple[str, str]:
    raw = selector.strip()
    if not raw:
        return "", ""
    if "=" not in raw:
        raise ValueError(f"invalid node selector {selector!r}, expected key=value")
    key, value = raw.split("=", 1)
    return key.strip(), value.strip()


def _refresh_controller_pod() -> str:
    _ensure_k8s_clients()
    ns = str(test_context["operator_namespace"])
    node_name = str(test_context.get("target_node_name", ""))
    pods = k8s_core_v1.list_namespaced_pod(namespace=ns, label_selector=OPERATOR_LABEL_SELECTOR).items
    running = [p for p in pods if p.status.phase == "Running"]
    if node_name:
        running = [p for p in running if p.spec.node_name == node_name]
    if not running:
        raise RuntimeError("no running mpam-controller pod found")
    pod_name = running[0].metadata.name
    test_context["controller_pod_name"] = pod_name
    if not test_context.get("target_node_name"):
        test_context["target_node_name"] = running[0].spec.node_name
    return pod_name


def _exec_in_controller(command: str) -> str:
    _ensure_k8s_clients()
    ns = str(test_context["operator_namespace"])
    pod_name = _refresh_controller_pod()
    return stream(
        k8s_core_v1.connect_get_namespaced_pod_exec,
        name=pod_name,
        namespace=ns,
        command=["sh", "-c", command],
        stderr=True,
        stdin=False,
        stdout=True,
        tty=False,
    )


def _detect_cgroup_driver() -> str:
    cached = str(test_context.get("cgroup_driver", "")).strip()
    if cached in {"systemd", "cgroupfs"}:
        return cached

    out = _exec_in_controller(
        "if [ -d /sys/fs/cgroup/cpu/kubepods.slice ]; then echo systemd; "
        "elif [ -d /sys/fs/cgroup/cpu/kubepods ]; then echo cgroupfs; "
        "else echo unknown; fi"
    ).strip()
    if out not in {"systemd", "cgroupfs"}:
        raise RuntimeError(f"unsupported cgroup driver for test: {out}")
    test_context["cgroup_driver"] = out
    return out


def _wait_until(desc: str, fn, timeout: int, interval: int) -> None:
    deadline = time.time() + timeout
    last_error = None
    while time.time() < deadline:
        try:
            if fn():
                return
        except Exception as exc:  # pragma: no cover
            last_error = exc
        time.sleep(interval)
    if last_error is not None:
        raise RuntimeError(f"timeout waiting for {desc}, last error: {last_error}")
    raise RuntimeError(f"timeout waiting for {desc}")


def _ensure_namespace(namespace: str) -> None:
    _ensure_k8s_clients()
    try:
        k8s_core_v1.read_namespace(namespace)
        return
    except ApiException as exc:
        if exc.status != 404:
            raise

    body = client.V1Namespace(metadata=client.V1ObjectMeta(name=namespace))
    k8s_core_v1.create_namespace(body)


def _default_policy_doc(name: str) -> dict:
    return {
        "apiVersion": "qos.kunpeng.huawei.com/v1alpha1",
        "kind": "QoSPolicy",
        "metadata": {"name": name},
        "spec": {
            "mb": {"hdl": 1, "pri": 3, "min": 0, "max": 100},
            "l3": {"pri": 0, "min": 0, "max": 100, "ways": 2},
        },
    }


def _select_node_label_for_policy_selector() -> Tuple[str, str]:
    _ensure_k8s_clients()
    node_name = str(test_context.get("target_node_name", "")).strip()
    if not node_name:
        _refresh_controller_pod()
        node_name = str(test_context.get("target_node_name", "")).strip()
    if not node_name:
        raise RuntimeError("target node name is empty")

    node = k8s_core_v1.read_node(node_name)
    labels = node.metadata.labels or {}

    preferred_keys = [
        "kubernetes.io/hostname",
        "kubernetes.io/arch",
        "kubernetes.io/os",
    ]
    for key in preferred_keys:
        value = labels.get(key, "").strip()
        if value:
            return key, value

    for key, value in labels.items():
        if key and str(value).strip():
            return key, str(value).strip()

    raise RuntimeError(f"node {node_name} has no usable labels for selector")


def _mismatch_selector_from_match(key: str, value: str) -> Dict[str, str]:
    mismatched = f"{value}-mpam-e2e-miss"
    return {key: mismatched}


def _policy_with_selector(name: str, selector: Dict[str, str]) -> dict:
    doc = _default_policy_doc(name)
    doc["spec"]["nodeSelector"] = selector
    return doc


def _l3_ways_to_hex_mask(ways: int) -> str:
    if ways < 1:
        raise ValueError(f"invalid l3 ways: {ways}")
    return format((1 << ways) - 1, "x").lower()


def _expected_schemata_values_from_policy_spec(spec: Dict[str, Any]) -> Dict[str, str]:
    mb = spec.get("mb", {})
    l3 = spec.get("l3", {})
    return {
        "MBHDL": str(mb.get("hdl", 1)),
        "MBPRI": str(mb.get("pri", 3)),
        "L3PRI": str(l3.get("pri", 0)),
        "MBMIN": str(mb.get("min", 0)),
        "L3MIN": str(l3.get("min", 0)),
        "L3MAX": str(l3.get("max", 100)),
        "MB": str(mb.get("max", 100)),
        "L3": _l3_ways_to_hex_mask(int(l3.get("ways", 2))),
    }


def _parse_schemata(content: str) -> Dict[str, Dict[str, str]]:
    result: Dict[str, Dict[str, str]] = {}
    lines = [line.strip() for line in content.splitlines() if line.strip()]
    for line in lines:
        if ":" not in line:
            continue
        key, payload = line.split(":", 1)
        key = key.strip()
        pairs: Dict[str, str] = {}
        for item in payload.split(";"):
            kv = item.strip()
            if not kv or "=" not in kv:
                continue
            item_id, value = kv.split("=", 1)
            pairs[item_id.strip()] = value.strip()
        result[key] = pairs
    return result


def _schemata_matches_expected(content: str, expected: Dict[str, str]) -> bool:
    parsed = _parse_schemata(content)
    for key, exp_value in expected.items():
        assignments = parsed.get(key)
        if not assignments:
            return False
        for actual in assignments.values():
            if key == "L3":
                # L3 mask is hex string in schemata. Compare by numeric value
                # so values like "000f" and "f" are considered equal.
                try:
                    if int(actual, 16) != int(exp_value, 16):
                        return False
                except ValueError:
                    return False
                continue

            # Decimal fields should be compared by numeric value to avoid
            # false negatives caused by leading zeros (e.g. "00001" vs "1").
            try:
                if int(actual, 10) != int(exp_value, 10):
                    return False
            except ValueError:
                return False
    return True


def _invalid_policy_doc(name: str) -> dict:
    return {
        "apiVersion": "qos.kunpeng.huawei.com/v1alpha1",
        "kind": "QoSPolicy",
        "metadata": {"name": name},
        "spec": {
            "mb": {"hdl": 2, "pri": 9, "min": -1, "max": 150},
            "l3": {"pri": 5, "min": -1, "max": 101, "ways": 0},
        },
    }


def _create_test_pod_doc(name: str, group_name: str, extra_labels: Optional[Dict[str, str]] = None) -> dict:
    namespace = str(test_context["namespace"])
    run_id = str(test_context["run_id"])
    node_name = str(test_context["target_node_name"])
    image = str(test_context["pod_image"])
    labels = {
        RUN_LABEL_KEY: run_id,
        GROUP_LABEL_KEY: group_name,
    }
    if extra_labels:
        labels.update(extra_labels)

    return {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
            "name": name,
            "namespace": namespace,
            "labels": labels,
        },
        "spec": {
            "nodeName": node_name,
            "restartPolicy": "Never",
            "containers": [
                {
                    "name": "workload",
                    "image": image,
                    "imagePullPolicy": "IfNotPresent",
                    "command": ["sh", "-c", "sleep 36000"],
                }
            ],
        },
    }


def _wait_pod_running(namespace: str, name: str) -> None:
    _ensure_k8s_clients()
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])

    def _is_running() -> bool:
        pod = k8s_core_v1.read_namespaced_pod(name=name, namespace=namespace)
        return pod.status.phase == "Running"

    _wait_until(f"pod {namespace}/{name} running", _is_running, timeout, interval)


def _read_group_schemata(group_name: str) -> str:
    data = _exec_in_controller(f"cat /sys/fs/resctrl/{group_name}/schemata")
    return data.strip()


def _group_exists(group_name: str) -> bool:
    out = _exec_in_controller(
        f"if [ -d /sys/fs/resctrl/{group_name} ]; then echo exists; else echo missing; fi"
    )
    return "exists" in out


def _group_tasks_non_empty(group_name: str) -> bool:
    out = _exec_in_controller(
        " ".join(
            [
                f"if [ -f /sys/fs/resctrl/{group_name}/tasks ]",
                f"&& grep -Eq '^[0-9]+$' /sys/fs/resctrl/{group_name}/tasks; then",
                "echo nonempty;",
                "else",
                "echo empty;",
                "fi",
            ]
        )
    )
    return "nonempty" in out


def _container_cpu_qos_path(pod: client.V1Pod, container_id: str) -> str:
    qos_class = str(pod.status.qos_class or "").lower()
    pod_uid = str(pod.metadata.uid)
    pod_uid_for_systemd = pod_uid.replace("-", "_")

    if "://" not in container_id:
        raise ValueError(f"invalid container id format: {container_id}")
    runtime, cid = container_id.split("://", 1)
    runtime = runtime.strip()
    cid = cid.strip()
    if not runtime or not cid:
        raise ValueError(f"invalid container id: {container_id}")

    if _detect_cgroup_driver() == "systemd":
        # systemd driver layout
        if qos_class == "burstable":
            qos_dir = "kubepods-burstable.slice"
            pod_dir = f"kubepods-burstable-pod{pod_uid_for_systemd}.slice"
        elif qos_class == "besteffort":
            qos_dir = "kubepods-besteffort.slice"
            pod_dir = f"kubepods-besteffort-pod{pod_uid_for_systemd}.slice"
        else:
            qos_dir = ""
            pod_dir = f"kubepods-pod{pod_uid_for_systemd}.slice"

        if runtime == "docker":
            container_dir = f"docker-{cid}.scope"
        elif runtime == "containerd":
            container_dir = f"cri-containerd-{cid}.scope"
        else:
            raise ValueError(f"unsupported container runtime: {runtime}")

        parts = ["/sys/fs/cgroup/cpu", "kubepods.slice"]
        if qos_dir:
            parts.append(qos_dir)
        parts.extend([pod_dir, container_dir, "cpu.qos_level"])
        return "/".join(parts)

    # cgroupfs driver layout
    if qos_class == "burstable":
        qos_dir = "burstable"
    elif qos_class == "besteffort":
        qos_dir = "besteffort"
    else:
        qos_dir = ""

    parts = ["/sys/fs/cgroup/cpu", "kubepods"]
    if qos_dir:
        parts.append(qos_dir)
    parts.extend([f"pod{pod_uid}", cid, "cpu.qos_level"])
    return "/".join(parts)


def _all_containers_cpu_qos_level(pod: client.V1Pod, expected_level: str) -> bool:
    statuses = pod.status.container_statuses or []
    if not statuses:
        return False

    for st in statuses:
        container_id = str(st.container_id or "").strip()
        if not container_id:
            return False
        target = _container_cpu_qos_path(pod, container_id)
        out = _exec_in_controller(f"cat {target} 2>/dev/null || true").strip()
        if out != expected_level:
            return False
    return True


@given("MPAM controller 已部署并且节点具备 resctrl")
def deploy_mpam_controller(mpam_test_config):
    _ensure_k8s_clients()
    test_context["namespace"] = mpam_test_config["namespace"]
    test_context["operator_namespace"] = mpam_test_config["operator_namespace"]
    test_context["operator_image"] = mpam_test_config["operator_image"]
    test_context["pod_image"] = mpam_test_config["pod_image"]
    test_context["node_selector"] = mpam_test_config["node_selector"]
    test_context["timeout"] = mpam_test_config["reconcile_timeout_seconds"]
    test_context["poll_interval"] = mpam_test_config["poll_interval_seconds"]

    _ensure_namespace(str(test_context["namespace"]))

    _run_kubectl(["apply", "-f", str(CRD_MANIFEST)], check=True)

    daemon_docs = list(yaml.safe_load_all(DAEMONSET_MANIFEST.read_text()))
    key, value = _parse_selector(str(test_context["node_selector"]))

    for doc in daemon_docs:
        if not isinstance(doc, dict):
            continue
        if doc.get("kind") != "DaemonSet":
            continue
        tpl_spec = doc.setdefault("spec", {}).setdefault("template", {}).setdefault("spec", {})
        containers = tpl_spec.get("containers", [])
        if containers:
            containers[0]["image"] = str(test_context["operator_image"])
        if key:
            tpl_spec["nodeSelector"] = {key: value}

    _apply_yaml_docs(daemon_docs)
    test_context["operator_installed"] = True

    op_ns = str(test_context["operator_namespace"])

    def _daemonset_ready() -> bool:
        ds = k8s_apps_v1.read_namespaced_daemon_set(name="mpam-controller", namespace=op_ns)
        desired = ds.status.desired_number_scheduled or 0
        ready = ds.status.number_ready or 0
        return desired > 0 and ready >= desired

    _wait_until(
        "mpam-controller daemonset ready",
        _daemonset_ready,
        int(test_context["timeout"]),
        int(test_context["poll_interval"]),
    )

    _refresh_controller_pod()
    test_context["target_node_name"] = str(test_context["target_node_name"])

    probe = _exec_in_controller(
        "if [ -f /sys/fs/resctrl/schemata ]; then echo ready; else echo not-ready; fi"
    )
    if "ready" not in probe:
        raise AssertionError("/sys/fs/resctrl/schemata not available in controller pod")


def _apply_policy(doc: dict) -> None:
    _apply_yaml_docs([doc])
    policies = test_context["policies"]
    if isinstance(policies, set):
        policies.add(doc["metadata"]["name"])
    specs = test_context.get("policy_specs")
    if isinstance(specs, dict):
        # Keep latest intended policy spec for schemata value assertions.
        specs[doc["metadata"]["name"]] = yaml.safe_load(yaml.safe_dump(doc["spec"]))


@when(parsers.parse('创建默认全局 QoSPolicy "{name}"'))
def create_default_policy(name: str):
    _apply_policy(_default_policy_doc(name))
    test_context["current_group"] = name


@when(parsers.parse('创建 cpu.qos_level 为 -1 的全局 QoSPolicy "{name}"'))
def create_cpu_qos_minus_one_policy(name: str):
    doc = _default_policy_doc(name)
    doc["spec"]["cpu"] = {"qosLevel": -1}
    _apply_policy(doc)
    test_context["current_group"] = name


@given(parsers.parse('创建 cpu.qos_level 为 -1 的全局 QoSPolicy "{name}"'))
def given_cpu_qos_minus_one_policy(name: str):
    create_cpu_qos_minus_one_policy(name)


@when(parsers.parse('创建仅在当前节点生效的 QoSPolicy "{name}"'))
def create_node_selector_match_policy(name: str):
    key, value = _select_node_label_for_policy_selector()
    selector = {key: value}
    test_context["node_selector_match"] = selector
    test_context["node_selector_mismatch"] = _mismatch_selector_from_match(key, value)
    _apply_policy(_policy_with_selector(name, selector))
    test_context["current_group"] = name


@given(parsers.parse('已创建仅在当前节点生效的 QoSPolicy "{name}"'))
def given_node_selector_match_policy(name: str):
    create_node_selector_match_policy(name)


@when(parsers.parse('创建不匹配当前节点的 QoSPolicy "{name}"'))
def create_node_selector_miss_policy(name: str):
    key, value = _select_node_label_for_policy_selector()
    mismatch = _mismatch_selector_from_match(key, value)
    test_context["node_selector_match"] = {key: value}
    test_context["node_selector_mismatch"] = mismatch
    _apply_policy(_policy_with_selector(name, mismatch))
    test_context["current_group"] = name


@given(parsers.parse('已创建默认全局 QoSPolicy "{name}"'))
def given_default_policy(name: str):
    create_default_policy(name)


@when(parsers.parse('创建带控制组标签 "{group_name}" 的测试 Pod "{pod_name}"'))
def create_group_pod(group_name: str, pod_name: str):
    _ensure_namespace(str(test_context["namespace"]))
    _apply_yaml_docs([_create_test_pod_doc(pod_name, group_name)])
    pods = test_context["pods"]
    if isinstance(pods, set):
        pods.add(pod_name)
    test_context["current_group"] = group_name


@when(parsers.parse('创建带离线标签和控制组标签 "{group_name}" 的测试 Pod "{pod_name}"'))
def create_offline_group_pod(group_name: str, pod_name: str):
    _ensure_namespace(str(test_context["namespace"]))
    _apply_yaml_docs(
        [
            _create_test_pod_doc(
                pod_name,
                group_name,
                extra_labels={WORKLOAD_CLASS_LABEL_KEY: WORKLOAD_CLASS_OFFLINE},
            )
        ]
    )
    pods = test_context["pods"]
    if isinstance(pods, set):
        pods.add(pod_name)
    test_context["current_group"] = group_name


@given(parsers.parse('已创建带控制组标签 "{group_name}" 的测试 Pod "{pod_name}"'))
def given_group_pod(group_name: str, pod_name: str):
    create_group_pod(group_name, pod_name)
    wait_pod_running(pod_name)


@then(parsers.parse('Pod "{pod_name}" 最终应为 Running'))
def wait_pod_running(pod_name: str):
    _wait_pod_running(str(test_context["namespace"]), pod_name)


@then(parsers.parse('Pod "{pod_name}" 的所有容器 cpu.qos_level 应为 "{expected_level}"'))
def assert_pod_cpu_qos_level(pod_name: str, expected_level: str):
    _ensure_k8s_clients()
    namespace = str(test_context["namespace"])
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])

    def _matched() -> bool:
        pod = k8s_core_v1.read_namespaced_pod(name=pod_name, namespace=namespace)
        return _all_containers_cpu_qos_level(pod, expected_level)

    _wait_until(
        f"pod {namespace}/{pod_name} cpu.qos_level={expected_level}",
        _matched,
        timeout,
        interval,
    )


@then(parsers.parse('resctrl 控制组 "{group_name}" 应该存在'))
def assert_group_exists(group_name: str):
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])
    _wait_until(
        f"resctrl group {group_name} exists",
        lambda: _group_exists(group_name),
        timeout,
        interval,
    )


@then(parsers.parse('resctrl 控制组 "{group_name}" 不应存在'))
def assert_group_not_exists(group_name: str):
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])
    _wait_until(
        f"resctrl group {group_name} should not exist",
        lambda: not _group_exists(group_name),
        timeout,
        interval,
    )


@then(parsers.parse('控制组 "{group_name}" 的 schemata 应包含关键项 "{k1}" 和 "{k2}"'))
def assert_schemata_contains(group_name: str, k1: str, k2: str):
    content = _read_group_schemata(group_name)
    if k1 not in content or k2 not in content:
        raise AssertionError(f"schemata missing expected keys, content={content!r}")


@then(parsers.parse('控制组 "{group_name}" 的 schemata 应与默认 QoSPolicy 配置一致'))
def assert_schemata_match_default_policy(group_name: str):
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])
    specs = test_context.get("policy_specs")
    policy_spec = None
    if isinstance(specs, dict):
        policy_spec = specs.get(group_name)
    if policy_spec is None:
        policy_spec = _default_policy_doc(group_name)["spec"]
    expected = _expected_schemata_values_from_policy_spec(policy_spec)

    _wait_until(
        f"schemata for {group_name} matches default policy values",
        lambda: _schemata_matches_expected(_read_group_schemata(group_name), expected),
        timeout,
        interval,
    )


@then(parsers.parse('控制组 "{group_name}" 的 tasks 应为非空'))
def assert_tasks_non_empty(group_name: str):
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])
    _wait_until(
        f"resctrl group {group_name} has tasks",
        lambda: _group_tasks_non_empty(group_name),
        timeout,
        interval,
    )


@when(parsers.parse('更新 QoSPolicy "{name}" 的 MB.MAX 为 {mb_max:d} 且 L3.WAYS 为 {l3_ways:d}'))
def update_policy(name: str, mb_max: int, l3_ways: int):
    test_context["schemata_before_update"] = _read_group_schemata(name)

    doc = _default_policy_doc(name)
    doc["spec"]["mb"]["max"] = mb_max
    doc["spec"]["l3"]["ways"] = l3_ways
    _apply_policy(doc)


@when(parsers.parse('更新 QoSPolicy "{name}" 的 NodeSelector 为不匹配当前节点'))
def update_policy_node_selector_mismatch(name: str):
    specs = test_context.get("policy_specs")
    current_spec = None
    if isinstance(specs, dict):
        current_spec = specs.get(name)
    if current_spec is None:
        current_spec = _default_policy_doc(name)["spec"]
    doc = {
        "apiVersion": "qos.kunpeng.huawei.com/v1alpha1",
        "kind": "QoSPolicy",
        "metadata": {"name": name},
        "spec": current_spec,
    }
    mismatch = test_context.get("node_selector_mismatch")
    if not isinstance(mismatch, dict) or not mismatch:
        key, value = _select_node_label_for_policy_selector()
        mismatch = _mismatch_selector_from_match(key, value)
        test_context["node_selector_mismatch"] = mismatch
    doc["spec"]["nodeSelector"] = mismatch
    _apply_policy(doc)


@then(parsers.parse('控制组 "{group_name}" 的 schemata 应匹配更新后的 MB.MAX={mb_max:d} 和 L3.WAYS={l3_ways:d}'))
def assert_schemata_match_updated_policy(group_name: str, mb_max: int, l3_ways: int):
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])
    expected_doc = _default_policy_doc(group_name)
    expected_doc["spec"]["mb"]["max"] = mb_max
    expected_doc["spec"]["l3"]["ways"] = l3_ways
    expected = _expected_schemata_values_from_policy_spec(expected_doc["spec"])

    _wait_until(
        f"schemata for {group_name} matches updated policy values",
        lambda: _schemata_matches_expected(_read_group_schemata(group_name), expected),
        timeout,
        interval,
    )


@then(parsers.parse('控制组 "{group_name}" 的 schemata 应发生变化'))
def assert_schemata_changed(group_name: str):
    before = str(test_context.get("schemata_before_update", ""))
    if not before:
        raise AssertionError("pre-update schemata snapshot is empty")

    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])

    def _changed() -> bool:
        return _read_group_schemata(group_name) != before

    _wait_until(f"schemata for {group_name} changed", _changed, timeout, interval)


@when(parsers.parse('删除 QoSPolicy "{name}"'))
def delete_policy(name: str):
    _run_kubectl(["delete", "qospolicy", name, "--ignore-not-found=true"], check=True)


@then(parsers.parse('resctrl 控制组 "{group_name}" 最终应被清理'))
def assert_group_removed(group_name: str):
    timeout = int(test_context["timeout"])
    interval = int(test_context["poll_interval"])

    def _removed() -> bool:
        return not _group_exists(group_name)

    _wait_until(f"resctrl group {group_name} removed", _removed, timeout, interval)


@when(parsers.parse('创建非法 QoSPolicy "{name}"'))
def create_invalid_policy(name: str):
    payload = yaml.safe_dump(_invalid_policy_doc(name), sort_keys=False)
    res = _run_kubectl(["apply", "-f", "-"], stdin=payload, check=False)
    test_context["last_apply_error"] = (res.stderr or "") + (res.stdout or "")


@then("创建应被 API 直接拒绝")
def assert_invalid_rejected():
    msg = str(test_context.get("last_apply_error", ""))
    if not msg:
        raise AssertionError("expected kubectl apply to fail but no error message captured")
    lowered = msg.lower()
    if "invalid" not in lowered and "denied" not in lowered:
        raise AssertionError(f"unexpected apply failure message: {msg}")


@then(parsers.parse('不应创建名为 "{group_name}" 的 resctrl 控制组'))
def assert_invalid_group_not_created(group_name: str):
    if _group_exists(group_name):
        raise AssertionError(f"unexpected resctrl group exists: {group_name}")


def collect_failure_diagnostics() -> None:
    """Print useful debugging info when a scenario fails."""
    _ensure_k8s_clients()
    print("\n========== MPAM E2E failure diagnostics ==========")

    op_ns = str(test_context["operator_namespace"])
    try:
        pods = k8s_core_v1.list_namespaced_pod(namespace=op_ns, label_selector=OPERATOR_LABEL_SELECTOR).items
        for pod in pods:
            print(f"\n---- controller logs: {op_ns}/{pod.metadata.name} ----")
            try:
                logs = k8s_core_v1.read_namespaced_pod_log(
                    name=pod.metadata.name,
                    namespace=op_ns,
                    tail_lines=200,
                    timestamps=True,
                )
                print(logs)
            except Exception as exc:  # pragma: no cover
                print(f"failed to read controller logs: {exc}")
    except Exception as exc:  # pragma: no cover
        print(f"failed to list controller pods: {exc}")

    group_name = str(test_context.get("current_group", "")).strip()
    if group_name:
        try:
            print(f"\n---- resctrl snapshot for group {group_name} ----")
            schemata = _exec_in_controller(
                f"if [ -f /sys/fs/resctrl/{group_name}/schemata ]; then cat /sys/fs/resctrl/{group_name}/schemata; else echo '<missing schemata>'; fi"
            )
            tasks = _exec_in_controller(
                f"if [ -f /sys/fs/resctrl/{group_name}/tasks ]; then cat /sys/fs/resctrl/{group_name}/tasks; else echo '<missing tasks>'; fi"
            )
            print("schemata:")
            print(schemata)
            print("tasks:")
            print(tasks)
        except Exception as exc:  # pragma: no cover
            print(f"failed to snapshot resctrl group {group_name}: {exc}")

    namespace = str(test_context["namespace"])
    pods = test_context.get("pods", set())
    if isinstance(pods, set):
        for pod_name in sorted(pods):
            print(f"\n---- describe pod {namespace}/{pod_name} ----")
            try:
                res = _run_kubectl(["-n", namespace, "describe", "pod", pod_name], check=False)
                print(res.stdout or res.stderr)
            except Exception as exc:  # pragma: no cover
                print(f"failed to describe pod {pod_name}: {exc}")

    policies = test_context.get("policies", set())
    if isinstance(policies, set):
        for name in sorted(policies):
            print(f"\n---- describe qospolicy {name} ----")
            try:
                res = _run_kubectl(["describe", "qospolicy", name], check=False)
                print(res.stdout or res.stderr)
            except Exception as exc:  # pragma: no cover
                print(f"failed to describe qospolicy {name}: {exc}")


def cleanup_test_resources() -> None:
    """Per-scenario hard cleanup for repeatability."""
    namespace = str(test_context["namespace"])

    pods = test_context.get("pods", set())
    if isinstance(pods, set):
        for pod_name in sorted(pods):
            _run_kubectl(["-n", namespace, "delete", "pod", pod_name, "--ignore-not-found=true"], check=False)

    policies = test_context.get("policies", set())
    if isinstance(policies, set):
        for name in sorted(policies):
            _run_kubectl(["delete", "qospolicy", name, "--ignore-not-found=true"], check=False)

    if test_context.get("operator_installed"):
        _run_kubectl(["delete", "-f", str(DAEMONSET_MANIFEST), "--ignore-not-found=true"], check=False)
        _run_kubectl(["delete", "-f", str(CRD_MANIFEST), "--ignore-not-found=true"], check=False)

    test_context["operator_installed"] = False
    test_context["target_node_name"] = ""
    test_context["controller_pod_name"] = ""
    test_context["cgroup_driver"] = ""
    test_context["current_group"] = ""
    test_context["schemata_before_update"] = ""
    test_context["last_apply_error"] = ""
    test_context["policies"] = set()
    test_context["pods"] = set()
    test_context["policy_specs"] = {}
    test_context["node_selector_match"] = {}
    test_context["node_selector_mismatch"] = {}
