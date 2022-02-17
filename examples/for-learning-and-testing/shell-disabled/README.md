This example is executed with shell helpers and hooks disabled, so neither the hooks or shell helpers should execute. 

{{ shell "./example-script.sh" "This should not execute and instead be replaced with a placeholder" }}