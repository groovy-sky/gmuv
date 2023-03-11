#!/bin/sh

# Exit immediately if a command exits with a non-zero status
set -e

if [ -z "${INPUT_ACCOUNT}" ]; then
  echo "Account is empty."
fi

if [ -z "${INPUT_REPOSITORY}" ]; then
  echo "Repository is empty."
fi

if [ -z "${INPUT_FILENAME}" ]
then
  /gmuv -u "${INPUT_ACCOUNT}" -r "${INPUT_REPOSITORY}" -o cli
else
  /gmuv -u "${INPUT_ACCOUNT}" -r "${INPUT_REPOSITORY}" -o file -f "${INPUT_FILENAME}" 
fi
