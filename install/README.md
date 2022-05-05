# kluctl Installation

Binaries for macOS and Linux AMD64 are available for download on the
[release page](https://github.com/kluctl/kluctl/releases).

To install the latest release run:

```bash
curl -s https://raw.githubusercontent.com/kluctl/kluctl/main/install/kluctl.sh | bash
```

The install script does the following:
* attempts to detect your OS
* downloads and unpacks the release tar file in a temporary directory
* copies the kluctl binary to `/usr/local/bin`
* removes the temporary directory

## Alternative installation methods

See https://kluctl.io/docs/installation for alternative installation methods.

## Build from source

Clone the repository:

```bash
git clone https://github.com/kluctl/kluctl
cd kluctl
```

Build the `kluctl` binary (requires go >= 1.18 and python >= 3.10):

```bash
make build
```

Run the binary:

```bash
./bin/kluctl -h
```
