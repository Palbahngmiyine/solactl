# solactl

> **Preview** — 이 프로젝트는 현재 프리뷰 단계입니다. API와 CLI 인터페이스가 변경될 수 있습니다.

SOLAPI 메시징 플랫폼을 위한 CLI 도구입니다.

## 플랫폼 지원

Linux와 macOS를 우선 지원합니다.

| 플랫폼 | 아키텍처 | 지원 |
|--------|----------|------|
| Linux | amd64, arm64 | 우선 지원 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) | 우선 지원 |
| Windows | amd64, arm64 | 바이너리 제공 (제한적 테스트) |

## 설치

### 스크립트 설치 (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/solapi/solactl/main/scripts/install.sh | bash
```

`~/.local/bin`에 설치됩니다. PATH에 포함되어 있지 않으면 안내 메시지가 출력됩니다.

### 소스 빌드

```bash
git clone https://github.com/solapi/solactl.git
cd solactl
make build        # bin/solactl 생성
make install      # $GOPATH/bin에 설치
```

### 업그레이드

```bash
solactl upgrade
```

## 사용법

```bash
# 초기 설정 (API Key / Secret)
solactl configure

# SMS 발송
solactl send sms --to 01012345678 --text "안녕하세요"

# 발신번호 목록
solactl senderid list

# 발송 내역 조회
solactl messages list

# 잔액 조회
solactl balance
```

자세한 사용법은 `solactl --help` 또는 각 서브커맨드의 `--help`를 참조하세요.

