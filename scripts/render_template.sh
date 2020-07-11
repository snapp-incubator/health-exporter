#!/bin/bash
set -e
PROJECT_NAME=$(basename "`pwd`")
sed -i '' "s/TEMPLATE_SUBSTITUTE_PROJECT_NAME/$PROJECT_NAME/g" deploy/okd.yml
sed -i '' "s/TEMPLATE_SUBSTITUTE_PROJECT_NAME/$PROJECT_NAME/g" Makefile
sed -i '' "s/TEMPLATE_SUBSTITUTE_PROJECT_NAME/$PROJECT_NAME/g" README.md

PROJECT_GIT=$(git remote -v | grep origin | head -1 | awk '{ print $2 }')
sed -i '' "s/TEMPLATE_SUBSTITUTE_PROJECT_GIT/$PROJECT_GIT/g" deploy/okd.yml