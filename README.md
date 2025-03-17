# kubelet-device-plugins

[![GitHub License](https://img.shields.io/github/license/anza-labs/kubelet-device-plugins)][license]
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](code_of_conduct.md)
[![GitHub issues](https://img.shields.io/github/issues/anza-labs/kubelet-device-plugins)](https://github.com/anza-labs/kubelet-device-plugins/issues)
[![GitHub release](https://img.shields.io/github/release/anza-labs/kubelet-device-plugins)](https://GitHub.com/anza-labs/kubelet-device-plugins/releases/)
[![Go Report Card](https://goreportcard.com/badge/github.com/anza-labs/kubelet-device-plugins)](https://goreportcard.com/report/github.com/anza-labs/kubelet-device-plugins)

`kubelet-device-plugins` is a Kubernetes Device Plugin that manages access to Linux devices.

- [kubelet-device-plugins](#kubelet-device-plugins)
  - [Features](#features)
  - [Installation](#installation)
  - [Usage](#usage)
    - [KVM](#kvm)
    - [TUN](#tun)
  - [How It Works](#how-it-works)
  - [Compatibility](#compatibility)
  - [License](#license)
  - [Attributions](#attributions)

## Features

- Provides access to Linux devices for containers running in Kubernetes.
- Implements the Kubernetes Device Plugin API to manage device allocation.
- Ensures that only workloads explicitly requesting device access receive it.

## Installation

To deploy the `kubelet-device-plugins`, apply the provided manifests:

```sh
LATEST="$(curl -s 'https://api.github.com/repos/anza-labs/kubelet-device-plugins/releases/latest' | jq -r '.tag_name')"
kubectl apply -k "https://github.com/anza-labs/kubelet-device-plugins/?ref=${LATEST}"
```

## Usage

### KVM

To request access to `/dev/kvm` in a pod, specify the device resource in the `resources` section:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kvm-checker
spec:
  restartPolicy: Never
  containers:
    - name: kvm-checker
      image: busybox
      command: ["sh", "-c", "[ -e /dev/kvm ]"]
      resources:
        requests:
          devices.anza-labs.dev/kvm: '1' # Request KVM device
        limits:
          devices.anza-labs.dev/kvm: '1' # Limit KVM device
```

### TUN

To request access to `tun` in a pod, specify the device resource in the `resources` section:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kvm-checker
spec:
  restartPolicy: Never
  containers:
    - name: kvm-checker
      image: busybox
      command: ["sh", "-c", "[ -e /dev/net/tun ]"]
      resources:
        requests:
          devices.anza-labs.dev/tun: '1' # Request TUN device
        limits:
          devices.anza-labs.dev/tun: '1' # Limit TUN device
```

## How It Works

1. The `kubelet-device-plugins` registers with the kubelet and advertises available KVM devices.
2. When a pod requests the `devices.anza-labs.dev/kvm` resource, the device plugin assigns a `/dev/kvm` device to the container.
3. The container is granted access to `/dev/kvm` for virtualization tasks.

## Compatibility

- Kubernetes 1.20+
- Nodes must have KVM enabled (check with `lsmod | grep kvm`)

## License

`kubelet-device-plugins` are licensed under the [Apache-2.0][license].

## Attributions

This codebase is inspired by:
- [github.com/squat/generic-device-plugin](https://github.com/squat/generic-device-plugin)
- [github.com/kubevirt/kubernetes-device-plugins](https://github.com/kubevirt/kubernetes-device-plugins)

<!-- Resources -->

[license]: https://github.com/anza-labs/kubelet-device-plugins/blob/main/LICENSE
