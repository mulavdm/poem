[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param([Parameter(Mandatory = $true)][string]$CertificatePath)

$resolved = (Resolve-Path -LiteralPath $CertificatePath).Path
if ($PSCmdlet.ShouldProcess('Cert:\CurrentUser\TrustedPeople', "Trust $resolved for the current user")) {
    Import-Certificate -FilePath $resolved -CertStoreLocation Cert:\CurrentUser\TrustedPeople
}
