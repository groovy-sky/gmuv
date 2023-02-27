#!/bin/bash
set -e

# easy to debug if anything wrong
go version

./gmuv -u $1 -r $2 