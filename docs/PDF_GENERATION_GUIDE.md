# LLM Gateway Documentation - PDF Generation Guide

## Overview
This guide explains how to convert the comprehensive HTML documentation to PDF format for printing and distribution.

## Files Included
- **LLM_Gateway_User_Manual.html** - Complete enterprise-level user manual (1000+ pages when printed)
- **Quick_Reference_Guide.md** - Quick reference for common tasks
- **API_Reference.md** - Detailed API endpoint reference

---

## Method 1: Browser Print to PDF (Simplified)

### Steps:
1. Open `LLM_Gateway_User_Manual.html` in your web browser
2. Press **Ctrl+P** (Windows/Linux) or **Cmd+P** (Mac)
3. Configure print settings:
   - **Destination:** Save as PDF
   - **Paper size:** A4 (recommended for standard printing)
   - **Margins:** Normal (0.5 inches)
   - **Background graphics:** ON (for color diagrams)
   - **Headers/footers:** OFF (optional)
4. Click **Save** and choose your save location
5. Name the file: `LLM_Gateway_User_Manual_v1.0.pdf`

**Estimated file size:** 15-25 MB (depending on settings)
**Print quality:** Medium to High

---

## Method 2: Using Headless Chrome (Recommended for Automation)

### Prerequisites:
```bash
# Install Google Chrome/Chromium
# On Windows: Already installed or download from google.com/chrome
# On Mac: brew install google-chrome
# On Linux: sudo apt-get install chromium
```

### Windows PowerShell Script:
```powershell
# Save as: convert-to-pdf.ps1
$chromeExePath = "C:\Program Files\Google\Chrome\Application\chrome.exe"
$htmlFile = "LLM_Gateway_User_Manual.html"
$outputPdf = "LLM_Gateway_User_Manual.pdf"

# Get absolute paths
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$htmlPath = Join-Path $scriptDir $htmlFile
$pdfPath = Join-Path $scriptDir $outputPdf

# Convert to PDF
& $chromeExePath `
  --headless `
  --disable-gpu `
  --print-to-pdf=$pdfPath `
  --print-to-pdf-margin-top=0.5 `
  --print-to-pdf-margin-bottom=0.5 `
  --print-to-pdf-margin-left=0.5 `
  --print-to-pdf-margin-right=0.5 `
  "file:///$($htmlPath.Replace('\', '/'))"

Write-Host "PDF generated: $pdfPath"
```

### Running the script:
```powershell
cd C:\Users\vinod\OneDrive\llm_gateway\docs
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\convert-to-pdf.ps1
```

---

## Method 3: Using Puppeteer (Node.js)

### Prerequisites:
```bash
npm install puppeteer
```

### Script (save as `generate-pdf.js`):
```javascript
const puppeteer = require('puppeteer');
const path = require('path');
const fs = require('fs');

(async () => {
  const browser = await puppeteer.launch({
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  try {
    const page = await browser.newPage();
    
    // Set viewport for consistent rendering
    await page.setViewport({
      width: 1280,
      height: 720,
    });

    // Load HTML file
    const htmlFile = path.resolve(__dirname, 'LLM_Gateway_User_Manual.html');
    await page.goto(`file://${htmlFile}`, {
      waitUntil: 'networkidle2',
      timeout: 60000
    });

    // Wait for dynamic content to load
    await page.waitForTimeout(2000);

    // Generate PDF
    const pdfPath = path.resolve(__dirname, 'LLM_Gateway_User_Manual.pdf');
    await page.pdf({
      path: pdfPath,
      format: 'A4',
      margin: {
        top: '0.5in',
        right: '0.5in',
        bottom: '0.5in',
        left: '0.5in'
      },
      printBackground: true,
      displayHeaderFooter: true,
      headerTemplate: '<div style="font-size: 10px; margin: 10px 20px;">LLM Gateway User Manual</div>',
      footerTemplate: '<div style="font-size: 10px; margin: 10px 20px;"><span class="pageNumber"></span> of <span class="totalPages"></span></div>'
    });

    console.log(`✓ PDF generated successfully: ${pdfPath}`);
    console.log(`✓ File size: ${(fs.statSync(pdfPath).size / 1024 / 1024).toFixed(2)} MB`);

  } catch (error) {
    console.error('Error generating PDF:', error);
  } finally {
    await browser.close();
  }
})();
```

### Running:
```bash
node generate-pdf.js
```

---

## Method 4: Using wkhtmltopdf (Cross-platform)

### Installation:

**Windows:**
```bash
choco install wkhtmltopdf
```

**Mac:**
```bash
brew install --cask wkhtmltopdf
```

**Linux:**
```bash
sudo apt-get install wkhtmltopdf
```

### Command:
```bash
wkhtmltopdf \
  --page-size A4 \
  --margin-top 0.5in \
  --margin-right 0.5in \
  --margin-bottom 0.5in \
  --margin-left 0.5in \
  --print-media-type \
  --enable-local-file-access \
  LLM_Gateway_User_Manual.html \
  LLM_Gateway_User_Manual.pdf
```

---

## Method 5: Using pdfkit (Python)

### Installation:
```bash
pip install pdfkit
# Also requires wkhtmltopdf (install from Method 4 first)
```

### Script (save as `generate_pdf.py`):
```python
import pdfkit
import os

# Configure pdfkit options
options = {
    'page-size': 'A4',
    'margin-top': '0.5in',
    'margin-right': '0.5in',
    'margin-bottom': '0.5in',
    'margin-left': '0.5in',
    'print-media-type': None,
    'enable-local-file-access': None,
}

# Input and output files
script_dir = os.path.dirname(os.path.abspath(__file__))
html_file = os.path.join(script_dir, 'LLM_Gateway_User_Manual.html')
pdf_file = os.path.join(script_dir, 'LLM_Gateway_User_Manual.pdf')

# Convert HTML to PDF
try:
    pdfkit.from_file(html_file, pdf_file, options=options)
    file_size = os.path.getsize(pdf_file) / (1024 * 1024)
    print(f"✓ PDF generated successfully: {pdf_file}")
    print(f"✓ File size: {file_size:.2f} MB")
except Exception as e:
    print(f"✗ Error generating PDF: {e}")
```

### Running:
```bash
python generate_pdf.py
```

---

## Recommended Setup for Enterprise Distribution

### Step 1: Automated PDF Generation
Create a CI/CD pipeline that automatically generates PDFs when documentation is updated.

### Step 2: Versioning
```bash
# Name PDFs with version number
LLM_Gateway_User_Manual_v1.0.0_2024-03-15.pdf
LLM_Gateway_User_Manual_v1.0.1_2024-03-20.pdf
```

### Step 3: Distribution
- **Internal Wiki:** Upload to Confluence/SharePoint
- **Document Management:** Store in OneDrive/Google Drive
- **Email Distribution:** Send to users on new releases
- **Web Portal:** Host on intranet for download

### Step 4: Archive
```bash
# Create archive with all resources
mkdir -p LLM_Gateway_Docs_v1.0.0
cp LLM_Gateway_User_Manual.pdf LLM_Gateway_Docs_v1.0.0/
cp Quick_Reference_Guide.md LLM_Gateway_Docs_v1.0.0/
cp API_Reference.md LLM_Gateway_Docs_v1.0.0/
zip -r LLM_Gateway_Docs_v1.0.0.zip LLM_Gateway_Docs_v1.0.0/
```

---

## Quality Checklist

- [ ] PDF opens without errors
- [ ] All text is readable (font size ≥ 11pt)
- [ ] Tables are properly formatted
- [ ] Code blocks display correctly
- [ ] Images/diagrams are clear
- [ ] Page numbers are visible
- [ ] Table of contents links work
- [ ] PDF file size is reasonable (< 50MB)
- [ ] Can be printed without errors
- [ ] Metadata is correct (author, title, subject)

---

## Troubleshooting

### PDF is too large (> 50MB)
- **Solution:** Disable background graphics, reduce image quality, use compression

### Text is hard to read
- **Solution:** Increase margins, use better font rendering, adjust DPI settings

### Tables are broken across pages
- **Solution:** Add page breaks before large tables, use landscape orientation

### Missing content
- **Solution:** Ensure JavaScript is executed, increase timeout, check for CORS issues

### Slow generation
- **Solution:** Use headless Chrome instead of wkhtmltopdf, parallelize if batch processing

---

## Recommended Tools

| Tool | Platform | Quality | Speed | Cost |
|------|----------|---------|-------|------|
| Chrome DevTools | All | ⭐⭐⭐⭐ | Medium | Free |
| Puppeteer | Node.js | ⭐⭐⭐⭐ | Fast | Free |
| Headless Chrome | All | ⭐⭐⭐⭐ | Very Fast | Free |
| wkhtmltopdf | All | ⭐⭐⭐ | Slow | Free |
| Aspose | Commercial | ⭐⭐⭐⭐⭐ | Fast | Paid |

---

## Tips for Best Results

1. **Use A4 paper size** for international compatibility
2. **Enable print background graphics** for colored elements
3. **Add page numbers and headers** for professional appearance
4. **Generate in headless mode** for CI/CD automation
5. **Test on different viewers** (Adobe Reader, browsers, etc.)
6. **Keep total pages reasonable** - split into multiple documents if needed
7. **Include table of contents** - helps with navigation
8. **Add bookmarks** for PDF sections
9. **Optimize for both screen and print viewing**
10. **Version your documentation** consistently

---

## Creating Variants

### Quick Start Guide
```bash
# Extract only getting-started section
wkhtmltopdf \
  --include-in-outline /dev/stdin \
  <(sed -n '/<section id="getting-started"/,/<\/section>/p' LLM_Gateway_User_Manual.html) \
  LLM_Gateway_Quick_Start.pdf
```

### API Reference Only
```bash
wkhtmltopdf \
  <(sed -n '/<section id="api-integration"/,/<\/section>/p' LLM_Gateway_User_Manual.html) \
  LLM_Gateway_API_Reference.pdf
```

### Administrator Guide
```bash
wkhtmltopdf \
  <(sed -n '/<section id="administration"/,/<\/section>/p' LLM_Gateway_User_Manual.html) \
  LLM_Gateway_Admin_Guide.pdf
```

---

## Support & Updates

For issues with PDF generation:
1. Check browser console for errors
2. Verify HTML file integrity
3. Update your PDF generation tool
4. Try alternative method from this guide
5. Contact support@llm-gateway.com with details

---

**Last Updated:** March 2026
**Document Version:** 1.0.0
