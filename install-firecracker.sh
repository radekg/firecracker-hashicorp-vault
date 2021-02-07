#!/bin/bash

set -eu

install_dir=/firecracker/releases
bin_dir=/usr/bin
release_url="https://github.com/firecracker-microvm/firecracker/releases"
latest=$(basename $(curl -fsSLI -o /dev/null -w  %{url_effective} ${release_url}/latest))
arch=`uname -m`

if [ -d "${install_dir}/${latest}" ]; then
        echo "${latest} already installed"
else
        echo "downloading firecracker-${latest}-${arch}.tgz to ${install_dir}"
        curl -o "${install_dir}/firecracker-${latest}-${arch}.tgz" -L "${release_url}/download/${latest}/firecracker-${latest}-${arch}.tgz"
        pushd "${install_dir}"

        echo "decompressing firecracker-${latest}-${arch}.tgz in ${install_dir}"
        tar -xzf "firecracker-${latest}-${arch}.tgz"
        rm "firecracker-${latest}-${arch}.tgz"

        echo "linking firecracker ${latest}-${arch}"
        sudo ln -sfn "${install_dir}/release-${latest}/firecracker-${latest}-${arch}" "${bin_dir}/firecracker-${latest}-${arch}"
        sudo ln -sfn "${install_dir}/release-${latest}/jailer-${latest}-${arch}" "${bin_dir}/jailer-${latest}-${arch}"
        sudo ln -sfn "${bin_dir}/firecracker-${latest}-${arch}" "${bin_dir}/firecracker"
        sudo ln -sfn "${bin_dir}/jailer-${latest}-${arch}" "${bin_dir}/jailer"

        echo "firecracker ${latest}-${arch}: ready"
        firecracker --help | head -n1
fi