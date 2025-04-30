#!/bin/bash

# CHECKLIST
# Change version # in code
# Change version # below
# commit changes
# push commit
# run script
version=0.2.1
image_name=gabehf/sonarr-anime-importer

git tag v$version
git push origin v$version
docker build --tag $image_name:$version .
docker tag $image_name:$version $image_name:latest
docker push $image_name:$version
docker push $image_name:latest