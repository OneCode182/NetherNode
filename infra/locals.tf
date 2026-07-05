locals {
  project_tags = merge(
    {
      "Project"     = var.project_name
      "Environment" = var.environment
      "ManagedBy"   = "terraform"
    },
    var.project_tags,
  )

  effective_vpc_id    = var.vpc_id != "" ? var.vpc_id : data.aws_vpc.default.id
  effective_subnet_id = var.subnet_id != "" ? var.subnet_id : data.aws_subnets.default.ids[0]
  effective_ami_id    = var.ami_id != "" ? var.ami_id : data.aws_ami.al2023.id

  budget_alert_topic_arns = length(aws_sns_topic.budget_alerts) > 0 ? aws_sns_topic.budget_alerts[*].arn : []
  budget_subscriber_arns  = distinct(concat(var.budget_notification_arns, local.budget_alert_topic_arns))
}
