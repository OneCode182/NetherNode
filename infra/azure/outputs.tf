output "resource_group_name" {
  description = "Azure resource group name."
  value       = azurerm_resource_group.this.name
}

output "vm_name" {
  description = "Azure VM name."
  value       = azurerm_linux_virtual_machine.this.name
}

output "public_ip_address" {
  description = "Public IP address for Minecraft clients."
  value       = azurerm_public_ip.this.ip_address
}

output "minecraft_java_endpoint" {
  description = "Java client endpoint."
  value       = "${azurerm_public_ip.this.ip_address}:${var.minecraft_java_port}"
}

output "minecraft_bedrock_endpoint" {
  description = "Bedrock/Geyser endpoint."
  value       = "${azurerm_public_ip.this.ip_address}:${var.minecraft_bedrock_port}"
}
