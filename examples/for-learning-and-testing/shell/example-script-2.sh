#!/bin/bash
# An example script that will be executed via the 'shell' command in a boilerplate template. This script simply writes
# the environment variable TEXT into the path in the first argument.

echo "$TEXT" > "$1"