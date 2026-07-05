variable "project_name" {
  description = "Project name used for resource naming and tags."
  type        = string
  default     = "nethernode"
}

variable "environment" {
  description = "Deployment environment."
  type        = string
  default     = "dev"
}

variable "aws_region" {
  description = "AWS region for resources."
  type        = string
  default     = "us-east-1"
}

variable "project_tags" {
  description = "Extra tags applied to all tagged resources."
  type        = map(string)
  default     = {}
}

variable "vpc_id" {
  description = "Optional VPC ID. Empty = auto-use default VPC."
  type        = string
  default     = ""
}

variable "subnet_id" {
  description = "Optional subnet ID. Empty = first default subnet."
  type        = string
  default     = ""
}

variable "instance_type" {
  description = "EC2 instance size."
  type        = string
  default     = "t4g.small"
}

variable "ami_id" {
  description = "Optional AMI override. Empty = discover latest Amazon Linux 2023 AMI."
  type        = string
  default     = ""
}

variable "associate_public_ip_address" {
  description = "Enable public IPv4 on the instance."
  type        = bool
  default     = true
}

variable "minecraft_ingress_cidrs" {
  description = "CIDR list for Minecraft Java and Bedrock ingress."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "minecraft_java_port" {
  description = "Minecraft Java TCP port."
  type        = number
  default     = 25565
}

variable "minecraft_bedrock_port" {
  description = "Geyser Bedrock UDP port."
  type        = number
  default     = 19132
}

variable "root_volume_size_gib" {
  description = "Root EBS size in GiB."
  type        = number
  default     = 20
}

variable "root_volume_iops" {
  description = "Root gp3 IOPS."
  type        = number
  default     = 3000
}

variable "root_volume_throughput" {
  description = "Root gp3 throughput in MB/s."
  type        = number
  default     = 125
}

variable "app_repo_url" {
  description = "Git URL for Docker compose repository."
  type        = string
  default     = ""
}

variable "app_repo_branch" {
  description = "Repository branch to deploy."
  type        = string
  default     = "main"
}

variable "app_repo_clone_path" {
  description = "Instance path to clone repo into."
  type        = string
  default     = "/opt/nethernode/app"
}

variable "compose_relative_path" {
  description = "Compose file path inside repo."
  type        = string
  default     = "compose.yaml"
}

variable "minecraft_eula_accepted" {
  description = "Set true only after accepting the Minecraft EULA."
  type        = bool
  default     = false
}

variable "start_server_on_boot" {
  description = "Start Minecraft during EC2 bootstrap. Keep false for low-cost manual start."
  type        = bool
  default     = false
}

variable "github_repository" {
  description = "GitHub repository allowed to assume the deploy role, in owner/repo format. Empty disables GitHub OIDC resources."
  type        = string
  default     = ""
}

variable "github_branch" {
  description = "GitHub branch allowed to assume the deploy role."
  type        = string
  default     = "main"
}

variable "github_oidc_provider_arn" {
  description = "Existing GitHub OIDC provider ARN. Empty creates one when github_repository is set."
  type        = string
  default     = ""
}

variable "create_github_oidc_provider" {
  description = "Create the account-level GitHub OIDC provider when github_repository is set and no provider ARN is supplied."
  type        = bool
  default     = true
}

variable "budget_enabled" {
  description = "Create AWS budget."
  type        = bool
  default     = true
}

variable "monthly_budget_limit_usd" {
  description = "Monthly budget cap in USD."
  type        = number
  default     = 8.33
}

variable "budget_time_unit" {
  description = "Budget time unit (e.g. MONTHLY)."
  type        = string
  default     = "MONTHLY"
}

variable "budget_alarms" {
  description = "Budget thresholds for alerts."
  type = list(object({
    comparison_operator = string
    threshold           = number
    threshold_type      = string
    notification_type   = string
  }))
  default = [
    {
      comparison_operator = "GREATER_THAN"
      threshold           = 80
      threshold_type      = "PERCENTAGE"
      notification_type   = "ACTUAL"
    },
    {
      comparison_operator = "GREATER_THAN"
      threshold           = 100
      threshold_type      = "PERCENTAGE"
      notification_type   = "FORECASTED"
    }
  ]
}

variable "budget_notification_arns" {
  description = "Existing SNS topic ARNs for budget notifications."
  type        = list(string)
  default     = []
}

variable "budget_notification_emails" {
  description = "Email addresses for budget SNS topic subscriptions."
  type        = list(string)
  default     = []
}
