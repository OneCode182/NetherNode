output "instance_id" {
  description = "EC2 instance id."
  value       = aws_instance.app.id
}

output "instance_private_ip" {
  description = "EC2 private IPv4."
  value       = aws_instance.app.private_ip
}

output "instance_public_ip" {
  description = "EC2 public IPv4."
  value       = aws_instance.app.public_ip
}

output "security_group_id" {
  description = "App security group id."
  value       = aws_security_group.minecraft.id
}

output "budget_id" {
  description = "AWS budget resource id."
  value       = length(aws_budgets_budget.infra) > 0 ? aws_budgets_budget.infra[0].id : null
}

output "budget_sns_topic_arn" {
  description = "SNS topic for budget alerts."
  value       = length(aws_sns_topic.budget_alerts) > 0 ? aws_sns_topic.budget_alerts[0].arn : null
}
