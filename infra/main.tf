data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }

  filter {
    name   = "default-for-az"
    values = ["true"]
  }
}

data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*"]
  }

  filter {
    name   = "architecture"
    values = [var.ami_architecture]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }
}

data "aws_caller_identity" "current" {}

resource "aws_security_group" "minecraft" {
  name        = "${var.project_name}-${var.environment}-app"
  description = "NetherNode app network boundary."
  vpc_id      = local.effective_vpc_id

  ingress {
    description = "Minecraft Java"
    from_port   = var.minecraft_java_port
    to_port     = var.minecraft_java_port
    protocol    = "tcp"
    cidr_blocks = var.minecraft_ingress_cidrs
  }

  ingress {
    description = "Geyser Bedrock UDP"
    from_port   = var.minecraft_bedrock_port
    to_port     = var.minecraft_bedrock_port
    protocol    = "udp"
    cidr_blocks = var.minecraft_ingress_cidrs
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }
}

resource "aws_iam_role" "ssm" {
  name = "${var.project_name}-${var.environment}-ssm"

  assume_role_policy = data.aws_iam_policy_document.ec2_assume_role.json
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.ssm.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "ssm" {
  name = "${var.project_name}-${var.environment}-ssm"
  role = aws_iam_role.ssm.name
}

resource "aws_key_pair" "app" {
  count      = var.ssh_public_key != "" ? 1 : 0
  key_name   = "${var.project_name}-${var.environment}-key"
  public_key = var.ssh_public_key
}

resource "aws_instance" "app" {
  ami                    = local.effective_ami_id
  instance_type          = var.instance_type
  subnet_id              = local.effective_subnet_id
  vpc_security_group_ids = [aws_security_group.minecraft.id]
  iam_instance_profile   = aws_iam_instance_profile.ssm.name
  key_name               = var.ssh_public_key != "" ? aws_key_pair.app[0].key_name : null

  associate_public_ip_address = var.associate_public_ip_address

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.root_volume_size_gib
    iops                  = var.root_volume_iops
    throughput            = var.root_volume_throughput
    delete_on_termination = true
    encrypted             = true
  }

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 2
    instance_metadata_tags      = "enabled"
  }

  user_data = templatefile("${path.module}/user-data.tftpl", {
    app_repo_url          = var.app_repo_url
    app_repo_branch       = var.app_repo_branch
    app_repo_clone_path   = var.app_repo_clone_path
    compose_relative_path = var.compose_relative_path
    minecraft_eula        = var.minecraft_eula_accepted ? "TRUE" : "FALSE"
    start_server_on_boot  = var.start_server_on_boot ? "true" : "false"
  })

  tags = merge(local.project_tags, {
    Name = "${var.project_name}-${var.environment}-minecraft"
  })
}

resource "aws_iam_openid_connect_provider" "github" {
  count = local.github_oidc_enabled && var.create_github_oidc_provider && var.github_oidc_provider_arn == "" ? 1 : 0

  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1", "1c58a3a8518e8759bf075b76b750d4f2df264fcd"]
}

resource "aws_iam_role" "github_actions" {
  count = local.github_oidc_enabled ? 1 : 0

  name               = "${var.project_name}-${var.environment}-github-actions"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume[0].json
}

resource "aws_iam_role_policy" "github_actions" {
  count = local.github_oidc_enabled ? 1 : 0

  name   = "${var.project_name}-${var.environment}-server-control"
  role   = aws_iam_role.github_actions[0].id
  policy = data.aws_iam_policy_document.github_actions_control[0].json
}

resource "aws_sns_topic" "budget_alerts" {
  count = var.budget_enabled ? 1 : 0
  name  = "${var.project_name}-${var.environment}-budget-alerts"
}

resource "aws_sns_topic_subscription" "budget_email" {
  count     = length(aws_sns_topic.budget_alerts) > 0 ? length(var.budget_notification_emails) : 0
  topic_arn = aws_sns_topic.budget_alerts[0].arn
  protocol  = "email"
  endpoint  = var.budget_notification_emails[count.index]
}

resource "aws_budgets_budget" "infra" {
  count             = var.budget_enabled ? 1 : 0
  name              = "${var.project_name}-${var.environment}-monthly-limit"
  budget_type       = "COST"
  limit_amount      = tostring(var.monthly_budget_limit_usd)
  limit_unit        = "USD"
  time_unit         = var.budget_time_unit
  time_period_start = "2026-01-01_00:00"

  dynamic "notification" {
    for_each = length(local.budget_subscriber_arns) > 0 ? var.budget_alarms : []
    content {
      comparison_operator       = notification.value.comparison_operator
      threshold                 = notification.value.threshold
      threshold_type            = notification.value.threshold_type
      notification_type         = notification.value.notification_type
      subscriber_sns_topic_arns = local.budget_subscriber_arns
    }
  }
}

data "aws_iam_policy_document" "ec2_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "github_actions_assume" {
  count = local.github_oidc_enabled ? 1 : 0

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        "repo:${var.github_repository}:ref:refs/heads/${var.github_branch}",
      ]
    }
  }
}

data "aws_iam_policy_document" "github_actions_control" {
  count = local.github_oidc_enabled ? 1 : 0

  statement {
    actions = [
      "ec2:StartInstances",
      "ec2:StopInstances",
    ]
    resources = [aws_instance.app.arn]
  }

  statement {
    actions = [
      "ec2:DescribeInstanceStatus",
      "ec2:DescribeInstances",
      "ssm:DescribeInstanceInformation",
      "ssm:GetCommandInvocation",
      "ssm:ListCommandInvocations",
    ]
    resources = ["*"]
  }

  statement {
    actions = ["ssm:SendCommand"]
    resources = [
      aws_instance.app.arn,
      "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:document/AWS-RunShellScript",
    ]
  }
}
