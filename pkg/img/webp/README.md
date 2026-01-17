---
SPDX-FileCopyrightText: 2025 Mislav Marohnić <hi@mislav.net>
SPDX-License-Identifier: CC0-1.0
---

# WebP decoder

This is a pure Go WebP image decoder inlined from `golang.org/x/image/webp` so that a [patch](https://github.com/golang/image/pull/16) could be applied to it.

```sh
curl -L https://github.com/golang/image/raw/refs/heads/master/webp/decode.go -o internal/webp/decode.go
curl -L https://github.com/golang/image/pull/16.patch | git apply --directory internal --exclude '*_test.go'
```
