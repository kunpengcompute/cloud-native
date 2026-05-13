# Fake Agent (for e2e debug)

## Purpose
This fake Python agent is used before the real interference analyzer is ready.

It only does two things:
- receive online pod cgroup info
- return a random interference reason (`l3` / `mb` / `cpu`)

## Run

```bash
python3 test/kunpeng-qos-controller/fake-agent/fake_agent.py --host 127.0.0.1 --port 18080
```

Optional:

```bash
python3 test/kunpeng-qos-controller/fake-agent/fake_agent.py --seed 123
```

## API

### POST `/v1/online-pods`
- body: current online pods payload from controller
- response:

```json
{"accepted": true, "message": "ok"}
```

### GET `/v1/interference?node_name=<node>`
- response example:

```json
{
  "version": "v1",
  "node_name": "node-a",
  "reason": "l3",
  "ttl_seconds": 10,
  "items": [
    {"pod_uid": "pod-uid-a", "score": 0.87}
  ]
}
```

