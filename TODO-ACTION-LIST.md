# 회사에서 해야할 액션 리스트

## 1단계: Kind 로컬 테스트 (회사 로컬PC)

### 사전 확인
- [ ] Kind 클러스터가 실행 중인지 확인
  ```bash
  kind get clusters
  kubectl cluster-info
  ```
- [ ] Docker가 정상 동작하는지 확인
  ```bash
  docker ps
  ```

### 배포 및 테스트
- [ ] 프로젝트 클론 및 이동
  ```bash
  git clone git@github.com:asjgi/n8n-workflow-otelcol.git
  cd n8n-workflow-otelcol
  ```

- [ ] Kind 환경에 배포
  ```bash
  ./scripts/deploy-kind.sh
  ```

- [ ] 배포 상태 확인 및 테스트
  ```bash
  ./scripts/test-kind.sh
  ```

- [ ] 서비스 접근 테스트
  - [ ] OTEL Automation API: http://localhost:30080/api/v1/health
  - [ ] n8n 워크플로우: http://localhost:30567
  - [ ] Loki: http://localhost:31000/ready

### 기능 테스트
- [ ] API 직접 테스트
  ```bash
  # Health check
  curl http://localhost:30080/api/v1/health

  # 서비스 파이프라인 추가 테스트
  curl -X POST http://localhost:30080/api/v1/otel/pipeline/add \
    -H "Content-Type: application/json" \
    -d '{
      "service_name": "test-api",
      "namespace": "default"
    }'
  ```

- [ ] ConfigMap 변경 확인
  ```bash
  kubectl get configmap otel-collector-config -n observability -o yaml
  ```

- [ ] 로그 확인
  ```bash
  kubectl logs -f deployment/otel-pipeline-automation -n otel-automation
  ```

### 문제 해결 (필요시)
- [ ] 실패 시 로그 확인
  ```bash
  kubectl get events --sort-by=.metadata.creationTimestamp
  kubectl describe pod -n otel-automation
  ```

- [ ] 재배포 (필요시)
  ```bash
  ./scripts/cleanup-kind.sh
  ./scripts/deploy-kind.sh
  ```

## 2단계: EKS 환경 준비

### AWS 환경 설정
- [ ] AWS CLI 및 kubectl EKS 클러스터 접근 설정
  ```bash
  aws eks update-kubeconfig --region <region> --name <cluster-name>
  kubectl get nodes
  ```

### ECR 이미지 준비
- [ ] ECR 리포지토리 생성 (없으면)
  ```bash
  aws ecr create-repository --repository-name otel-pipeline-automation
  ```

- [ ] Docker 이미지 빌드 및 푸시
  ```bash
  # ECR 로그인
  aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <account>.dkr.ecr.<region>.amazonaws.com

  # 이미지 빌드 및 태그
  docker build -t otel-pipeline-automation .
  docker tag otel-pipeline-automation:latest <account>.dkr.ecr.<region>.amazonaws.com/otel-pipeline-automation:latest

  # 이미지 푸시
  docker push <account>.dkr.ecr.<region>.amazonaws.com/otel-pipeline-automation:latest
  ```

### EKS용 매니페스트 수정
- [ ] `deployments/kubernetes.yaml` 파일에서 이미지 URL 수정
  ```yaml
  image: <account>.dkr.ecr.<region>.amazonaws.com/otel-pipeline-automation:latest
  ```

- [ ] 환경 변수 수정 (필요시)
  - [ ] `N8N_WEBHOOK_URL` - 실제 n8n 서비스 URL로 변경
  - [ ] `LOKI_ENDPOINT` - 실제 Loki 엔드포인트로 변경
  - [ ] `CLUSTER_NAME` - EKS 클러스터명으로 변경

## 3단계: EKS 배포

### 네임스페이스 및 RBAC 설정
- [ ] 필요한 네임스페이스 생성 (없으면)
  ```bash
  kubectl create namespace observability
  kubectl create namespace otel-automation
  ```

### 서비스 배포
- [ ] Kubernetes 매니페스트 적용
  ```bash
  kubectl apply -f deployments/kubernetes.yaml
  ```

- [ ] 배포 상태 확인
  ```bash
  kubectl get pods -n otel-automation
  kubectl get services -n otel-automation
  ```

### 기존 OTEL Collector와 연동
- [ ] 기존 OTEL Collector ConfigMap 확인
  ```bash
  kubectl get configmap -n observability
  kubectl get configmap otel-collector-config -n observability -o yaml
  ```

- [ ] OTEL Collector DaemonSet 확인
  ```bash
  kubectl get daemonset -n observability
  ```

## 4단계: 통합 테스트

### API 접근 테스트
- [ ] 서비스 엔드포인트 확인
  ```bash
  kubectl get service otel-pipeline-automation -n otel-automation
  ```

- [ ] LoadBalancer/Ingress를 통한 외부 접근 설정 (필요시)

### 전체 파이프라인 테스트
- [ ] Port IDP에서 실제 요청 테스트
- [ ] n8n 워크플로우 동작 확인
- [ ] OTEL Collector ConfigMap 자동 업데이트 확인
- [ ] 실제 로그가 Loki로 수집되는지 확인

### 모니터링 설정
- [ ] 서비스 로그 확인
  ```bash
  kubectl logs -f deployment/otel-pipeline-automation -n otel-automation
  ```

- [ ] Prometheus 메트릭 확인 (있으면)
- [ ] Grafana 대시보드 설정 (있으면)

## 5단계: 운영 준비

### 문서화
- [ ] 운영 가이드 작성
- [ ] 트러블슈팅 가이드 작성
- [ ] API 문서 업데이트

### 알림 설정
- [ ] 서비스 장애 알림 설정
- [ ] 로그 에러 알림 설정

### 백업 및 복구
- [ ] ConfigMap 백업 전략 수립
- [ ] 재해복구 계획 수립

## 예상 이슈 및 해결방법

### Kind에서 자주 발생하는 문제
- **이미지 pull 실패**: `imagePullPolicy: Never`로 설정됨, `kind load docker-image` 실행했는지 확인
- **서비스 접근 불가**: NodePort가 올바르게 설정되었는지 확인
- **리소스 부족**: 리소스 limit을 낮춰서 설정함

### EKS에서 자주 발생하는 문제
- **RBAC 권한 부족**: ServiceAccount에 올바른 ClusterRole이 바인딩되어 있는지 확인
- **이미지 pull 실패**: ECR 권한 및 이미지 URL 확인
- **네트워크 접근 불가**: 보안 그룹 및 NLB 설정 확인

### 일반적인 해결 명령어
```bash
# Pod 상태 확인
kubectl get pods --all-namespaces

# 이벤트 확인
kubectl get events --sort-by=.metadata.creationTimestamp

# 로그 확인
kubectl logs -f <pod-name> -n <namespace>

# Pod 상세 정보
kubectl describe pod <pod-name> -n <namespace>

# 서비스 확인
kubectl get services --all-namespaces
```

## 완료 체크리스트
- [ ] Kind 로컬 테스트 완료
- [ ] EKS 배포 완료
- [ ] 전체 파이프라인 동작 확인
- [ ] 운영 문서 작성 완료
- [ ] 모니터링 설정 완료