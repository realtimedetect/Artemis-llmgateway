#!/usr/bin/env pwsh
<#
.SYNOPSIS
    LLM Gateway Documentation PDF Converter
    
.DESCRIPTION
    Converts HTML documentation to professional PDFs using Headless Chrome.
    Supports multiple variations and batch processing.
    
.PARAMETER Source
    Source HTML file (default: LLM_Gateway_User_Manual.html)
    
.PARAMETER Output
    Output PDF filename (default: LLM_Gateway_User_Manual.pdf)
    
.PARAMETER AllDocuments
    Generate PDFs for all documents
    
.PARAMETER Version
    Add version to filenames
    
.EXAMPLE
    .\convert-to-pdf.ps1
    .\convert-to-pdf.ps1 -AllDocuments -Version "1.0.0"
    .\convert-to-pdf.ps1 -Source "QUICK_REFERENCE_GUIDE.md"
#>

param(
    [string]$Source = "LLM_Gateway_User_Manual.html",
    [string]$Output = "",
    [switch]$AllDocuments = $false,
    [string]$Version = "",
    [switch]$Verbose = $false
)

# Configuration
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $scriptDir) { $scriptDir = Get-Location }

$chromePaths = @(
    "C:\Program Files\Google\Chrome\Application\chrome.exe",
    "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe",
    "C:\Program Files\Chromium\Application\chrome.exe",
    "C:\Program Files (x86)\Chromium\Application\chrome.exe"
)

# Find Chrome executable
$chromeExePath = $null
foreach ($path in $chromePaths) {
    if (Test-Path $path) {
        $chromeExePath = $path
        break
    }
}

if (-not $chromeExePath) {
    Write-Error "Chrome/Chromium not found. Please install Google Chrome first."
    exit 1
}

function Convert-HtmlToPdf {
    param(
        [string]$InputFile,
        [string]$OutputFile
    )
    
    # Get absolute paths
    $inputPath = Join-Path $scriptDir $InputFile
    $outputPath = Join-Path $scriptDir $OutputFile
    
    if (-not (Test-Path $inputPath)) {
        Write-Error "Input file not found: $inputPath"
        return $false
    }
    
    Write-Host "Converting: $InputFile → $OutputFile" -ForegroundColor Cyan
    
    # Convert path for file:// URL
    $fileUrl = "file:///$($inputPath.Replace('\', '/'))"
    
    try {
        # Run Chrome in headless mode
        & $chromeExePath `
            --headless=new `
            --disable-gpu `
            --print-to-pdf=$outputPath `
            --print-to-pdf-margin-top=0.5 `
            --print-to-pdf-margin-bottom=0.5 `
            --print-to-pdf-margin-left=0.5 `
            --print-to-pdf-margin-right=0.5 `
            --print-to-pdf-paper-width=8.5 `
            --print-to-pdf-paper-height=11 `
            --enable-local-file-access `
            $fileUrl 2>&1 | Out-Null
        
        # Verify output
        if (Test-Path $outputPath) {
            $fileSize = (Get-Item $outputPath).Length / 1MB
            Write-Host "✓ Success: $OutputFile ($([Math]::Round($fileSize, 2)) MB)" -ForegroundColor Green
            return $true
        } else {
            Write-Error "Failed to generate PDF: $outputPath"
            return $false
        }
    }
    catch {
        Write-Error "Error during conversion: $_"
        return $false
    }
}

function Convert-MarkdownToPdf {
    param(
        [string]$InputFile,
        [string]$OutputFile
    )
    
    $inputPath = Join-Path $scriptDir $InputFile
    
    # Check if pandoc is available
    $pandoc = Get-Command pandoc -ErrorAction SilentlyContinue
    if (-not $pandoc) {
        Write-Warning "Pandoc not found. Skipping Markdown: $InputFile"
        Write-Host "  Install with: choco install pandoc" -ForegroundColor Yellow
        return $false
    }
    
    Write-Host "Converting: $InputFile → $OutputFile" -ForegroundColor Cyan
    
    try {
        & pandoc $inputPath `
            --from markdown `
            --to pdf `
            --output $OutputFile `
            --variable geometry:margin=0.5in `
            --variable urlcolor=blue `
            --toc `
            --toc-depth=2 2>&1 | Out-Null
        
        if (Test-Path (Join-Path $scriptDir $OutputFile)) {
            $fileSize = (Get-Item (Join-Path $scriptDir $OutputFile)).Length / 1MB
            Write-Host "✓ Success: $OutputFile ($([Math]::Round($fileSize, 2)) MB)" -ForegroundColor Green
            return $true
        }
    }
    catch {
        Write-Warning "Pandoc conversion failed: $_"
    }
    
    return $false
}

# Main execution
Write-Host "
╔════════════════════════════════════════════════════════╗
║   LLM Gateway Documentation PDF Converter             ║
║   Version 1.0.0                                        ║
╚════════════════════════════════════════════════════════╝
" -ForegroundColor Cyan

Write-Host "Using Chrome: $chromeExePath`n"

$successCount = 0
$totalCount = 0

if ($AllDocuments) {
    Write-Host "Generating all documentation PDFs...`n" -ForegroundColor Yellow
    
    $documents = @(
        @{ Input = "LLM_Gateway_User_Manual.html"; Output = "LLM_Gateway_User_Manual$($Version ? "_v$Version" : "").pdf"; Type = "html" },
        @{ Input = "QUICK_REFERENCE_GUIDE.md"; Output = "QUICK_REFERENCE_GUIDE$($Version ? "_v$Version" : "").pdf"; Type = "markdown" },
        @{ Input = "API_REFERENCE.md"; Output = "API_REFERENCE$($Version ? "_v$Version" : "").pdf"; Type = "markdown" },
        @{ Input = "PDF_GENERATION_GUIDE.md"; Output = "PDF_GENERATION_GUIDE$($Version ? "_v$Version" : "").pdf"; Type = "markdown" }
    )
    
    foreach ($doc in $documents) {
        $totalCount++
        
        if ($doc.Type -eq "html") {
            if (Convert-HtmlToPdf -InputFile $doc.Input -OutputFile $doc.Output) {
                $successCount++
            }
        } else {
            if (Convert-MarkdownToPdf -InputFile $doc.Input -OutputFile $doc.Output) {
                $successCount++
            }
        }
    }
    
    # Create archive
    Write-Host "`nCreating documentation archive..." -ForegroundColor Cyan
    $archiveName = "LLM_Gateway_Docs$($Version ? "_v$Version" : "_v1.0.0").zip"
    $tempDir = Join-Path $env:TEMP "llm_gateway_docs_$$"
    
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    Get-ChildItem $scriptDir -Filter "*.pdf" | Where-Object { $_.BaseName -match "LLM_Gateway|QUICK_REFERENCE|API_REFERENCE|PDF_GENERATION" } | Copy-Item -Destination $tempDir
    Copy-Item (Join-Path $scriptDir "DOCUMENTATION_INDEX.md") -Destination $tempDir -ErrorAction SilentlyContinue
    
    Compress-Archive -Path "$tempDir/*" -DestinationPath (Join-Path $scriptDir $archiveName) -Force
    Remove-Item -Recurse -Force $tempDir
    
    Write-Host "✓ Archive created: $archiveName" -ForegroundColor Green
    
} else {
    $totalCount = 1
    
    # Set output filename
    if (-not $Output) {
        if ($Source -match "\.html$") {
            $Output = $Source -replace "\.html$", ".pdf"
        } elseif ($Source -match "\.md$") {
            $Output = $Source -replace "\.md$", ".pdf"
        } else {
            $Output = "$Source.pdf"
        }
        
        if ($Version) {
            $Output = $Output -replace "\.pdf$", "_v$Version.pdf"
        }
    }
    
    # Convert based on file type
    if ($Source -match "\.html$") {
        if (Convert-HtmlToPdf -InputFile $Source -OutputFile $Output) {
            $successCount++
        }
    } elseif ($Source -match "\.md$") {
        if (Convert-MarkdownToPdf -InputFile $Source -OutputFile $Output) {
            $successCount++
        }
    } else {
        Write-Error "Unknown file type: $Source (must be .html or .md)"
    }
}

# Summary
Write-Host "`n" + ("=" * 50) -ForegroundColor Cyan
Write-Host "Conversion Summary" -ForegroundColor Cyan
Write-Host "=" * 50 -ForegroundColor Cyan
Write-Host "Total documents: $totalCount"
Write-Host "Successful: $successCount" -ForegroundColor $(if ($successCount -eq $totalCount) { "Green" } else { "Yellow" })
Write-Host "Failed: $($totalCount - $successCount)"
Write-Host "=" * 50 -ForegroundColor Cyan

if ($successCount -gt 0) {
    Write-Host "`nOutput location: $scriptDir" -ForegroundColor Green
    Write-Host "Files are ready for printing and distribution!`n" -ForegroundColor Green
    exit 0
} else {
    Write-Host "`nNo PDFs were generated. Check errors above." -ForegroundColor Red
    exit 1
}
