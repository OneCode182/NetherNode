# NetherNode Azure Scaffold

Repo-only extension scaffold. Do not deploy without a separate human approval
and cost review.

## Shape

| AWS MVP | Azure scaffold |
|---|---|
| EC2 instance | `azurerm_linux_virtual_machine` |
| Security group | `azurerm_network_security_group` |
| VPC/subnet | `azurerm_virtual_network` + `azurerm_subnet` |
| Public IPv4/DNS update | `azurerm_public_ip` output |
| EBS/root data path | VM OS disk + `/opt/nethernode/data/minecraft` |
| User data | `cloud-init.yaml` |
| Docker Compose runtime | same `compose.yaml`, `server/`, `ops/`, Go `nethernode` CLI |

## Variables

- `location`: Azure region, default `eastus`.
- `vm_size`: VM size, default `Standard_B2s`.
- `ssh_public_key`: replace placeholder before plan/apply.
- `repo_url`, `repo_branch`: repository checkout on the VM.
- `minecraft_java_port`: Java TCP, default `25565`.
- `minecraft_bedrock_port`: Bedrock UDP, default `19132`.
- `allowed_minecraft_cidrs`: restrict this before real use.
- `enable_ssh_ingress`: default `false`.

## Validate Only

```bash
terraform -chdir=infra/azure init -backend=false
terraform -chdir=infra/azure fmt -check
terraform -chdir=infra/azure validate
```

No `terraform apply` in the V2 repo-only workflow.

## Portability Rule

Keep cloud-specific work here. Runtime truth stays in:

- `server/`
- `compose.yaml`
- `ops/`
- `cmd/nethernode`
- `internal/`

World migration remains backup/restore of `/opt/nethernode/data/minecraft`.
