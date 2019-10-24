#!/bin/sh

NAME="gosb"
CURRENT=`pwd`

# Compiling
printf "%`tput cols`s"|tr ' ' '.'
echo "Compiling"
cd src/
GOOS=linux GOARCH=amd64 ./make.bash --no-banner
cd ..
