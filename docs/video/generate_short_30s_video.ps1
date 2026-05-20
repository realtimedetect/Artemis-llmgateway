$ErrorActionPreference = 'Stop'

Set-Location (Join-Path $PSScriptRoot '..\..')

$ffmpeg = 'ffmpeg'
if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
  $candidate = 'C:\Users\vinod\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-8.0.1-full_build\bin\ffmpeg.exe'
  if (Test-Path $candidate) {
    $ffmpeg = $candidate
  } else {
    throw 'ffmpeg not found. Run . $PROFILE in your terminal or install ffmpeg first.'
  }
}

$videoDir = Join-Path (Get-Location) 'docs\video'
$draftDir = Join-Path $videoDir 'draft'
$screensDir = Join-Path $videoDir 'screenshots\short30'
New-Item -ItemType Directory -Force -Path $draftDir | Out-Null
New-Item -ItemType Directory -Force -Path $screensDir | Out-Null

$baseVideo = Join-Path $draftDir 'llm_gateway_short_30s_base.mp4'
$subbedVideo = Join-Path $draftDir 'llm_gateway_short_30s.mp4'
$voiceInput = Join-Path $videoDir 'voiceover_short_30s.wav'
$voiceOutput = Join-Path $draftDir 'llm_gateway_short_30s_with_voice.mp4'
$subsFile = Join-Path $videoDir 'llm_gateway_short_30s_subtitles.srt'

$fontBold = 'C\:/Windows/Fonts/segoeuib.ttf'
$fontRegular = 'C\:/Windows/Fonts/segoeui.ttf'
$sceneListFile = Join-Path $draftDir 'short30_scenes.txt'
$sceneFiles = @()

$scenes = @(
  @{
    Name = 'scene01'
    Duration = 6.0
    Screenshot = Join-Path $screensDir '01-overview.png'
    Title = 'Overview and Visibility'
    Detail = 'Requests, latency, cost, and provider health in one dashboard'
    Fallback = '0x0f172a'
  },
  @{
    Name = 'scene02'
    Duration = 5.5
    Screenshot = Join-Path $screensDir '02-providers.png'
    Title = 'Multi-Provider Resilience'
    Detail = 'Provider registry, health state, retries, and failover readiness'
    Fallback = '0x052e16'
  },
  @{
    Name = 'scene03'
    Duration = 7.0
    Screenshot = Join-Path $screensDir '03-routes.png'
    Title = 'Policy-Based Routing'
    Detail = 'Stable route slugs, prompts, limits, and provider abstraction'
    Fallback = '0x172554'
  },
  @{
    Name = 'scene04'
    Duration = 6.0
    Screenshot = Join-Path $screensDir '04-costs.png'
    Title = 'Cost Intelligence'
    Detail = 'Pricing rules, spend groups, and live usage visibility'
    Fallback = '0x1f2937'
  },
  @{
    Name = 'scene05'
    Duration = 5.5
    Screenshot = Join-Path $screensDir '05-audits.png'
    Title = 'Audit and Governance'
    Detail = 'Trace requests, inspect payloads, and support compliance reviews'
    Fallback = '0x0c4a6e'
  }
)

$missingScreens = @()

foreach ($scene in $scenes) {
  $sceneFile = Join-Path $draftDir ($scene.Name + '.mp4')
  $sceneFiles += $sceneFile
  $durationText = [string]$scene.Duration

  if (Test-Path $scene.Screenshot) {
    $filter = "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,drawbox=x=56:y=760:w=1808:h=220:color=black@0.62:t=fill,drawtext=fontfile='$fontBold':text='$($scene.Title)':fontsize=48:fontcolor=white:x=88:y=800,drawtext=fontfile='$fontRegular':text='$($scene.Detail)':fontsize=30:fontcolor=0xdbeafe:x=88:y=870"
    & $ffmpeg -y -loop 1 -framerate 30 -t $durationText -i $scene.Screenshot -f lavfi -i "anullsrc=cl=stereo:r=48000" -shortest -vf $filter -c:v libx264 -pix_fmt yuv420p -c:a aac -b:a 128k $sceneFile
  } else {
    $missingScreens += $scene.Screenshot
    $filter = "drawbox=x=56:y=180:w=1808:h=620:color=black@0.18:t=fill,drawtext=fontfile='$fontBold':text='llm_gateway':fontsize=88:fontcolor=white:x=(w-text_w)/2:y=240,drawtext=fontfile='$fontBold':text='$($scene.Title)':fontsize=54:fontcolor=white:x=(w-text_w)/2:y=390,drawtext=fontfile='$fontRegular':text='$($scene.Detail)':fontsize=32:fontcolor=0xdbeafe:x=(w-text_w)/2:y=470,drawtext=fontfile='$fontRegular':text='Add screenshot: $([System.IO.Path]::GetFileName($scene.Screenshot))':fontsize=28:fontcolor=0x86efac:x=(w-text_w)/2:y=570"
    & $ffmpeg -y -f lavfi -i "color=c=$($scene.Fallback):s=1920x1080:r=30:d=$durationText" -f lavfi -i "anullsrc=cl=stereo:r=48000" -shortest -vf $filter -c:v libx264 -pix_fmt yuv420p -c:a aac -b:a 128k $sceneFile
  }
}

$sceneFiles | ForEach-Object {
  "file '$($_.Replace("'", "''"))'"
} | Set-Content -Encoding ascii $sceneListFile

& $ffmpeg -y -f concat -safe 0 -i $sceneListFile -c copy $baseVideo

# Burn subtitles.
& $ffmpeg -y -i $baseVideo -vf "subtitles=docs/video/llm_gateway_short_30s_subtitles.srt:force_style='FontName=Segoe UI,FontSize=30,PrimaryColour=&H00FFFFFF&,OutlineColour=&H00222222&,BackColour=&H88000000&,BorderStyle=3,Outline=1,Shadow=0,MarginV=44'" -c:v libx264 -preset medium -crf 20 -c:a copy $subbedVideo

if (Test-Path $voiceInput) {
  & $ffmpeg -y -i $subbedVideo -i $voiceInput -filter_complex "[0:a]volume=0.15[a0];[1:a]volume=1.0[a1];[a0][a1]amix=inputs=2:duration=first:dropout_transition=2[a]" -map 0:v -map "[a]" -c:v copy -c:a aac -b:a 192k $voiceOutput
  Write-Output "Created with voice: $voiceOutput"
} else {
  Write-Output "Voiceover file not found at $voiceInput"
  Write-Output "Created subtitle video: $subbedVideo"
}

if ($missingScreens.Count -gt 0) {
  Write-Output 'Missing screenshots for richer video scenes:'
  $missingScreens | ForEach-Object { Write-Output " - $_" }
}
