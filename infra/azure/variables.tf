variable "name_prefix" {
  description = "Resource name prefix."
  type        = string
  default     = "nethernode"
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "eastus"
}

variable "vm_size" {
  description = "Low-cost VM size for the Minecraft host."
  type        = string
  default     = "Standard_B2s"
}

variable "admin_username" {
  description = "Admin username for the Linux VM."
  type        = string
  default     = "nethernode"
}

variable "ssh_public_key" {
  description = "Public SSH key material. Replace placeholder before plan/apply."
  type        = string
  default     = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINetherNodePlaceholderDoNotApplyUntilReplaced nethernode-placeholder"
}

variable "repo_url" {
  description = "Git repository URL cloned on the VM."
  type        = string
  default     = "https://github.com/<owner>/NetherNode.git"
}

variable "repo_branch" {
  description = "Branch checked out on the VM."
  type        = string
  default     = "master"
}

variable "minecraft_java_port" {
  description = "Minecraft Java TCP port."
  type        = number
  default     = 25565
}

variable "minecraft_bedrock_port" {
  description = "Minecraft Bedrock/Geyser UDP port."
  type        = number
  default     = 19132
}

variable "os_disk_size_gb" {
  description = "OS disk size. Keep small for low-cost MVP."
  type        = number
  default     = 32
}

variable "allowed_minecraft_cidrs" {
  description = "CIDRs allowed to connect to Minecraft ports."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "enable_ssh_ingress" {
  description = "Open SSH 22/tcp to allowed_ssh_cidrs. Keep false unless needed."
  type        = bool
  default     = false
}

variable "allowed_ssh_cidrs" {
  description = "CIDRs allowed for SSH when enable_ssh_ingress=true."
  type        = list(string)
  default     = []
}
