# Changelog

## [0.1.6](https://github.com/solapi/solactl/compare/v0.1.5...v0.1.6) (2026-05-06)


### Features

* add quota commands for sending limit requests ([e0bdadb](https://github.com/solapi/solactl/commit/e0bdadbe105c8cef4f931545018f84279d81cf98))
* **quota:** add quota commands for sending limit requests ([37bef04](https://github.com/solapi/solactl/commit/37bef041be8f0b1f4a0c4afa1b70558d0203974c))
* **quota:** add quota commands for sending limit requests ([ba413ea](https://github.com/solapi/solactl/commit/ba413ea03b64e0df04fba9b56c4b81c17e91b0ae))


### Bug Fixes

* **quota:** send trimmed reason and normalize multi-line list display ([d7a9711](https://github.com/solapi/solactl/commit/d7a9711244bf39f382717a607e3e6d9da2024dfb))
* **quota:** send trimmed reason and normalize multi-line list display ([24a61d4](https://github.com/solapi/solactl/commit/24a61d4cda195b0195fa17328255a45479d3ea71))
* **quota:** trim whitespace before measuring --reason length ([ec33a19](https://github.com/solapi/solactl/commit/ec33a19029bfeb230d06d143e32c4bb1174af62b))

## [0.1.5](https://github.com/solapi/solactl/compare/v0.1.4...v0.1.5) (2026-04-16)


### Bug Fixes

* align senderid list and resolveFrom with actual API response schema ([5990274](https://github.com/solapi/solactl/commit/599027469279aa1d06cc9c9b46363bcd3cd49b58))
* align senderid list and resolveFrom with actual API response schema ([cf0d40d](https://github.com/solapi/solactl/commit/cf0d40da0425d7fd751754d90decd772f0e5b90f))
* filter inactive senders in JSON mode and harden tests ([416fcb1](https://github.com/solapi/solactl/commit/416fcb1f03cedc8a138bccfbe5666ca3ac68062c))
* preserve raw JSON passthrough for --all --json mode ([0d112a6](https://github.com/solapi/solactl/commit/0d112a6095b75d381d9a88aca71aa18c92d1a793))

## [0.1.4](https://github.com/solapi/solactl/compare/v0.1.3...v0.1.4) (2026-04-16)


### Bug Fixes

* add checksum verification, URL validation, and debug log redaction ([ee89639](https://github.com/solapi/solactl/commit/ee89639fc67be559237cb0c038611b41f0ee8c50))
* add release-assets.githubusercontent.com to trusted hosts ([741e26a](https://github.com/solapi/solactl/commit/741e26a7b6f4bec1438326c29a14e946be7733b9))
* add supply-chain integrity and debug log hardening ([f491c3d](https://github.com/solapi/solactl/commit/f491c3d234e6a0b27b7de0721018d6163bb9ede5))
* address review issues from pr-review-toolkit and codex ([22e7699](https://github.com/solapi/solactl/commit/22e7699fb211d588379a2af0930e080e42ac49af))
* address round-2 codex review issues ([da54cf6](https://github.com/solapi/solactl/commit/da54cf64e9434ae45c1077db7d5f163955603046))
* handle top-level JSON array in debug log redaction ([7940ce0](https://github.com/solapi/solactl/commit/7940ce0a581dbbfdafc25894f3f66042c2fb1da4))

## [0.1.3](https://github.com/solapi/solactl/compare/v0.1.2...v0.1.3) (2026-04-15)


### Features

* add multi-profile credential support ([e11b0cc](https://github.com/solapi/solactl/commit/e11b0cccc3a854821ff4be8b7d5c6b4c5adcc442))
* add multi-profile credential support ([87648d3](https://github.com/solapi/solactl/commit/87648d32879017b42eecfdd48a975e77a6866638))


### Bug Fixes

* add nil profile guard to SetActiveProfile and DeleteProfile ([c4ab908](https://github.com/solapi/solactl/commit/c4ab9083cdead9e42ae25a35aa7a6e59addeb1ae))
* address review issues from pr-review-toolkit and codex ([c70a4e6](https://github.com/solapi/solactl/commit/c70a4e6b8f324b78013600b14206f6a8b15d1e08))
* address round-2 codex review issues ([07b0621](https://github.com/solapi/solactl/commit/07b0621ff70d49935e343b5c735a1d141c7e2cd4))
* warn on missing profile and document concurrent write limitation ([70a4d7c](https://github.com/solapi/solactl/commit/70a4d7c226b4ca1afb6d081a97030da01db8774f))

## [0.1.2](https://github.com/solapi/solactl/compare/v0.1.1...v0.1.2) (2026-04-15)


### Features

* add kakao channel, template, and brand template management ([d1164e0](https://github.com/solapi/solactl/commit/d1164e000595d3a0739080a59c89de15e43c598f))
* add kakao channel, template, and brand template management commands ([9b880d5](https://github.com/solapi/solactl/commit/9b880d5065d7e58f40c101bec7b0a0302fe8bad6))


### Bug Fixes

* address review issues from pr-review-toolkit and codex ([c2ec95f](https://github.com/solapi/solactl/commit/c2ec95f9b0b3583d6d7aea6e91dbba94b0ac66e5))
* unify interface{} to any, reset pflag Changed state in tests ([ee0e9ab](https://github.com/solapi/solactl/commit/ee0e9ab400487f908cc734dc674b532b400723ea))

## [0.1.1](https://github.com/solapi/solactl/compare/v0.1.0...v0.1.1) (2026-04-08)


### Features

* Phase 1-2 구현 — 기반 구조, 발신번호, SMS/LMS/MMS 발송 ([0a5cd70](https://github.com/solapi/solactl/commit/0a5cd700e9e7c78a429d6c6234db3f455a6bab05))
* Phase 3-5 구현 — 카카오 ATA/BMS, RCS, Self-Upgrade, 잔액/발송내역 조회 ([feb480d](https://github.com/solapi/solactl/commit/feb480d8d7189ff4338e9ec4243874b531f6f7fe))
* Phase 3-5 구현 — 카카오/RCS/Upgrade/조회 ([cefb667](https://github.com/solapi/solactl/commit/cefb6670ab07c11b3facb704346e1baeebee7392))
* Phase 6 — 클라이언트 사이드 Validation, --from 자동선택, 빌드 인프라 ([a60fb88](https://github.com/solapi/solactl/commit/a60fb880e32eae6991ff7cdf0891ecedcc435304))
* Phase 6 구현 — 클라이언트 사이드 Validation, --from 자동선택, 빌드 인프라 ([39c0dbd](https://github.com/solapi/solactl/commit/39c0dbdf0ef42b8354d6e499bba1cd38e33369d8))


### Bug Fixes

* 6회 반복 코드 리뷰 기반 테스트 강화 및 버그 수정 ([579c3c0](https://github.com/solapi/solactl/commit/579c3c0638570985b27ce4aa3748553bb9f186f6))
* Gemini PR [#2](https://github.com/solapi/solactl/issues/2) 리뷰 기반 버그 수정 및 코드 품질 개선 ([32dced7](https://github.com/solapi/solactl/commit/32dced7d6ebf1ba8538f181b8f7b860d10f32ac6))
