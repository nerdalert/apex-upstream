# Quick Start

This guide will walk you through getting your first devices connected via Nexodus.

## Install and Start the Nexodus Agent

### Fedora

```sh
# Enable the COPR repository and install the nexodus package
sudo dnf copr enable russellb/nexodus
sudo dnf install nexodus

# Start the nexodus service and set it to automatically start on boot
sudo systemctl start nexodus
sudo systemctl enable nexodus
```

Edit `/etc/sysconfig/nexodus` if you plan to use a Nexodus service other than <https://try.nexodus.io>.

```sh

Query the status of `nexd` and follow the instructions to register your device.

```sh
sudo nexctl nexd status
```

### Brew

For Mac, you can install the Nexodus Agent via [Homebrew](https://brew.sh/).

```sh
brew tap nexodus-io/nexodus
brew install nexodus
```

Start `nexd` with `sudo` and follow the instructions to register your device.

```sh
sudo nexd https://try.nexodus.io
```

### Other

Run the Nexodus install script to download and install the latest version of the agent and its dependencies.

```sh
curl https://nexodus-io.s3.amazonaws.com/installer/nexodus-installer.sh -o nexodus-installer.sh
chmod +x nexodus-installer.sh
./nexodus-installer.sh
```

Start `nexd` with `sudo` and follow the instructions to register your device.

```sh
sudo nexd https://try.nexodus.io
```

## Test Connectivity

Once you have the agent installed and running, you can test connectivity between your devices. To determine the IP address assigned to each device, you can check the service web interface at <https://try.nexodus.io>, look the `nexd` logs, or get the IP using `nexctl`.

```sh
sudo nexctl nexd get tunnelip
sudo nexctl nexd get tunnelip --ipv6
```

Try `ping` or whatever other connectivity test you prefer.

```sh
ping 100.100.0.1
```