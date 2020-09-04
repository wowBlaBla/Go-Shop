#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
APPLICATION="GoShop"
echo "Application: "$APPLICATION
VERSION="1.0.0"
echo "Version: "$VERSION
COMPILED=$(date +%Y%m%d%H%M%S)
echo "Compiled: "$COMPILED

declare -a array=("common/common.go")
for file in "${array[@]}"
do
   echo "File: "$file
   sed -i.back 's/APPLICATION = ".*"/APPLICATION = "'$APPLICATION'"/g' $DIR/$file
   if [ $? -eq 0 ];then
      rm $DIR/$file.back
   else
      echo "Fail to update application"
      mv $DIR/$file.back $DIR/$file
      exit 1
   fi
   sed -i.back 's/VERSION = ".*"/VERSION = "'$VERSION'"/g' $DIR/$file
   if [ $? -eq 0 ];then
      rm $DIR/$file.back
   else
      echo "Fail to update version"
      mv $DIR/$file.back $DIR/$file
      exit 1
   fi
   sed -i.back 's/COMPILED = ".*"/COMPILED = "'$COMPILED'"/g' $DIR/$file
   if [ $? -eq 0 ];then
      rm $DIR/$file.back
   else
      echo "Fail to update compiled"
      mv $DIR/$file.back $DIR/$file
      exit 1
   fi
   echo "OK"
done
