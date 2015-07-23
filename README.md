deploy-note
===

## Usage

```
go get github.com/heshed/deploy-note

ORGS=heshed REPOS=milestones-test:milestones-test MILESTONE=1.0.0 deploy-note

# 또는
ORGS=heshed
REPOS=milestones-test:milestones-test
MILESTONE=1.0.0
deploy-note
```

## Result

```
배포 시간

milestones-test:2015-07-23 10:00
milestones-test:2015-07-23 10:00


배포 내역

- [bug duplicate enhancement] [milestones-test #3 / closed issue](https://github.com/heshed/milestones-test/issues/3)
- [bug Label-test] [milestones-test #2 / issue 2](https://github.com/heshed/milestones-test/issues/2)
- [enhancement Label-test] [milestones-test #1 / issue test](https://github.com/heshed/milestones-test/issues/1)
- [bug duplicate enhancement] [milestones-test #3 / closed issue](https://github.com/heshed/milestones-test/issues/3)
- [bug Label-test] [milestones-test #2 / issue 2](https://github.com/heshed/milestones-test/issues/2)
- [enhancement Label-test] [milestones-test #1 / issue test](https://github.com/heshed/milestones-test/issues/1)


배포 버전

milestones-test:1.0.0
milestones-test:1.0.0


참조

 @aaa.bbb @bbb.bbb, @ccc.ccc, @ddd.ddd

배포 공지

통검프론트 :
통검 공통 템플릿 :
```

## Known Bugs

- `<nil>` 이 프린트되는 원인을 아직 못찾았다.
- 멘션자들이 합산되지 않는다..
