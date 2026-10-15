# Changelog

## [0.6.0](https://github.com/WindKube/aws-metrics-exporter/compare/v0.5.0...v0.6.0) (2026-10-15)


### Features

* add support for scraping billing data ([8c0ab0f](https://github.com/WindKube/aws-metrics-exporter/commit/8c0ab0ff52cf2d05187769d36db76629d3aeb8a4))
* add support for scraping billing data ([3f5224c](https://github.com/WindKube/aws-metrics-exporter/commit/3f5224cd090533a7a9a4674bdf0ce4ada6e34dc9))
* add support for scraping ecs data ([bea5e6d](https://github.com/WindKube/aws-metrics-exporter/commit/bea5e6deca1517746b7c363cbe2c8c1ab871cb59))
* add support for scraping ecs data ([e9eb2a7](https://github.com/WindKube/aws-metrics-exporter/commit/e9eb2a78ae6972e00db60b646625b4af61234686))


### Bug Fixes

* update dependencies ([00bc7b9](https://github.com/WindKube/aws-metrics-exporter/commit/00bc7b98f24beb2eafbda985100f34d09c7ac0da))

## [0.5.0](https://github.com/WindKube/aws-metrics-exporter/compare/v0.4.0...v0.5.0) (2026-09-24)


### Features

* scrape RDS instances and clusters ([768d4ce](https://github.com/WindKube/aws-metrics-exporter/commit/768d4ce6b382c3272ecee924d4a60fd690c5f356))
* scrape RDS instances and clusters ([94a3629](https://github.com/WindKube/aws-metrics-exporter/commit/94a36298a8ed51000776508539782e1302e82962))

## [0.4.0](https://github.com/WindKube/aws-metrics-exporter/compare/v0.3.1...v0.4.0) (2026-09-21)


### Features

* scrape ElastiCache cache clusters ([1319d03](https://github.com/WindKube/aws-metrics-exporter/commit/1319d036abd9bf6950f34ed9551864c07089af3c))
* scrape ElastiCache cache clusters ([cd6ce94](https://github.com/WindKube/aws-metrics-exporter/commit/cd6ce94ca8e1c9a0a8cb17ec21cefeb3ac0e9a2d))

## [0.3.1](https://github.com/WindKube/aws-metrics-exporter/compare/v0.3.0...v0.3.1) (2026-09-21)


### Bug Fixes

* probe the index pattern instead of the cluster root ([9dbf03a](https://github.com/WindKube/aws-metrics-exporter/commit/9dbf03ad421cd51efd947444f730e466d3cbf768))
* probe the index pattern instead of the cluster root ([43031d9](https://github.com/WindKube/aws-metrics-exporter/commit/43031d98a751847d302290963a92feac9d0c9466))

## [0.3.0](https://github.com/WindKube/aws-metrics-exporter/compare/v0.2.0...v0.3.0) (2026-09-21)


### ⚠ BREAKING CHANGES

* storage.elasticsearch.manage_index_template is gone and the exporter no longer creates the index template. Create it before the first scrape, see deploy/elasticsearch/README.md.

### Features

* manage the Elasticsearch index template outside the application ([acd9dfb](https://github.com/WindKube/aws-metrics-exporter/commit/acd9dfb6ff9cdd2424d859a406c9a45b11a6d14b))

## [0.2.0](https://github.com/WindKube/aws-metrics-exporter/compare/v0.1.2...v0.2.0) (2026-09-21)


### Features

* connect to Temporal over TLS ([3904ef2](https://github.com/WindKube/aws-metrics-exporter/commit/3904ef2281a716e5e33f5d933afd40393f782bd4))
* connect to Temporal over TLS ([754cd6e](https://github.com/WindKube/aws-metrics-exporter/commit/754cd6e2c746034d5243938eed0af9e2b2f900b3))

## [0.1.2](https://github.com/WindKube/aws-metrics-exporter/compare/v0.1.1...v0.1.2) (2026-09-19)


### Bug Fixes

* derive the manifest list digest from its raw bytes ([12a507e](https://github.com/WindKube/aws-metrics-exporter/commit/12a507ea12a6b17db91625a6843c4593b320f679))
* derive the manifest list digest from its raw bytes ([b3f6f3b](https://github.com/WindKube/aws-metrics-exporter/commit/b3f6f3b2243ffbff445c4936b5a7e0c7df32f044))

## [0.1.1](https://github.com/WindKube/aws-metrics-exporter/compare/v0.1.0...v0.1.1) (2026-09-19)


### Bug Fixes

* scan each release image on its own platform ([8657ba5](https://github.com/WindKube/aws-metrics-exporter/commit/8657ba526edfe74d0bec9e6780b6fc79e887ceed))
* scan each release image on its own platform ([3d0237a](https://github.com/WindKube/aws-metrics-exporter/commit/3d0237a93a2725e95b182e5222ef5e1f5b282410))

## 0.1.0 (2026-09-19)


### Bug Fixes

* pin go version ([ffb0efb](https://github.com/WindKube/aws-metrics-exporter/commit/ffb0efb380259e5c58c87edbd0225adaaf558b94))
