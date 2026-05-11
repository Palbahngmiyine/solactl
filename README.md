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

모든 설치 방법은 관리자 권한 없이 **사용자 영역**에 설치됩니다.

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/solapi/solactl/main/scripts/install.sh | bash
```

- 설치 경로: `~/.local/bin/solactl`
- 체크섬(SHA256) 검증 후 압축 해제
- `PATH`에 포함되어 있지 않으면 셸 설정 파일(`~/.zshrc` / `~/.bashrc`) 등록 안내 출력

### Windows (PowerShell)

winget처럼 사용자 영역에만 설치되며 관리자 권한이 필요하지 않습니다. PowerShell 5.1 이상(또는 PowerShell 7+) 에서 실행하세요.

```powershell
irm https://raw.githubusercontent.com/solapi/solactl/main/scripts/install.ps1 | iex
```

- 설치 경로: `%LOCALAPPDATA%\Programs\solactl\solactl.exe`
- 체크섬(SHA256) 검증 후 zip 압축 해제
- 사용자 `PATH`(`HKCU\Environment\Path`) 에 설치 디렉터리를 자동 추가 — 새 터미널부터 적용
- 실행 중인 `solactl.exe` 가 잠겨 있으면 기존 파일을 `solactl.exe.old` 로 옮긴 뒤 교체

#### 옵션

특정 버전 고정 / 설치 경로 지정이 필요하면 스크립트를 로컬에 받아 인자로 실행합니다.

```powershell
# 스크립트 다운로드 후 실행
Invoke-WebRequest -UseBasicParsing `
  -Uri https://raw.githubusercontent.com/solapi/solactl/main/scripts/install.ps1 `
  -OutFile $env:TEMP\install.ps1

# 특정 버전 설치
powershell -ExecutionPolicy Bypass -File $env:TEMP\install.ps1 -Version v0.1.6

# 설치 경로 변경
powershell -ExecutionPolicy Bypass -File $env:TEMP\install.ps1 -InstallDir D:\tools\solactl
```

> `irm | iex` 한 줄 설치는 메모리에서 실행되므로 별도의 ExecutionPolicy 설정이 필요하지 않습니다.

### 소스 빌드

```bash
git clone https://github.com/solapi/solactl.git
cd solactl
make build        # bin/solactl 생성
make install      # $GOPATH/bin에 설치
```

### 업그레이드

설치된 `solactl` 자체에서 업그레이드할 수 있습니다. 모든 플랫폼 공통입니다.

```bash
solactl upgrade
```

또는 위의 설치 스크립트를 다시 실행해도 됩니다.

### 서명 검증 (고급, 선택)

릴리스의 `checksums.txt` 는 [cosign keyless](https://docs.sigstore.dev/cosign/signing/overview/) 로 서명됩니다. 일반 설치 경로는 SHA256 체크섬만 검증하지만, 추가 보증이 필요하면 cosign 으로 서명을 검증할 수 있습니다.

검증 절차와 신뢰 모델은 [VERIFYING.md](./VERIFYING.md) 를 참고하세요.

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

# 발송 한도 조회 / 증가 요청
solactl quota get
solactl quota request --target 5000 --reason "..."
solactl quota list-requests
```

자세한 사용법은 `solactl --help` 또는 각 서브커맨드의 `--help`를 참조하세요.

## 발송 한도 요청

`solactl quota request` 로 발송 한도 증가를 요청할 수 있습니다. 요청은 SOLAPI 운영팀의 검토를 거쳐 승인 또는 반려됩니다.

### 승인을 빠르게 받는 팁

> **요청 사유에 실제 발송할 메시지 본문을 그대로 적으세요.** 검토자가 발송 의도와 컨텍스트를 즉시 확인할 수 있어 승인이 가장 빨라집니다.

`--reason` 에 다음 정보를 가능한 한 구체적으로 포함하세요.

- [ ] **수신자** — 누구에게 보내는지, 수신 동의를 어떻게 확보했는지
- [ ] **메시지 본문** — 실제로 발송할 내용 전문 또는 핵심 예시 (광고성 메시지면 광고 표기 포함 여부)
- [ ] **발송 일정 / 규모** — 캠페인 일자, 1일 / 1회 예상 발송 건수
- [ ] **비즈니스 사유** — 한도 증액이 필요한 이유 (이벤트, 정기 알림 등)

### 예시

<details>
<summary>잘 작성된 사유 (승인 빠름)</summary>

```bash
solactl quota request --target 5000 --reason "$(cat <<EOF
대상: 자사몰 회원 4,800명 (가입 시 마케팅 수신 동의 보유)
내용: '[OO몰] 5월 단독 세일 안내. 회원 한정 30% 쿠폰: <링크>'
발송 시점: 2026-05-15 14:00 일회성, 약 4,800건
사유: 정기 캠페인 발송으로 일일 한도 초과 예상
EOF
)"
```

</details>

<details>
<summary>부족한 사유 (반려되거나 추가 확인 필요)</summary>

```bash
solactl quota request --target 5000 --reason "이벤트 발송"
```

→ 수신자 / 메시지 본문 / 발송 시점이 모두 누락되어 검토자가 판단하기 어렵습니다.

</details>

### 검토 / 추적

```bash
solactl quota get                              # 현재 한도 확인
solactl quota list-requests                    # 모든 요청 이력
solactl quota list-requests --status PENDING   # 검토 대기 중인 요청만
```

상태값은 `PENDING` (검토 대기) → `APPROVED` (승인) 또는 `REJECTED` (반려) 로 변경됩니다.

> **주의** — 동일 계정에 PENDING 요청이 이미 있을 때 새 요청을 제출하면 이전 요청은 자동으로 REJECTED 처리됩니다.

