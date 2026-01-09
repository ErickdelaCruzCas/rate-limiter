# Rate Limiter Cookbook - Project Roadmap

**Last Updated:** 2026-01-09
**Current Phase:** Fase 5 COMPLETADA ✅ | **Next Phase:** Fase 6 (Load Testing en Go)

---

## 🎯 Estado General del Proyecto

| Fase | Estado | Completitud | Última Actualización |
|------|--------|-------------|---------------------|
| Fase 0 | ✅ COMPLETADA | 100% | - |
| Fase 1 | ✅ COMPLETADA | 100% | - |
| Fase 2 | ✅ COMPLETADA | 100% | 2026-01-02 |
| Fase 3 | ⏭️ SKIPPED | N/A | Benchmarks en Fase 8 |
| Fase 4 | ✅ COMPLETADA | 100% | 2026-01-02 |
| Fase 5 | ✅ COMPLETADA | 100% | 2026-01-09 |
| Fase 6 | ⏳ PENDIENTE | 0% | - |
| Fase 7 | ⏳ PENDIENTE | 0% | - |
| Fase 8 | ⏳ PENDIENTE | 0% | - |

---

## ✅ FASE 0 - README y Mentalidad (COMPLETADA)

### Objetivos
- Definir qué problema resolvemos
- Definir qué NO resolvemos
- Alinear expectativas

### Estado: COMPLETADA ✅
- README.md completo con filosofía del proyecto
- Roadmap definido
- Trade-offs documentados

---

## ✅ FASE 1 - Core Puro (sin I/O) (COMPLETADA)

### Objetivos
- Algoritmos deterministas
- Sin red, sin threads, sin clocks reales
- Reloj inyectable (Clock + ManualClock)
- Tests exhaustivos

### Algoritmos Implementados ✅
- **TokenBucket** (`core/algorithms/token_bucket/`)
  - Refill continuo dinámico
  - Burst-friendly
  - Complejidad: O(1) tiempo, O(1) memoria

- **FixedWindow** (`core/algorithms/fixed_window/`)
  - Reset por ventanas alineadas
  - Simple pero con boundary problem
  - Complejidad: O(1) tiempo, O(1) memoria

- **SlidingWindowLog** (`core/algorithms/sliding_window/`)
  - Precisión exacta con ArrayDeque
  - Sin boundary problem
  - Complejidad: O(permits) tiempo, O(limit) memoria

- **SlidingWindowCounter** (`core/algorithms/sliding_window_counter/`)
  - Ring buffer de buckets
  - Aproximación práctica
  - Complejidad: O(1) tiempo, O(buckets) memoria

### Infraestructura ✅
- Clock abstraction: `Clock` interface
- `ManualClock` para tests deterministas
- `SystemClock` para tests concurrentes (añadido en Fase 2)
- Model layer: `RateLimiter`, `Decision`, `RateLimitResult`

---

## ✅ FASE 2 - Tests como Ciudadanos de Primera (COMPLETADA)

### Objetivos Originales
- Tests unitarios por algoritmo (5 tests funcionales)
- Edge cases cubiertos
- Regresión temporal con reloj mockeado
- Configuración de JUnit 5 con Bazel

### ✨ Estado MEJORADO (2026-01-02)

**Total de tests: 48 tests ✅**

#### TokenBucket - 10 tests
**Tests Deterministas (ManualClock):**
- ✅ testAllow_whenTokensAvailable
- ✅ testReject_whenInsufficientTokens
- ✅ **testTokenRegeneration_fromRejectToAllow** (Regeneración REJECT→ALLOW)
- ✅ testBurstCapacity
- ✅ testRetryAfterCalculation
- ✅ testCapacityLimit_doesNotExceedMax

**Tests Concurrentes (SystemClock):**
- ✅ testConcurrent_multipleThreadsAcquireTokens
- ✅ testConcurrent_threadContention (20 allowed, 30 rejected)
- ✅ testConcurrent_refillUnderLoad (64/100 con reintentos)

**Edge Cases:**
- ✅ testInvalidArguments

#### FixedWindow - 12 tests
**Tests Deterministas:**
- ✅ testAllow_whenWithinLimit
- ✅ testReject_whenLimitExceeded
- ✅ **testWindowReset_fromRejectToAllow** (Reset REJECT→ALLOW)
- ✅ testPartialWindowReset
- ✅ testRetryAfterCalculation
- ✅ **testBoundaryProblem** (Demuestra boundary problem)
- ✅ testWindowAlignment

**Tests Concurrentes:**
- ✅ testConcurrent_multipleThreadsWithinLimit
- ✅ testConcurrent_threadContention (30 allowed, 20 rejected)
- ✅ testConcurrent_windowReset (88/100 con resets)

**Edge Cases:**
- ✅ testInvalidArguments

#### SlidingWindowLog - 12 tests
**Tests Deterministas:**
- ✅ testAllow_whenWithinLimit
- ✅ testReject_whenLimitExceeded
- ✅ **testSlidingWindow_fromRejectToAllow** (Eviction gradual)
- ✅ **testPreciseSlidingWindow** (Precisión exacta)
- ✅ testRetryAfterCalculation
- ✅ **testExactPrecision_noBoundaryProblem** (NO boundary problem)
- ✅ testGradualEviction
- ✅ testEmptyWindow

**Tests Concurrentes:**
- ✅ testConcurrent_multipleThreadsWithinLimit
- ✅ testConcurrent_exactLimit (30 allowed, 20 rejected exactos)
- ✅ testConcurrent_slidingEviction (43/100 con eviction)

**Edge Cases:**
- ✅ testInvalidArguments

#### SlidingWindowCounter - 14 tests
**Tests Deterministas:**
- ✅ testAllow_whenWithinLimit
- ✅ testReject_whenLimitExceeded
- ✅ **testBucketRoll_fromRejectToAllow** (Buckets REJECT→ALLOW)
- ✅ **testGradualBucketRoll** (Buckets salen gradualmente)
- ✅ testRetryAfterCalculation
- ✅ testMultipleBucketsInOneRoll
- ✅ testBucketAlignment
- ✅ **testRingBufferWrap** (Validación ring buffer)
- ✅ **testSingleBucket** (1 bucket = Fixed Window)
- ✅ **testManySmallBuckets** (100 buckets pequeños)

**Tests Concurrentes:**
- ✅ testConcurrent_multipleThreadsWithinLimit
- ✅ testConcurrent_exactLimit (30 allowed, 20 rejected exactos)
- ✅ testConcurrent_bucketRolling (52/100 con rolling)

**Edge Cases:**
- ✅ testInvalidArguments

### Mejoras Técnicas Realizadas
- ✅ Todos los algoritmos son **thread-safe** (`synchronized`)
- ✅ `SystemClock` creado para tests concurrentes reales
- ✅ Tests de concurrencia con `CountDownLatch`, `ExecutorService`
- ✅ Tests de contención (múltiples threads compitiendo)
- ✅ Tests con retry-after y reintentos
- ✅ Coverage completo de edge cases

### Comando de Verificación
```bash
bazel test //core/algorithms/...
# ✅ 4/4 test suites, 48 tests total PASSED
```

---

## ⏭️ FASE 3 - Performance Testing en Java (SKIPPED)

### Estado: SKIPPED ⏭️

**Razón del skip:**
- El objetivo principal del proyecto es profundizar en **Go**, no Java
- Los benchmarks serán más valiosos en **Fase 8** cuando se pueda comparar Java vs Go end-to-end
- Implementar el engine primero (Fase 4) permite testear performance en contexto realista
- JMH añadiría complejidad innecesaria si el foco es ir a Go rápido

### ¿Qué se hará en su lugar?

**Performance testing se mueve a Fase 8** con enfoque en:
- **Benchmarks en Go** (`testing/benchmark`) - principal foco
- Benchmarks básicos de Java para comparación (no JMH completo)
- Comparación directa Java vs Go con mismo escenario
- Load testing end-to-end con herramienta propia (Fase 6)
- Métricas más realistas en contexto de engine completo y gRPC

### Decisión tomada: 2026-01-02

Esta fase se salta para acelerar el camino hacia Go, que es el objetivo principal del aprendizaje

---

## ✅ FASE 4 - Java Engine (Concurrencia y Alto Throughput) (COMPLETADA)

### Objetivos
Wrapper thread-safe con LRU cache para almacenamiento multi-key.

### ✨ Estado COMPLETADO (2026-01-02)

**Total de tests: 23 tests ✅**

#### Componentes Implementados ✅

**Core Engine** (`java/engine/`):
- ✅ `RateLimiterEngine` - Thread-safe multi-key engine
- ✅ `LimiterEntry` - Wrapper con RateLimiter + ReentrantLock
- ✅ `LRUCache` - Custom LRU con LinkedHashMap
- ✅ `RateLimiterFactory` - Factory pattern con AlgorithmType enum
- ✅ `RateLimiterConfig` - Configuración inmutable por algoritmo
- ✅ `AlgorithmType` - Enum para los 4 algoritmos

#### Diseño del Engine ✅
- ✅ `RateLimiterEngine.java` con API `tryAcquire(String key, int permits)`
- ✅ `LRUCache<String, LimiterEntry>` para storage (no ConcurrentHashMap directo)
- ✅ LRU eviction policy con LinkedHashMap (accessOrder=true)
- ✅ Configuración de algoritmo por key vía factory pattern

#### Primitivas de Concurrencia ✅
- ✅ **ReentrantLock** seleccionado (más control que synchronized)
- ✅ Per-key locking (fine-grained, no global bottleneck)
- ✅ Synchronized LRUCache para get/put operations
- ✅ Thread-safety garantizada por locks + synchronized

**Decisión**: ReentrantLock ganó sobre StampedLock por simplicidad y debuggability.

#### Testing de Concurrencia ✅
- ✅ 11 tests funcionales (`RateLimiterEngineTest`)
  - Allow/reject, multi-key isolation, LRU eviction
  - Token refill, diferentes algoritmos, edge cases
- ✅ 7 tests de concurrencia (`RateLimiterEngineConcurrencyTest`)
  - CountDownLatch para race conditions
  - Same key contention, multi-key isolation
  - High contention (50 threads), LRU under load
  - Deadlock prevention, correctness under load (100 threads)
- ✅ 5 tests de stress (`RateLimiterEngineStressTest`)
  - Single-threaded throughput
  - Multi-threaded throughput (10 threads)
  - Latency percentiles (p50/p95/p99/max)
  - Memory pressure (50K keys → 10K eviction)
  - Sustained load con refill

#### Performance Alcanzado ✅

**Single-threaded**:
- ✅ **30.9M req/s** (target: 100K) - **309x over target**

**Multi-threaded (10 threads)**:
- ✅ **17.4M req/s** (target: 500K) - **35x over target**

**Latency (p99)**:
- ✅ **125 nanoseconds** (target: <1ms) - **8000x better than target**

**Memory**:
- ✅ LRU eviction: 80% eviction rate (50K → 10K keys)

### Documentación ✅
- ✅ `java/engine/README.md` - Comprehensive design doc
  - Architecture overview
  - Design decisions (ReentrantLock, Custom LRU, Factory)
  - Thread-safety guarantees
  - Performance characteristics
  - Usage examples

### Comando de Verificación
```bash
bazel test //java/engine/...
# ✅ 3/3 test suites, 23 tests total PASSED

bazel test //java/engine:engine_stress_test --test_output=all
# Single-threaded: 30.9M req/s
# Multi-threaded: 17.4M req/s
# Latency p99: 125 ns
```

---

## ✅ FASE 5 - gRPC API (COMPLETADA)

### Objetivos
Exponer rate limiter como servicio gRPC con alto throughput.

### ✨ Estado COMPLETADO (2026-01-09)

**Total de tests: 13 tests ✅ (7 integration + 6 stress)**

#### Contrato Proto ✅
- ✅ Definido `proto/ratelimit.proto` con service RateLimitService
- ✅ `CheckRateLimit(key, permits)` - unary RPC call
- ✅ `HealthCheck()` - health check endpoint
- ✅ Mensajes: CheckRateLimitRequest/Response, HealthCheckRequest/Response
- ✅ Nanosecond precision para retry_after_nanos

#### Codegen con Bazel ✅
- ✅ `proto_library` target (bazel build //proto:ratelimit_proto)
- ✅ `java_proto_library` target (genera mensajes Java)
- ✅ `java_grpc_library` target (genera stubs gRPC)
- ✅ Configurado grpc-java 1.68.1 + protobuf 29.2
- ✅ Proto compartible entre Java y Go (Phase 7)

#### Implementación del Servidor ✅
- ✅ `RateLimitServiceImpl` - Thin wrapper sobre RateLimiterEngine
- ✅ `RateLimitServer` - Servidor standalone en puerto 9090 (configurable)
- ✅ Graceful shutdown con timeout de 5 segundos + shutdown hook
- ✅ Error handling: INVALID_ARGUMENT para inputs inválidos, INTERNAL para errores
- ✅ Stateless design (thread-safe via engine)

#### Testing Completo ✅
- ✅ **7 Integration tests** (`RateLimitServiceImplTest`) con InProcessServer
  - testAllow_whenWithinLimit
  - testReject_whenExceedingLimit
  - testValidation_emptyKey
  - testValidation_invalidPermits
  - testHealthCheck
  - testMultiKeyIsolation
  - testTokenRefill_fromRejectToAllow
- ✅ **6 Stress tests** (`RateLimitServiceStressTest`) con SystemClock
  - testSingleThreadedThroughput
  - testMultiThreadedThroughput
  - testLatencyPercentiles
  - testHighContentionSameKey
  - testMultiKeyNoContention
  - testSustainedLoad

#### Performance Alcanzado ✅

**Con InProcessServer (sin network overhead):**

**Single-threaded**:
- ✅ **230K req/s** (target: 10K) - **23x over target**

**Multi-threaded (10 threads)**:
- ✅ **846K req/s** (target: 50K) - **17x over target**

**Latency**:
- ✅ **p50: 0.96 μs**
- ✅ **p95: 1.13 μs**
- ✅ **p99: 1.25 μs** (target: <1ms) - **800x better than target**
- ✅ **max: 57.29 μs**

**Sustained load (5 threads, 3s)**:
- ✅ **914K req/s** sustained throughput

#### Documentación ✅
- ✅ `java/grpc/README.md` - Comprehensive guide
  - Architecture diagram
  - API reference with examples
  - Configuration options
  - Testing philosophy
  - Performance results
  - Design decisions
  - Troubleshooting guide

### Comando de Verificación
```bash
# Run server
bazel run //java/grpc:server
# Output: RateLimitServer started on port: 9090

# Run integration tests
bazel test //java/grpc:grpc_service_test
# ✅ 7/7 tests PASSED (120ms)

# Run stress tests
bazel test //java/grpc:grpc_stress_test --test_output=all
# ✅ 6/6 tests PASSED (4.4s)
# Single-threaded: 230K req/s
# Multi-threaded: 846K req/s
# Latency p99: 1.25 μs

# All gRPC tests
bazel test //java/grpc/...
# ✅ 2/2 test suites, 13 tests total PASSED
```

### Resiliencia (Pendiente para futuras fases)
- [ ] Retry policy en cliente (exponential backoff)
- [ ] Circuit breaker pattern
- [ ] Deadline propagation
- [ ] Rate limiting del servidor mismo
- [ ] TLS/mTLS support
- [ ] Interceptors para logging y métricas (preparado para Phase 8)

---

## ⏳ FASE 6 - Load Testing Tool en Go (PENDIENTE)

### Objetivos
Herramienta en Go para validar rate limiters bajo carga real.

### Tareas Pendientes

#### Cliente gRPC en Go
- [ ] Cliente gRPC en Go
- [ ] Generador de tráfico configurable (RPS, duración, concurrencia)
- [ ] Distribución de keys (uniforme, zipf, hot keys)

#### Métricas
- [ ] Throughput (achieved vs target)
- [ ] Latencias: p50, p95, p99, p99.9, max
- [ ] Tasa de rechazo
- [ ] Errores de red
- [ ] Histograma de latencias

#### Escenarios
- [ ] Carga constante (sustained load)
- [ ] Spike test
- [ ] Ramp-up test

#### Output
- [ ] Reporte en consola
- [ ] JSON export
- [ ] Gráficas ASCII

### Comando Esperado
```bash
bazel run //load:traffic_generator -- --rps=10000 --duration=60s
```

---

## ⏳ FASE 7 - Implementación en Go (PENDIENTE)

### Objetivos
Rate limiter completo en Go compartiendo mismo .proto.

### Tareas Pendientes

#### Algoritmos Core en Go
- [ ] Token Bucket
- [ ] Fixed Window
- [ ] Sliding Window Log
- [ ] Sliding Window Counter
- [ ] Clock abstraction (interface Clock, MockClock)

#### Primitivas de Concurrencia Go
- [ ] Goroutines vs threads Java
- [ ] Channels vs locks
- [ ] `sync.Mutex` vs `sync.RWMutex` vs `atomic`
- [ ] `sync.Map` vs `map[string]` + mutex

#### Engine Thread-Safe
- [ ] In-memory storage
- [ ] Goroutine pool
- [ ] Context propagation

#### gRPC Server en Go
- [ ] Mismo .proto que Java
- [ ] grpc-go implementation
- [ ] Comparación Go vs Java

### Comando Esperado
```bash
bazel test //go/...
bazel run //go/server:grpc_server
```

---

## ⏳ FASE 8 - Benchmarks y Comparación (PENDIENTE)

### Objetivos
Benchmarking sistemático de todas las implementaciones.

### Tareas Pendientes

#### Benchmarks de Algoritmos
- [ ] JMH en Java (extender Fase 3)
- [ ] `testing/benchmark` en Go
- [ ] Comparación directa mismo escenario

#### Benchmarks de Engines
- [ ] Throughput bajo diferentes cargas (1K, 10K, 100K, 1M RPS)
- [ ] Latencia con diferentes niveles de concurrencia
- [ ] Memory footprint (heap, GC pressure)
- [ ] CPU utilization

#### Benchmarks End-to-End (gRPC)
- [ ] Latencia con serialización proto
- [ ] Overhead red vs in-process
- [ ] Java gRPC vs Go gRPC

#### Visualización
- [ ] Gráficas de latencia (percentiles)
- [ ] Throughput vs latencia trade-off
- [ ] Comparación side-by-side Java vs Go

### Comando Esperado
```bash
bazel run //benchmarks:compare_all
```

---

## 🎯 Próximos Pasos Recomendados

### Inmediato (Fase 6 - Load Testing en Go) ← SIGUIENTE

**Fase 5 COMPLETADA** ✅ - gRPC API funcionando con 846K req/s

1. Crear cliente gRPC en Go para generar tráfico
2. Implementar generador de tráfico configurable (RPS, duración, concurrencia)
3. Métricas: throughput, latencias (p50/p95/p99), tasa de rechazo
4. Escenarios: sustained load, spike test, ramp-up
5. Output: reporte en consola + JSON export

### Por Qué Fase 6 Ahora
- gRPC API (Fase 5) está completa y performante (846K req/s multi-threaded)
- Necesitamos validar performance con tráfico real de red (no InProcessServer)
- Go es el objetivo principal de aprendizaje - empezar con herramienta simple
- Load testing revelará cuellos de botella antes de implementar engine en Go

### Estructura de Carpetas Actual
```
rate-limiter/
├── core/              ✅ COMPLETO
│   ├── algorithms/    ✅ 4 algoritmos + 48 tests
│   ├── clock/         ✅ Clock + ManualClock + SystemClock
│   └── model/         ✅ Interfaces y tipos
├── java/              ✅ COMPLETO
│   ├── engine/        ✅ RateLimiterEngine + 23 tests (Fase 4)
│   └── grpc/          ✅ gRPC Server + 13 tests (Fase 5)
├── proto/             ✅ COMPLETO
│   └── ratelimit.proto  ✅ Shared contract (Fase 5)
├── load/              ⏳ SIGUIENTE (Fase 6)
├── go/                ⏳ PENDIENTE (Fase 7)
└── benchmarks/        ⏳ PENDIENTE (Fase 8)
```

---

## 📊 Métricas de Progreso

### Completitud General
- **Fases Completadas:** 4/8 (50%) - Fases 0, 1, 2, 4, 5 ✅ (Fase 3 skipped)
- **Tests Escritos:** 84 tests ✅ (48 core + 23 engine + 13 gRPC)
- **Test Coverage:** 100% algoritmos core + engine + gRPC
- **Thread-Safety:** ✅ Algoritmos + Engine + gRPC Service
- **Performance Engine:** ✅ 30.9M req/s direct (309x target)
- **Performance gRPC:** ✅ 846K req/s multi-threaded (17x target)
- **Documentación:** ✅ README + CLAUDE.md + java/engine/README.md + java/grpc/README.md

### Archivos Clave
- `/core/algorithms/` - 4 algoritmos implementados
- `/core/clock/` - 3 implementaciones de Clock
- `/core/model/` - Interfaces core
- `/java/engine/` - RateLimiterEngine thread-safe (Fase 4)
- `/java/grpc/` - gRPC Server + Service (Fase 5)
- `/proto/ratelimit.proto` - Shared contract Java/Go (Fase 5)
- `/java/engine/README.md` - Documentación engine
- `/java/grpc/README.md` - Documentación gRPC API
- `/.claude/PROJECT_ROADMAP.md` - Este archivo
- `/CLAUDE.md` - Guía para futuras instancias de Claude

---

## 🔗 Referencias Rápidas

### Comandos Útiles
```bash
# Tests - Core Algorithms
bazel test //core/algorithms/...

# Tests - Java Engine
bazel test //java/engine/...
bazel test //java/engine:engine_stress_test --test_output=all

# Tests - gRPC
bazel test //java/grpc/...
bazel test //java/grpc:grpc_stress_test --test_output=all

# Run gRPC Server
bazel run //java/grpc:server
bazel run //java/grpc:server -- 8080  # custom port

# Tests - Todo el proyecto
bazel test //core/... //java/...

# Build
bazel build //core/... //java/...

# Clean
bazel clean
```

### Archivos Importantes
- `MODULE.bazel` - Dependencias externas (JUnit 5)
- `.bazelrc` - Configuración Java 21
- `README.md` - Filosofía y roadmap completo
- `CLAUDE.md` - Guía para Claude Code
- `docs/design-notes.MD` - Notas de diseño core

---

**Última actualización:** 2026-01-09
**Actualizado por:** Claude Code (Fase 5 completada - gRPC API con 13 tests, 846K req/s)
