# Rate Limiter Cookbook - Project Roadmap

**Last Updated:** 2026-01-02
**Current Phase:** Fase 2 COMPLETADA ✅ | **Next Phase:** Fase 3 (Performance Testing)

---

## 🎯 Estado General del Proyecto

| Fase | Estado | Completitud | Última Actualización |
|------|--------|-------------|---------------------|
| Fase 0 | ✅ COMPLETADA | 100% | - |
| Fase 1 | ✅ COMPLETADA | 100% | - |
| Fase 2 | ✅ COMPLETADA | 100% | 2026-01-02 |
| Fase 3 | ⏭️ SKIPPED | N/A | Benchmarks en Fase 8 |
| Fase 4 | ⏳ PENDIENTE | 0% | - |
| Fase 5 | ⏳ PENDIENTE | 0% | - |
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

## ⏳ FASE 4 - Java Engine (Concurrencia y Alto Throughput) (PENDIENTE)

### Objetivos
Wrapper thread-safe con `ConcurrentHashMap` para almacenamiento multi-key.

### Tareas Pendientes

#### Diseño del Engine
- [ ] Crear `java/engine/RateLimiterEngine.java`
- [ ] API: `boolean tryAcquire(String key, int permits)`
- [ ] `ConcurrentHashMap<String, RateLimiter>` para storage
- [ ] Eviction policy (LRU simple) para limitar memoria
- [ ] Configuración de algoritmo por key

#### Primitivas de Concurrencia
- [ ] Evaluar `ReentrantLock` vs `synchronized`
- [ ] Usar `AtomicLong` para contadores lock-free
- [ ] Explorar `StampedLock` para optimistic reads

#### Testing de Concurrencia
- [ ] Tests con `CountDownLatch` para race conditions
- [ ] Validación thread-safety con múltiples threads
- [ ] Tests de stress con carga sostenida

#### Targets de Performance
- [ ] > 100K requests/sec (single-threaded)
- [ ] > 500K requests/sec (multi-threaded)
- [ ] Latencia p99 < 1ms para hot keys

### Comando Esperado
```bash
bazel test //java/engine:engine_test
bazel run //java/engine:stress_test
```

---

## ⏳ FASE 5 - gRPC API (PENDIENTE)

### Objetivos
Exponer rate limiter como servicio gRPC con alto throughput.

### Tareas Pendientes

#### Contrato Proto
- [ ] Definir `proto/ratelimit.proto`
- [ ] `CheckRateLimit(key, permits)` - unary call
- [ ] `CheckRateLimitBatch(requests[])` - batch
- [ ] `ResetRateLimit(key)` - admin operation
- [ ] Health check endpoint

#### Codegen con Bazel
- [ ] `proto_library` target
- [ ] `java_proto_library` target
- [ ] `java_grpc_library` target
- [ ] Compartir .proto entre Java y Go

#### Implementación del Servidor
- [ ] gRPC Java con servidor asíncrono
- [ ] Thread pool tuning
- [ ] Error handling (InvalidArgument, ResourceExhausted, Unavailable)
- [ ] Interceptors para logging y métricas
- [ ] Graceful shutdown

#### Resiliencia
- [ ] Retry policy en cliente (exponential backoff)
- [ ] Circuit breaker
- [ ] Deadline propagation
- [ ] Rate limiting del servidor

### Comando Esperado
```bash
bazel run //java/grpc:server
bazel test //java/grpc:server_test
```

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

### Inmediato (Fase 4 - Java Engine) ← SIGUIENTE

**Fase 3 SKIPPED** - Ir directo a engine

1. Crear estructura `java/engine/`
2. Implementar `RateLimiterEngine` con `ConcurrentHashMap`
3. Decidir: `ReentrantLock` vs `synchronized` por key
4. Implementar LRU simple para eviction
5. Tests de concurrencia con `CountDownLatch`

### Por Qué Fase 4 Directamente
- Tener sistema funcional end-to-end permite explorar más rápido
- Engine es necesario para Fase 5 (gRPC)
- Benchmarks (Fase 8) serán más valiosos con implementaciones completas Java + Go
- Saltar Fase 3 acelera camino hacia Go (objetivo principal)

### Estructura de Carpetas Esperada
```
rate-limiter/
├── core/              ✅ COMPLETO
│   ├── algorithms/    ✅ 4 algoritmos + 48 tests
│   ├── clock/         ✅ Clock + ManualClock + SystemClock
│   └── model/         ✅ Interfaces y tipos
├── java/              ⏳ PENDIENTE
│   ├── benchmarks/    <- SIGUIENTE (Fase 3)
│   ├── engine/        <- Fase 4
│   └── grpc/          <- Fase 5
├── go/                ⏳ PENDIENTE (Fase 7)
├── load/              ⏳ PENDIENTE (Fase 6)
└── proto/             ⏳ PENDIENTE (Fase 5)
```

---

## 📊 Métricas de Progreso

### Completitud General
- **Fases Completadas:** 2/8 (25%)
- **Tests Escritos:** 48 tests ✅
- **Test Coverage:** 100% de algoritmos core
- **Thread-Safety:** ✅ Todos los algoritmos
- **Documentación:** ✅ README + CLAUDE.md

### Archivos Clave
- `/core/algorithms/` - 4 algoritmos implementados
- `/core/clock/` - 3 implementaciones de Clock
- `/core/model/` - Interfaces core
- `/.claude/PROJECT_ROADMAP.md` - Este archivo
- `/CLAUDE.md` - Guía para futuras instancias de Claude

---

## 🔗 Referencias Rápidas

### Comandos Útiles
```bash
# Tests
bazel test //core/algorithms/...
bazel test //core/algorithms/token_bucket:token_bucket_test --test_output=all

# Build
bazel build //core/...

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

**Última actualización:** 2026-01-02
**Actualizado por:** Claude Code (Fase 2 completada con 48 tests)
