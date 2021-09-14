#!/bin/bash

echo "" >> out.txt
echo $1 >> out.txt

echo "~/.snap/data/$1/current" >> out.txt
ls -Ca ~/.snap/data/$1/current >> out.txt

echo "~/.snap/data/$1/common" >> out.txt
ls -Ca ~/.snap/data/$1/common >> out.txt

echo "~/Snap/$1" >> out.txt
ls -Ca ~/Snap/$1 >> out.txt

