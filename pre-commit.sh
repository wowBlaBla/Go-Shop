#!/bin/sh
FILE=common/common.go
if test -f "$FILE"; then
	DATE=$(date +"%Y%m%d%H%M%S")
	sed -i '' "s/COMPILED = \"[0-9]\{14\}\"/COMPILED = \"$DATE\"/" $FILE
fi
