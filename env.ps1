Get-Content .env | ForEach-Object {
   $var = $_ -split "="
   Set-Item -Path "env:\$($var[0])" -Value $var[1]
 }