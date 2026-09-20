# Registry Setup Instructions

This manifest must be applied manually:
```bash
kubectl apply -f manifests/registry.yaml
```

After applying, the K3s host needs `/etc/rancher/k3s/registries.yaml` configured with:

```yaml
mirrors:
  "localhost:30500":
    endpoint:
      - "http://localhost:30500"
```

Then restart K3s:
```bash
sudo systemctl restart k3s
```

The firewall should restrict port 30500 to localhost only:
```bash
sudo ufw allow from 127.0.0.1 to any port 30500
```
