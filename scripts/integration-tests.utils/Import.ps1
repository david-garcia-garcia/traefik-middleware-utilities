# Dot-source every sibling *.ps1 in this folder. Called from each domain *Tests.ps1.
Get-ChildItem $PSScriptRoot -Filter *.ps1 |
    Where-Object { $_.Name -ne "Import.ps1" } |
    Sort-Object Name |
    ForEach-Object { . $_.FullName }
