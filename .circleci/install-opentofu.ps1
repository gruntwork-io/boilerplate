OpenTofuInstallPath = "C:\Program Files\tofu\tofu.exe"
$OpenTofuTmpPath = "C:\OpenTofutmp"
$OpenTofuTmpBinaryPath = "C:\OpenTofutmp\tofu.exe"
$OpenTofuPath = "C:\Program Files\tofu"
if (Test-Path -Path $OpenTofuInstallPath)
{
	Remove-Item $OpenTofuInstallPath -Recurse
}
# Download OpenTofu and unpack it
$OpenTofuURI = "https://github.com/opentofu/opentofu/releases/download/v1.6.2/tofu_1.6.2_windows_amd64.zip"
$output = "tofu_1.6.2_windows_amd64.zip"
$ProgressPreference = "SilentlyContinue"
Invoke-WebRequest -Uri $OpenTofuURI -OutFile $output
New-Item -ItemType "directory" -Path $OpenTofuTmpPath
Expand-Archive -LiteralPath $output -DestinationPath $OpenTofuTmpPath
New-Item -ItemType "directory" -Path $OpenTofuPath
Move-Item $OpenTofuTmpBinaryPath $OpenTofuPath
$OldPath = [System.Environment]::GetEnvironmentVariable('PATH', "Machine")
$NewPath = "$OldPath;$OpenTofuPath"
[Environment]::SetEnvironmentVariable("PATH", "$NewPath", "Machine")
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
tofu version
