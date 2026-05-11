# 릴리스 서명 검증

`solactl` 의 모든 GitHub 릴리스는 [cosign keyless](https://docs.sigstore.dev/cosign/signing/overview/) 로 서명됩니다. 본 문서는 서명을 검증하는 절차와 그 의미를 설명합니다.

## TL;DR

```bash
TAG=$(curl -fsSL https://api.github.com/repos/solapi/solactl/releases/latest \
  | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
BASE="https://github.com/solapi/solactl/releases/download/${TAG}"

curl -fsSLO "${BASE}/checksums.txt"
curl -fsSLO "${BASE}/checksums.txt.sig"
curl -fsSLO "${BASE}/checksums.txt.pem"

cosign verify-blob checksums.txt \
  --signature checksums.txt.sig \
  --certificate checksums.txt.pem \
  --certificate-identity-regexp '^https://github\.com/solapi/solactl/\.github/workflows/release-please\.yml@refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

성공 시 `Verified OK` 가 출력됩니다.

## 검증이 보장하는 것

서명 검증에 성공하면 다음이 증명됩니다.

1. `checksums.txt` 는 **이 저장소** (`github.com/solapi/solactl`) 의
2. **`.github/workflows/release-please.yml` 워크플로우** 가
3. **태그된 릴리스 시점** (`refs/tags/v<X>.<Y>.<Z>`) 에 생성/서명한 파일이라는 점.

이후 `install.sh` / `install.ps1` (또는 수동) 이 `checksums.txt` 로 각 바이너리 아카이브의 SHA256 을 검증하므로,

> **서명된 체크섬 → 검증된 바이너리**

의 신뢰 체인이 완성됩니다.

## 일반 설치와의 관계

| 경로 | 검증 단계 | 추가 도구 |
|------|-----------|-----------|
| 일반 설치 (`install.sh` / `install.ps1`) | TLS + SHA256 (`checksums.txt`) | 불필요 |
| **고급 검증 (본 문서)** | TLS + SHA256 + cosign 서명 + Rekor 트랜스페어런시 로그 | `cosign` 2.0+ |

일반 설치 경로는 SHA256 만 검증하고, 서명 검증은 추가 보증이 필요한 사용자가 선택적으로 수행합니다.

## 신뢰 모델

cosign keyless 는 다음 구성 요소를 신뢰합니다.

- **Sigstore Fulcio CA** — 짧은 수명의 X.509 인증서 발급
- **GitHub Actions OIDC Provider** (`token.actions.githubusercontent.com`) — 워크플로우 실행 시점 신원 발급
- **Sigstore Rekor** — 모든 서명을 외부에서 검증 가능한 트랜스페어런시 로그에 기록

키 페어를 직접 관리하지 않으므로 키 분실/회전 이슈가 없으며, Rekor 덕분에 *과거 어떤 서명이 발급되었는지* 가 외부에서 감사 가능합니다.

## 필수 도구

[cosign](https://docs.sigstore.dev/cosign/installation/) 2.0 이상.

```bash
# macOS
brew install cosign

# Linux (예시 — 최신 버전은 cosign 릴리스 페이지 참조)
curl -fsSL -o /tmp/cosign \
  https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
chmod +x /tmp/cosign && sudo mv /tmp/cosign /usr/local/bin/cosign

# Windows (PowerShell)
winget install sigstore.cosign
```

## 단계별 절차

### 1. 검증할 태그 선택

```bash
# 최신 릴리스
TAG=$(curl -fsSL https://api.github.com/repos/solapi/solactl/releases/latest \
  | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

# 또는 특정 태그
TAG=v0.1.7
```

### 2. 아티팩트 다운로드

```bash
BASE="https://github.com/solapi/solactl/releases/download/${TAG}"
curl -fsSLO "${BASE}/checksums.txt"
curl -fsSLO "${BASE}/checksums.txt.sig"
curl -fsSLO "${BASE}/checksums.txt.pem"
```

| 파일 | 용도 |
|------|------|
| `checksums.txt` | 각 바이너리 아카이브의 SHA256 목록 |
| `checksums.txt.sig` | cosign 서명 |
| `checksums.txt.pem` | 서명에 사용된 ephemeral 인증서 |

### 3. cosign 으로 검증

```bash
cosign verify-blob checksums.txt \
  --signature checksums.txt.sig \
  --certificate checksums.txt.pem \
  --certificate-identity-regexp '^https://github\.com/solapi/solactl/\.github/workflows/release-please\.yml@refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

#### 인자 설명

- `--certificate-identity-regexp` — 서명자의 OIDC subject 가 매칭되어야 하는 정규식. **이 저장소의 `release-please.yml` 워크플로우가 정식 태그(`v<X>.<Y>.<Z>`) 위에서 실행되었을 때** 만 통과합니다. 위조된 fork 나 임의 브랜치에서 만든 서명은 거부됩니다.
- `--certificate-oidc-issuer` — OIDC 토큰 발급자. GitHub Actions 가 발급한 토큰만 허용.

### 4. 바이너리 검증 (체인 완성)

`checksums.txt` 가 진본임이 확인되었으므로, 그 안의 SHA256 으로 다운로드한 아카이브를 검증합니다.

```bash
# Linux/macOS
sha256sum --check --ignore-missing checksums.txt

# 또는 특정 파일만
grep "solactl_0.1.7_linux_amd64.tar.gz$" checksums.txt | sha256sum --check
```

## 트러블슈팅

### `Error: no matching signatures`

서명 또는 인증서 파일과 `checksums.txt` 가 동일 릴리스의 것이 아닐 가능성이 큽니다. 셋 다 동일 `${BASE}` 에서 다시 받으세요.

### `Error: certificate identity not matched`

`--certificate-identity-regexp` 가 인증서의 실제 subject 와 매치되지 않습니다. 다음을 확인하세요.

```bash
openssl x509 -in checksums.txt.pem -noout -text \
  | grep -A1 "Subject Alternative Name"
```

`URI:https://github.com/solapi/solactl/.github/workflows/release-please.yml@refs/tags/vX.Y.Z` 형태여야 합니다. fork 또는 다른 워크플로우 경유로 서명된 경우 거부됩니다.

### `Error: rekor log entry not found`

네트워크가 [rekor.sigstore.dev](https://rekor.sigstore.dev) 에 접근할 수 없거나, 서명이 transparency log 에 기록되지 않았습니다. 회사 프록시 환경이면 해당 도메인을 허용 목록에 추가하세요.

## Rekor 에서 직접 조회

서명은 Sigstore Rekor 트랜스페어런시 로그에 기록되며, 검증 도구 없이도 브라우저에서 조회할 수 있습니다.

- https://search.sigstore.dev/

`checksums.txt` 의 SHA256 또는 인증서의 fingerprint 로 검색하면 해당 릴리스의 서명 이력을 확인할 수 있습니다.

## 적용 이력

이 검증 절차는 본 저장소가 cosign keyless 서명을 도입한 **이후 릴리스부터** 적용됩니다. 도입 이전 릴리스에는 `checksums.txt.sig` / `checksums.txt.pem` 가 존재하지 않으며, 해당 릴리스는 SHA256 무결성 검증만으로 안전성을 평가해야 합니다.
