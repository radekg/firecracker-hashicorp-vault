#!/bin/bash

sudo apt-get update
sudo apt=get install git build-essential libncurses5-dev flex bison libssl-dev -y

git clone https://github.com/torvalds/linux.git linux.git
cd linux.git

git checkout v5.8

# put the contents on the linux.kernel.config in .config file
# that config comes from https://raw.githubusercontent.com/firecracker-microvm/firecracker/master/resources/microvm-kernel-x86_64.config
# and was recommended for v4.20 kernel version so the first time one runs the kernel build,
# there will be questions to answer
# whatever, I don't know what I'm doing here...

# you may have to decrease the number of parallel tasks, I'm using 32
time make vmlinux -j32
# ...
# real	0m54.052s
# user	23m51.313s
# sys	2m35.287s