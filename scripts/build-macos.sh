#!/bin/bash
set -euo pipefail
out=${1:-dist/darwin-arm64}
arch=${2:-arm64}
mkdir -p "$out/dfman-notify.app/Contents/MacOS"
cp native/macos/Info.plist "$out/dfman-notify.app/Contents/Info.plist"
swift_arch=$arch
[ "$arch" != amd64 ] || swift_arch=x86_64
xcrun swiftc -swift-version 5 -target "$swift_arch-apple-macosx11.0" native/macos/main.swift -o "$out/dfman-notify.app/Contents/MacOS/dfman-notify" -framework AppKit -framework UserNotifications
codesign --force --sign "${DFMAN_SIGN_IDENTITY:--}" "$out/dfman-notify.app"
GOOS=darwin GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${DFMAN_VERSION:-dev}" -o "$out/dfman" ./cmd/dfman
