# Usage:
## Pre-work:
1. Add necessary labels for all compute nodes in master node:
here we assume there is only one compute node with name "compute01"
```sh
kubectl label nodes compute01 node-role.kubernetes.io/worker=""
kubectl label node compute01 feature.node.kubernetes.io/network-sriov.capable=true
kubectl label node compute01 node-virtualization.kubernetes.io/type=qemu-kvm
```
2. (Optional) remove files left in last deployment: 
• in all compute nodes: `rm /tmp/sno-initial-node-state.json`;

• remove blacklist: `kubectl delete -f /path/to/blacklist.yaml`


## Deploy
1. Run `make deploy-setup-k8s` to deploy sriov-network-operator. 
When done, 
• `kubectl get pods -A` will show two new pods: `sriov-network-operato` and `sriov-network-config-daemon`. 

• `kubectl get SriovNetworkNodeState -n sriov-network-operator <compute_node_name> -o yaml` will show the content of SriovNetworkNodeState

• there will be the file(s)-`/tmp/sno-initial-node-state.json` in compute node(s).

2. (Optional) deploy blacklist.yaml.
3. Deploy SriovNetworkNodePolicy (eg: node-policy-1823.yaml). After this, `sriov-device-plugin` will be deployed in compute node(s).
4. Deploy SriovNetwork (eg: sriov-network.yaml/host-device.yml).
5. Deploy the pod using the vfs (eg: testpod1.yaml).