# Walton Cluster Experiment Test Scenarios

**35 highly varied test scenarios** designed for the high-capacity Walton cluster.

## Cluster Specifications

- **Node**: Single node (32 CPU cores, 242GB RAM)
- **Available**: ~26 cores, ~145GB RAM free
- **Maximum Safe Processors**: 20-25 pods
- **Current Bottleneck**: Per-pod processlist (1 job/pod)

## Test Philosophy

Unlike the original test suite, this plan **maximizes variety**:
- ✅ **No repeated configurations**
- ✅ Wide processor range: 1, 3, 5, 7, 8, 9, 10, 12, 15, 18, 20, 22, 25
- ✅ Wide dataset range: 1, 2, 3, 5, 7, 8, 10, 12, 13, 15, 18, 20
- ✅ Varied job sizes: 5, 8, 12, 17, 19, 23, 34, 51, 68
- ✅ Diverse intervals: 0s, 10s, 15s, 20s, 30s, 37s, 45s, 60s, 90s, 120s

## Test Phases (35 tests)

### Phase 1: Baseline Discovery (5 tests)
Explores fundamental scaling patterns from minimal to maximum.
- Test 1: 1 proc, 1 dataset (absolute minimum)
- Test 2: 3 proc, 2 datasets (small scale)
- Test 3: 7 proc, 5 datasets (mid scale)
- Test 4: 15 proc, 10 datasets (large scale)
- Test 5: **25 proc, 20 datasets** (maximum stress test)

### Phase 2: Processor Variance (5 tests)
Studies processor-to-dataset ratios.
- Test 6: 1:10 ratio (extreme bottleneck)
- Test 7: 5:15 ratio (under-provisioned)
- Test 8: 12:12 ratio (1:1 balanced)
- Test 9: 18:8 ratio (over-provisioned)
- Test 10: 22:3 ratio (extreme over-provision)

### Phase 3: Dataset Variance (4 tests)
Explores dataset count effects.
- Test 11: 20 proc, 1 dataset (single dataset stress)
- Test 12: 15 proc, 3 datasets (few datasets)
- Test 13: 8 proc, 18 datasets (many datasets)
- Test 14: 12 proc, 20 datasets (maximum datasets)

### Phase 4: Job Size Variance (5 tests)
Studies job size impact on throughput.
- Test 15-19: 5, 8, 17, 34, 68 files/job

### Phase 5: Start Interval Impact (5 tests)
Examines staggering effects.
- Test 20-24: 0s, 10s, 30s, 60s, 120s intervals

### Phase 6: Mixed Stress Tests (4 tests)
Complex scenarios combining multiple variables.
- Test 25: Small simultaneous burst (7 proc, 8 datasets, 0s)
- Test 26: Large simultaneous burst (18 proc, 20 datasets, 0s)
- Test 27: Severe imbalance (3 proc, 15 datasets)
- Test 28: High throughput (22 proc, 12 datasets, large jobs)

### Phase 7: Optimal Configs (3 tests)
Production-ready scenarios.
- Test 29: Light load (10 proc, 5 datasets)
- Test 30: Medium load (15 proc, 10 datasets)
- Test 31: Heavy load (20 proc, 15 datasets)

### Phase 8: Edge Cases (4 tests)
Extreme scenarios and unusual configurations.
- Test 32: Maximum job count (5 proc, 20 datasets, tiny jobs = 544 jobs)
- Test 33: Minimum job count (25 proc, 10 datasets, huge jobs = 20 jobs)
- Test 34: Odd ratios (9 proc, 13 datasets, 19 files/job)
- Test 35: Maximum processors (25 proc, 8 datasets)

## Key Insights

**Processor Bottleneck:**
- Per-pod processlist limits to 1 job/pod
- 10 processors = max 10 concurrent jobs (despite 210 in queue)
- Scaling to 20-25 processors will 2-2.5x throughput

**Producer Performance:**
- Producer batched 7 datasets in 3 minutes (210 jobs) ✅
- Producer is NOT the bottleneck
- Can handle aggressive dataset staggering

**Optimal Configuration (from early testing):**
- 15-20 processors for production workloads
- 30-60s dataset intervals for stability
- Medium job sizes (17-34 files) for balanced throughput

## Running Tests

From inside the experiment-tool pod:
```bash
oc exec -it $(oc get pod -l app=experiment-tool -n uc3-applications -o jsonpath='{.items[0].metadata.name}') -n uc3-applications -- /bin/sh
```

Individual test:
```bash
./uc3-experiment start --config /app/configs/scenarios-w/phase1-baseline/test01-minimal-1proc-1dataset.yaml
```

All tests:
```bash
# Copy from /Users/bcapper/Documents/ucm_app/documents/walton/run_all_tests_walton.sh
```

## Downloading Results

From your local machine:
```bash
./download_results.sh walton-test01-minimal
```

## Safety Notes

- ⚠️ Tests 5, 26, 33, 35 use **22-25 processors** (near cluster max)
- ⚠️ Tests 20, 25, 26 use **0s interval** (simultaneous burst)
- ✅ Most tests use safe ranges (5-18 processors, 30-60s intervals)
