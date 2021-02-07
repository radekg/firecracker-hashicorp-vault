# Baby steps with Firecracker

This document assumes a freshly installed HWE Ubuntu 18.04.05.

## Prepare the environment for all dependencies we will need further

```sh
sudo mkdir -p /firecracker
sudo chown -R ${USER}:${USER} /firecracker
mkdir -p /firecracker/{configs,filesystems,kernels,linux.git,releases}
```

```sh
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
sudo apt-get update
sudo apt-get install \
    bison \
    build-essential \
    flex \
    git \
    libelf-dev \
    libncurses5-dev \
    libssl-dev -y
sudo apt-get install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common -y
sudo apt-get install \
    docker-ce \
    docker-ce-cli \
    containerd.io -y
sudo groupadd docker # this may report that the group already exists
sudo usermod -aG docker $USER
```

Log out and log back in so the group membership is re-evaluated.

1. Put `install-firecracker.sh` in `/firecracker` directory.
2. Put `linux-kernel.config` in `/firecracker` directory.
3. `chmod +x /firecracker/install-firecracker.sh`

## Install latest Firecracker

```sh
/firecracker/install-firecracker.sh
```

This will install Firecracker in `/firecracker/releases/release-X` directory and link `firecracker` and `jailer` binaries on the `PATH`.

## Get Linux kernel

```sh
export KERNEL_VERSION=v5.8
cd /firecracker/linux.git
git clone https://github.com/torvalds/linux.git .
git checkout ${KERNEL_VERSION}
```

## Configure Linux kernel

```sh
cp /firecracker/linux-kernel.config /firecracker/linux.git/.config
```

The `linux-kernel.config` file comes from https://raw.githubusercontent.com/firecracker-microvm/firecracker/master/resources/microvm-kernel-x86_64.config.

## Build the kernel

You may have to decrease the number of parallel tasks, I'm using `32`.

```sh
time make vmlinux -j32
...
  LD      vmlinux.o
  MODPOST vmlinux.symvers
  MODINFO modules.builtin.modinfo
  GEN     modules.builtin
  LD      .tmp_vmlinux.kallsyms1
  KSYM    .tmp_vmlinux.kallsyms1.o
  LD      .tmp_vmlinux.kallsyms2
  KSYM    .tmp_vmlinux.kallsyms2.o
  LD      vmlinux
  SORTTAB vmlinux
  SYSMAP  System.map

real	0m54.052s
user	23m51.313s
sys	2m35.287s
```

And move the `vmlinux` build to our common directory:

```sh
mv ./vmlinux /firecracker/kernels/vmlinux-${KERNEL_VERSION}
```

## Build an ext4 file system and put HashiCorp Vault on it

I am not sure what is the minimum file system size I need. 50M is not enough so I go with 500M, this can be for sure optimised.

```sh
export FS=vault
rm /firecracker/filesystems/vault-root.ext4
dd if=/dev/zero of=/firecracker/filesystems/vault-root.ext4 bs=1M count=500
mkfs.ext4 /firecracker/filesystems/vault-root.ext4
mkdir -p /firecracker/filesystems/mnt-${FS}
sudo mount /firecracker/filesystems/${FS}-root.ext4 /firecracker/filesystems/mnt-${FS}
export CONTAINER_ID=$(docker run -t --rm -v /firecracker/filesystems/mnt-${FS}:/export-rootfs -d vault:latest)
docker exec -ti ${CONTAINER_ID} /bin/sh
```

In the container shell:

```sh
apk add openrc
apk add util-linux

# Set up a login terminal on the serial console (ttyS0):
ln -s agetty /etc/init.d/agetty.ttyS0
echo ttyS0 > /etc/securetty
rc-update add agetty.ttyS0 default

# Make sure special file systems are mounted on boot:
rc-update add devfs boot
rc-update add procfs boot
rc-update add sysfs boot
rc-update add local default

echo "#!/bin/sh" >> /etc/local.d/HelloWorld.start
echo "/usr/local/bin/docker-entrypoint.sh server -dev && reboot || reboot" >> /etc/local.d/HashiCorpVault.start
chmod +x /etc/local.d/HashiCorpVault.start
echo rc_verbose=yes > /etc/conf.d/local

# Then, copy the newly configured system to the rootfs image:
for d in home vault; do tar c "/$d" | tar x -C /export-rootfs; done
for d in bin etc home lib root sbin usr vault; do tar c "/$d" | tar x -C /export-rootfs; done
for dir in dev proc run sys var; do mkdir /export-rootfs/${dir}; done

# All done, exit docker shell
exit
```

Stop the container and unmount the file system:

```sh
docker stop ${CONTAINER_ID}
sudo umount /firecracker/filesystems/mnt-${FS}
```

## Put the Vault in Firecracker together

[This is directly lifted from Julia Evans](https://jvns.ca/blog/2021/01/23/firecracker--start-a-vm-in-less-than-a-second/).

Prepare kernel boot args:

```sh
# set up the kernel boot args
export MASK_LONG="255.255.255.252"
export FC_IP="169.254.0.21"
export TAP_IP="169.254.0.22"
export KERNEL_BOOT_ARGS="ro console=ttyS0 noapic reboot=k panic=1 pci=off nomodules random.trust_cpu=on"
export KERNEL_BOOT_ARGS="${KERNEL_BOOT_ARGS} ip=${FC_IP}::${TAP_IP}:${MASK_LONG}::eth0:off"
```

Set up a tap network interface for the Firecracker VM to user:

```sh
export TAP_DEV="fc-88-tap0"
export MASK_SHORT="/30"
export FC_MAC="02:FC:00:00:00:05"

sudo ip link del "$TAP_DEV" 2> /dev/null || true
sudo ip tuntap add dev "$TAP_DEV" mode tap
sudo sysctl -w net.ipv4.conf.${TAP_DEV}.proxy_arp=1 > /dev/null
sudo sysctl -w net.ipv6.conf.${TAP_DEV}.disable_ipv6=1 > /dev/null
sudo ip addr add "${TAP_IP}${MASK_SHORT}" dev "$TAP_DEV"
sudo ip link set dev "$TAP_DEV" up
```

Write the config file of the VM:

```sh
cat <<EOF > /firecracker/configs/vault-config.json
{
  "boot-source": {
    "kernel_image_path": "/firecracker/kernels/vmlinux-${KERNEL_VERSION}",
    "boot_args": "$KERNEL_BOOT_ARGS"
  },
  "drives": [
    {
      "drive_id": "rootfs",
      "path_on_host": "/firecracker/filesystems/${FS}-root.ext4",
      "is_root_device": true,
      "is_read_only": false
    }
  ],
  "network-interfaces": [
      {
          "iface_id": "eth0",
          "guest_mac": "${FC_MAC}",
          "host_dev_name": "${TAP_DEV}"
      }
  ],
  "machine-config": {
    "vcpu_count": 1,
    "mem_size_mib": 128,
    "ht_enabled": false
  }
}
EOF
```

Run the VM:

```sh
firecracker --no-api --config-file /firecracker/configs/vault-config.json
```