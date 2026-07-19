[CmdletBinding()]
param(
    [string]$Subject = 'CN=POEM Development',
    [string]$FriendlyName = 'POEM Development MSIX Signing',
    [string]$OutputPath = (Join-Path $PSScriptRoot 'poem-development.cer')
)

$ErrorActionPreference = 'Stop'
$existing = Get-ChildItem Cert:\CurrentUser\My | Where-Object { $_.Subject -eq $Subject -and $_.FriendlyName -eq $FriendlyName -and $_.NotAfter -gt (Get-Date).AddDays(30) } | Select-Object -First 1
if (-not $existing) {
    $existing = New-SelfSignedCertificate -Type Custom -Subject $Subject -FriendlyName $FriendlyName -KeyUsage DigitalSignature -KeyAlgorithm RSA -KeyLength 3072 -HashAlgorithm SHA256 -CertStoreLocation Cert:\CurrentUser\My -TextExtension @('2.5.29.37={text}1.3.6.1.5.5.7.3.3')
}
Export-Certificate -Cert $existing -FilePath $OutputPath -Force | Out-Null
[pscustomobject]@{ Thumbprint = $existing.Thumbprint; Subject = $existing.Subject; Certificate = (Resolve-Path -LiteralPath $OutputPath).Path }
