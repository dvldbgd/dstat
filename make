#!/bin/bash

set -e

go build .

sudo mv dstat /usr/local/bin/

