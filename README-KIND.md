# Kind 로컬 개발 가이드

이 가이드는 Kind(Kubernetes in Docker)를 사용하여 OTEL Pipeline Automation을 로컬에서 테스트하는 방법을 설명합니다.

## 사전 준비사항

- Docker
- Kind
- kubectl
- 기존 Kind 클러스터

## 빠른 시작

### 1. 배포

```bash
# 전체 스택 배포
./scripts/deploy-kind.sh
```

### 2. 테스트

```bash
# 배포 상태 및 기능 테스트
./scripts/test-kind.sh
```

### 3. 정리

```bash
# 리소스 정리
./scripts/cleanup-kind.sh
```

## 서비스 접근

배포 완료 후 다음 URL로 서비스에 접근할 수 있습니다:

- **OTEL Automation API**: http://localhost:30080
- **n8n 워크플로우**: http://localhost:30567
- **Loki**: http://localhost:31000

## API 테스트

### Health Check

```bash
curl http://localhost:30080/api/v1/health
```

### 서비스 파이프라인 추가

```bash
curl -X POST http://localhost:30080/api/v1/otel/pipeline/add \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "my-service",
    "namespace": "default",
    "log_path": "/var/log/pods/default_my-service_*/*.log"
  }'
```

### 서비스 상태 조회

```bash
curl http://localhost:30080/api/v1/otel/status/my-service/default
```

## 로그 확인

### Automation Service 로그

```bash
kubectl logs -f deployment/otel-pipeline-automation -n otel-automation
```

### OTEL Collector 로그

```bash
kubectl logs -f daemonset/otel-collector -n observability
```

### 모든 Pod 상태

```bash
kubectl get pods --all-namespaces
```

## 구성 확인

### ConfigMap 확인

```bash
kubectl get configmap otel-collector-config -n observability -o yaml
```

### 서비스 정보

```bash
kubectl get services --all-namespaces
```

## 디버깅

### 이벤트 확인

```bash
kubectl get events --sort-by=.metadata.creationTimestamp
```

### 특정 Pod 문제 해결

```bash
kubectl describe pod <pod-name> -n <namespace>
```

### 네트워크 연결 테스트

```bash
# Pod 내부에서 다른 서비스 연결 테스트
kubectl exec -it <pod-name> -n <namespace> -- curl http://service-name:port
```

## 주요 차이점 (프로덕션 vs Kind)

| 항목 | 프로덕션 | Kind |
|------|---------|------|
| 이미지 정책 | Always | Never |
| 리소스 제한 | 높음 | 낮음 |
| 복제본 수 | 2+ | 1 |
| 서비스 타입 | ClusterIP | NodePort |
| 스토리지 | 영구 볼륨 | 임시 |

## EKS 배포 준비

Kind에서 테스트가 완료되면 다음 단계로 EKS에 배포할 수 있습니다:

1. 이미지를 ECR에 푸시
2. 프로덕션 매니페스트 업데이트
3. Helm 차트 준비 (선택사항)
4. AWS Load Balancer Controller 설정

## 트러블슈팅

### 일반적인 문제

1. **이미지 Pull 오류**
   ```bash
   # 이미지가 Kind 클러스터에 로드되었는지 확인
   docker exec -it kind-control-plane crictl images | grep otel-pipeline-automation
   ```

2. **서비스 접근 불가**
   ```bash
   # NodePort 서비스 확인
   kubectl get services --all-namespaces | grep NodePort
   ```

3. **Pod 시작 실패**
   ```bash
   # Pod 로그 확인
   kubectl logs <pod-name> -n <namespace>
   # Pod 상세 정보 확인
   kubectl describe pod <pod-name> -n <namespace>
   ```

### 완전 재시작

문제가 계속 발생하면 완전히 재시작:

```bash
./scripts/cleanup-kind.sh
./scripts/deploy-kind.sh
```