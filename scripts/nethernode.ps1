[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$CliArgs
)

$ErrorActionPreference = 'Stop'

function Write-Info { param([string]$Message) Write-Host $Message -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host $Message -ForegroundColor Green }
function Write-Warn { param([string]$Message) Write-Host $Message -ForegroundColor Yellow }
function Write-Fail { param([string]$Message) Write-Host $Message -ForegroundColor Red }
function Write-Title { param([string]$Message) Write-Host "`n$Message" -ForegroundColor Magenta }

function Read-DotEnv {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Config file not found: $Path"
    }

    $values = @{}
    $lineNumber = 0
    foreach ($line in [System.IO.File]::ReadAllLines($Path)) {
        $lineNumber++
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith('#')) { continue }

        $match = [regex]::Match($trimmed, '^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$')
        if (-not $match.Success) {
            throw "Invalid dotenv line $lineNumber in $Path"
        }

        $key = $match.Groups[1].Value
        $value = $match.Groups[2].Value.Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $values[$key] = $value
    }
    return $values
}

function Get-EnvFilePath {
    $override = [Environment]::GetEnvironmentVariable('NETHERNODE_CLOUD_ENV', 'Process')
    if ([string]::IsNullOrWhiteSpace($override)) {
        return (Join-Path $PSScriptRoot 'nethernode.local.env')
    }
    if ([System.IO.Path]::IsPathRooted($override)) { return $override }
    return (Join-Path (Get-Location).Path $override)
}

function Get-ConfigValue {
    param([string]$Name, [hashtable]$DotEnv)

    if (Test-Path -LiteralPath "Env:$Name") {
        return (Get-Item -LiteralPath "Env:$Name").Value
    }
    if ($DotEnv.ContainsKey($Name)) { return $DotEnv[$Name] }
    return $null
}

function Load-Config {
    $envFile = Get-EnvFilePath
    $dotEnv = Read-DotEnv -Path $envFile
    $required = @('AWS_REGION', 'EC2_INSTANCE_ID', 'SSH_USER', 'SSH_KEY_PATH', 'REMOTE_APP_DIR')
    $config = @{}
    foreach ($name in $required) {
        $value = Get-ConfigValue -Name $name -DotEnv $dotEnv
        if ([string]::IsNullOrWhiteSpace($value)) {
            throw "Missing required config: $name (process env overrides $envFile)"
        }
        $config[$name] = $value
    }
    $optional = @{
        AWS_PROFILE = $null
        MINECRAFT_STATUS_HOST = $null
        POLL_INTERVAL_SECONDS = '10'
        SSH_CONNECT_TIMEOUT_SECONDS = '10'
    }
    foreach ($name in $optional.Keys) {
        $value = Get-ConfigValue -Name $name -DotEnv $dotEnv
        $config[$name] = if ([string]::IsNullOrWhiteSpace($value)) { $optional[$name] } else { $value }
    }
    foreach ($name in @('POLL_INTERVAL_SECONDS', 'SSH_CONNECT_TIMEOUT_SECONDS')) {
        $parsed = 0
        if (-not [int]::TryParse($config[$name], [ref]$parsed) -or $parsed -lt 1) {
            throw "$name must be a positive integer."
        }
        $config[$name] = $parsed
    }
    if (-not (Test-Path -LiteralPath $config.SSH_KEY_PATH -PathType Leaf)) {
        throw "SSH_KEY_PATH not found: $($config.SSH_KEY_PATH)"
    }
    Write-Info "Config: $envFile"
    return $config
}

function Assert-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found: $Name"
    }
}

function Invoke-Aws {
    param([hashtable]$Config, [string[]]$Arguments)

    $awsArguments = @($Arguments)
    if (-not [string]::IsNullOrWhiteSpace($Config.AWS_PROFILE)) {
        $awsArguments = @('--profile', $Config.AWS_PROFILE) + $awsArguments
    }
    $output = & aws @awsArguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw "AWS CLI failed: $($output -join [Environment]::NewLine)" }
    return @($output | ForEach-Object { $_.ToString() })
}

function Get-Ec2Info {
    param([hashtable]$Config)

    $output = Invoke-Aws -Config $Config -Arguments @(
        'ec2', 'describe-instances', '--region', $Config.AWS_REGION,
        '--instance-ids', $Config.EC2_INSTANCE_ID,
        '--query', 'Reservations[0].Instances[0].[State.Name,PublicIpAddress,PrivateIpAddress]',
        '--output', 'text'
    )
    $parts = (($output -join ' ') -split '\s+')
    if ($parts.Count -lt 1 -or [string]::IsNullOrWhiteSpace($parts[0])) { throw 'AWS returned no EC2 instance state.' }
    $publicIp = if ($parts.Count -gt 1 -and $parts[1] -ne 'None') { $parts[1] } else { $null }
    $privateIp = if ($parts.Count -gt 2 -and $parts[2] -ne 'None') { $parts[2] } else { $null }
    return [pscustomobject]@{ State = $parts[0]; PublicIp = $publicIp; PrivateIp = $privateIp }
}

function Get-SshHost {
    param([hashtable]$Config)
    $info = Get-Ec2Info -Config $Config
    if ($info.State -ne 'running') { throw "EC2 is $($info.State); SSH needs running instance." }
    if ([string]::IsNullOrWhiteSpace($info.PublicIp)) { throw 'EC2 has no public IP for SSH.' }
    return $info.PublicIp
}

function ConvertTo-ShellLiteral {
    param([string]$Value)
    $quote = [string][char]39
    return $quote + $Value.Replace($quote, $quote + '"' + $quote + '"' + $quote) + $quote
}

function Wait-Ssh {
    param([hashtable]$Config, [int]$MaxAttempts = 30)

    $hostName = Get-SshHost -Config $Config
    $target = "$($Config.SSH_USER)@$hostName"
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        $null = & ssh '-i' $Config.SSH_KEY_PATH '-o' 'BatchMode=yes' '-o' 'StrictHostKeyChecking=accept-new' '-o' "ConnectTimeout=$($Config.SSH_CONNECT_TIMEOUT_SECONDS)" $target 'true' 2>&1
        if ($LASTEXITCODE -eq 0) { return $hostName }
        if ($attempt -lt $MaxAttempts) {
            Write-Warn "SSH unavailable ($attempt/$MaxAttempts); retrying in $($Config.POLL_INTERVAL_SECONDS)s..."
            Start-Sleep -Seconds $Config.POLL_INTERVAL_SECONDS
        }
    }
    throw "SSH unavailable after $MaxAttempts attempts: $target"
}

function Invoke-Remote {
    param([hashtable]$Config, [string]$Command, [switch]$Quiet, [switch]$SkipWait)

    $hostName = if ($SkipWait) { Get-SshHost -Config $Config } else { Wait-Ssh -Config $Config }
    $sshArgs = @('-i', $Config.SSH_KEY_PATH, '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=accept-new', '-o', "ConnectTimeout=$($Config.SSH_CONNECT_TIMEOUT_SECONDS)", "$($Config.SSH_USER)@$hostName", $Command)
    $output = & ssh @sshArgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "SSH failed: $($output -join [Environment]::NewLine)" }
    if (-not $Quiet) { $output | ForEach-Object { Write-Host $_ } }
    return @($output | ForEach-Object { $_.ToString() })
}

function Invoke-RemoteScript {
    param([hashtable]$Config, [string]$Script, [string[]]$Arguments = @())
    $dir = ConvertTo-ShellLiteral $Config.REMOTE_APP_DIR
    $argText = ($Arguments | ForEach-Object { ConvertTo-ShellLiteral $_ }) -join ' '
    $command = "cd $dir && sudo -n bash $Script"
    if ($argText.Length -gt 0) { $command += " $argText" }
    Invoke-Remote -Config $Config -Command $command | Out-Null
}

function Invoke-RemoteNetherNode {
    param([hashtable]$Config, [string[]]$Arguments, [switch]$Quiet, [switch]$SkipWait)
    $argText = ($Arguments | ForEach-Object { ConvertTo-ShellLiteral $_ }) -join ' '
    $command = 'sudo -n nethernode'
    if ($argText.Length -gt 0) { $command += " $argText" }
    return Invoke-Remote -Config $Config -Command $command -Quiet:$Quiet -SkipWait:$SkipWait
}

function Test-RemoteContainerRunning {
    param([hashtable]$Config)
    $dir = ConvertTo-ShellLiteral $Config.REMOTE_APP_DIR
    $output = Invoke-Remote -Config $Config -Command "cd $dir && sudo -n docker compose ps --status running -q" -Quiet
    return ($output | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }).Count -gt 0
}

function Start-Ec2 {
    param([hashtable]$Config)
    $info = Get-Ec2Info -Config $Config
    if ($info.State -ne 'running') {
        Write-Info 'Starting EC2...'
        Invoke-Aws -Config $Config -Arguments @('ec2', 'start-instances', '--region', $Config.AWS_REGION, '--instance-ids', $Config.EC2_INSTANCE_ID) | Out-Null
    } else { Write-Info 'EC2 already running; checking instance status...' }
    Invoke-Aws -Config $Config -Arguments @('ec2', 'wait', 'instance-running', '--region', $Config.AWS_REGION, '--instance-ids', $Config.EC2_INSTANCE_ID) | Out-Null
    Invoke-Aws -Config $Config -Arguments @('ec2', 'wait', 'instance-status-ok', '--region', $Config.AWS_REGION, '--instance-ids', $Config.EC2_INSTANCE_ID) | Out-Null
    Write-Ok 'EC2 running; instance status OK.'
}

function Stop-Ec2 {
    param([hashtable]$Config)
    $info = Get-Ec2Info -Config $Config
    if ($info.State -eq 'stopped') { Write-Ok 'EC2 already stopped.'; return }
    if ($info.State -ne 'running') { throw "EC2 is $($info.State); refusing stop." }
    Write-Info 'Stopping EC2...'
    Invoke-Aws -Config $Config -Arguments @('ec2', 'stop-instances', '--region', $Config.AWS_REGION, '--instance-ids', $Config.EC2_INSTANCE_ID) | Out-Null
    Invoke-Aws -Config $Config -Arguments @('ec2', 'wait', 'instance-stopped', '--region', $Config.AWS_REGION, '--instance-ids', $Config.EC2_INSTANCE_ID) | Out-Null
    Write-Ok 'EC2 stopped.'
}

function Show-Status {
    param([hashtable]$Config, [switch]$Once)
    do {
        Clear-Host
        Write-Title 'NetherNode status'
        try {
            $info = Get-Ec2Info -Config $Config
            Write-Host "EC2 state : $($info.State)" -ForegroundColor $(if ($info.State -eq 'running') { 'Green' } else { 'Yellow' })
            Write-Host "Public IP : $($info.PublicIp)"
            Write-Host "Private IP: $($info.PrivateIp)"
            if ($info.State -eq 'running') {
                try {
                    $statusHost = if ([string]::IsNullOrWhiteSpace($Config.MINECRAFT_STATUS_HOST)) { $info.PublicIp } else { $Config.MINECRAFT_STATUS_HOST }
                    $remoteStatus = Invoke-RemoteNetherNode -Config $Config -Arguments @('status', '--host', $statusHost, '--color=never') -Quiet -SkipWait
                    Write-Host "`nRemote status:" -ForegroundColor Magenta
                    $remoteStatus | ForEach-Object { Write-Host $_ }
                } catch { Write-Warn "Remote status unavailable: $($_.Exception.Message)" }
            } else { Write-Host 'Remote status: unavailable' -ForegroundColor DarkGray }
        } catch { Write-Fail $_.Exception.Message }
        if (-not $Once) {
            Write-Host "`nRefreshing every $($Config.POLL_INTERVAL_SECONDS)s. Ctrl+C exits." -ForegroundColor DarkGray
            Start-Sleep -Seconds $Config.POLL_INTERVAL_SECONDS
        }
    } while (-not $Once)
}

function Show-Help {
    @'
NetherNode local controller

  nethernode.ps1 help
  nethernode.ps1 status [--once]
  nethernode.ps1 start [--only-ec2 | --only-server] [--no-watch]
  nethernode.ps1 stop [--only-ec2 | --only-server] [--no-watch]
  nethernode.ps1 restart [--no-watch]
  nethernode.ps1 save
  nethernode.ps1 backup

Config: scripts/nethernode.local.env, or NETHERNODE_CLOUD_ENV.
Process environment values override dotenv values.
'@ | Write-Host
}

function Assert-Flags {
    param([string[]]$Flags, [string[]]$Allowed)
    foreach ($flag in $Flags) { if ($Allowed -notcontains $flag) { throw "Unknown flag: $flag" } }
    if (($Flags -contains '--only-ec2') -and ($Flags -contains '--only-server')) { throw 'Use only one: --only-ec2 or --only-server.' }
}

function Invoke-Start {
    param([hashtable]$Config, [string[]]$Flags)
    Assert-Flags $Flags @('--only-ec2', '--only-server', '--no-watch')
    $onlyEc2 = $Flags -contains '--only-ec2'; $onlyServer = $Flags -contains '--only-server'
    if (-not $onlyServer) { Start-Ec2 -Config $Config }
    if (-not $onlyEc2) { Write-Info 'Starting server...'; Invoke-RemoteScript -Config $Config -Script 'ops/start.sh'; Write-Ok 'Server started.' }
    if ($Flags -notcontains '--no-watch') { Show-Status -Config $Config }
}

function Invoke-Stop {
    param([hashtable]$Config, [string[]]$Flags)
    Assert-Flags $Flags @('--only-ec2', '--only-server', '--no-watch')
    $onlyEc2 = $Flags -contains '--only-ec2'; $onlyServer = $Flags -contains '--only-server'
    $noWatch = $Flags -contains '--no-watch'
    if ($onlyEc2) {
        $info = Get-Ec2Info -Config $Config
        if ($info.State -eq 'stopped') {
            Write-Ok 'EC2 already stopped.'
            if (-not $noWatch) { Show-Status -Config $Config }
            return
        }
        if ($info.State -ne 'running') { throw "EC2 is $($info.State); refusing stop." }
        if (Test-RemoteContainerRunning -Config $Config) { throw 'Refusing --only-ec2: server container is running. Stop server first.' }
        Stop-Ec2 -Config $Config
        if (-not $noWatch) { Show-Status -Config $Config }
        return
    }
    Write-Info 'Backing up server...'; Invoke-RemoteNetherNode -Config $Config -Arguments @('backup-server') | Out-Null; Write-Ok 'Backup complete.'
    Write-Info 'Stopping server...'; Invoke-RemoteNetherNode -Config $Config -Arguments @('stop', '--no-backup') | Out-Null; Write-Ok 'Server stopped.'
    if (-not $onlyServer) { Stop-Ec2 -Config $Config }
    if (-not $noWatch) { Show-Status -Config $Config }
}

function Invoke-Restart {
    param([hashtable]$Config, [string[]]$Flags)
    Assert-Flags $Flags @('--no-watch')
    $info = Get-Ec2Info -Config $Config
    if ($info.State -eq 'stopped') {
        Invoke-Start -Config $Config -Flags $Flags
        return
    }
    Start-Ec2 -Config $Config
    Write-Info 'Backing up server...'; Invoke-RemoteNetherNode -Config $Config -Arguments @('backup-server') | Out-Null; Write-Ok 'Backup complete.'
    Write-Info 'Restarting server...'; Invoke-RemoteNetherNode -Config $Config -Arguments @('restart', '--no-backup') | Out-Null; Write-Ok 'Server restarted.'
    if ($Flags -notcontains '--no-watch') { Show-Status -Config $Config }
}

function Invoke-Menu {
    param([hashtable]$Config)
    while ($true) {
        Write-Title 'NetherNode'
        Write-Host '[1] Status  [2] Start  [3] Stop  [4] Restart  [5] Save  [6] Backup  [H] Help  [Q] Quit'
        $choice = (Read-Host 'Choose').Trim().ToLowerInvariant()
        try {
            switch ($choice) {
                '1' { Show-Status -Config $Config }
                '2' { Invoke-Start -Config $Config -Flags @('--no-watch') }
                '3' { Invoke-Stop -Config $Config -Flags @('--no-watch') }
                '4' { Invoke-Restart -Config $Config -Flags @('--no-watch') }
                '5' { Invoke-RemoteNetherNode -Config $Config -Arguments @('save-server') | Out-Null; Write-Ok 'Save complete.' }
                '6' { Invoke-RemoteNetherNode -Config $Config -Arguments @('backup-server') | Out-Null; Write-Ok 'Backup complete.' }
                'h' { Show-Help }
                'q' { return }
                default { Write-Warn 'Choose 1-6, H, or Q.' }
            }
        } catch { Write-Fail $_.Exception.Message }
    }
}

function Main {
    param([string[]]$Arguments)

    if ($Arguments.Count -gt 0) {
        $command = $Arguments[0].ToLowerInvariant()
        $flags = @($Arguments | Select-Object -Skip 1)
        if ($command -in @('help', '-h', '--help')) {
            if ($flags.Count -gt 0) { throw 'help accepts no flags.' }
            Show-Help
            return
        }
    }

    Assert-Command aws; Assert-Command ssh
    $config = Load-Config
    if ($Arguments.Count -eq 0) { Invoke-Menu -Config $config; return }
    switch ($command) {
        'status' { Assert-Flags $flags @('--once'); Show-Status -Config $config -Once:($flags -contains '--once') }
        'start' { Invoke-Start -Config $config -Flags $flags }
        'stop' { Invoke-Stop -Config $config -Flags $flags }
        'restart' { Invoke-Restart -Config $config -Flags $flags }
        'save' { if ($flags.Count -gt 0) { throw 'save accepts no flags.' }; Invoke-RemoteNetherNode -Config $config -Arguments @('save-server') | Out-Null; Write-Ok 'Save complete.' }
        'backup' { if ($flags.Count -gt 0) { throw 'backup accepts no flags.' }; Invoke-RemoteNetherNode -Config $config -Arguments @('backup-server') | Out-Null; Write-Ok 'Backup complete.' }
        default { throw "Unknown command: $command. Run help." }
    }
}

try { Main -Arguments $CliArgs } catch { Write-Fail $_.Exception.Message; exit 1 }
