# Update the repository description and topics from the checked-in metadata.
# Run from an authenticated GitHub CLI session with repository admin access.
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repository = 'tigusigalpa/fastnear-go'
$metadataPath = Join-Path $PSScriptRoot '../.github/repository-metadata.json'
$metadata = Get-Content -LiteralPath $metadataPath -Raw | ConvertFrom-Json

if ($metadata.description.Length -lt 320 -or $metadata.description.Length -gt 340) {
    throw 'The repository description must contain 320 to 340 characters.'
}

$ghArguments = @('repo', 'edit', $repository, '--description', $metadata.description)
foreach ($topic in $metadata.topics) {
    $ghArguments += @('--add-topic', $topic)
}

& gh @ghArguments
if ($LASTEXITCODE -ne 0) {
    throw 'GitHub repository metadata update failed.'
}

& gh repo view $repository --json description,repositoryTopics,url
if ($LASTEXITCODE -ne 0) {
    throw 'Could not verify the updated repository metadata.'
}
