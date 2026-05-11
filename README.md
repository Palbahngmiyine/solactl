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

# 발송 내역 export (CSV/JSON/JSONL)
solactl messages export --output messages.csv

# 일별 통계 export (CSV/JSON/JSONL)
solactl statistics export-daily --output stats.csv \
  --start-date 2026-05-04 --end-date 2026-05-11

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

## 발송 내역 / 통계 Export

대량 export는 messages-v4 부하를 줄이기 위해 다음 가드를 자동 적용합니다.

- **6개월(180일) 이전 데이터는 조회 불가** — 사내 DB에서 자동 삭제됩니다.
- **7일 초과 범위는 1일 단위 윈도우로 자동 분할** — UTC 자정 기준으로 잘게 호출.
- **`--throttle` (기본 500ms)** — 페이지/윈도우 호출 사이 sleep. 최소 100ms 강제.
- **`--page-size` 강제 상한** — `messages export` 200, `statistics export-daily` 100.
- **Ctrl+C 시 부분 결과 보존** — stderr에 `--resume-token` 안내. 다음 명령에 `--append --resume-token <토큰>`을 붙여 이어받을 수 있습니다.

### 메시지 내역 export

```bash
# 기본: 최근 7일, CSV, page-size 50, throttle 500ms
solactl messages export --output messages.csv

# 31일 범위 — 자동으로 31개 1일 윈도우로 분할
solactl messages export --output messages.csv \
  --start-date 2026-04-02 --end-date 2026-05-03

# JSONL 포맷 + 필터
solactl messages export --output messages.jsonl --format jsonl \
  --type SMS --status-code 4000 --from 029302266

# 중단 후 재개
solactl messages export --output messages.csv --append \
  --resume-token eyJ2IjoxLCJ3IjoiMjAyNi0wNS0wMSJ9
```

CSV 컬럼: `messageId, type, status, statusCode, to, from, country, subject, dateCreated, dateUpdated, groupId, accountId, text, customFields`.

### 일별 통계 export

```bash
# 7일 범위
solactl statistics export-daily --output stats.csv \
  --start-date 2026-05-04 --end-date 2026-05-11

# 31일 범위 — 자동 일별 분할
solactl statistics export-daily --output stats.csv \
  --start-date 2026-04-02 --end-date 2026-05-03

# Windows Excel 한글 호환 (UTF-8 BOM)
solactl statistics export-daily --output stats.csv \
  --start-date 2026-05-04 --end-date 2026-05-11 --bom
```

CSV는 고정 prefix (`date, accountId, prepaid, balance, point, profit, refundBalance, refundPoint`) + 응답에서 발견된 모든 `count.*` 키를 정렬해 컬럼화 (`count_SMS, count_LMS, count_MMS, ...`).

