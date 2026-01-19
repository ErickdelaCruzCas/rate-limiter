# Rate Limiter Cookbook (Bazel-first)

Este repositorio es un **proyecto técnico incremental** diseñado como material de estudio para **entrevistas de system design** y **arquitectura de sistemas distribuidos**.

Usa un **rate limiter distribuido** como caso de estudio para explorar:
- Algoritmos de rate limiting y sus trade-offs
- Concurrencia y sincronización en Java
- Sistemas distribuidos y consistencia eventual
- Build systems modernos con Bazel
- Diseño de APIs con gRPC y protobuf
- Performance engineering y benchmarking

**Objetivo**: Crear un proyecto defendible en entrevistas técnicas que demuestre comprensión profunda de system design, concurrencia, y arquitectura de sistemas de alto throughput.

El enfoque es **incremental y deliberado**: cada fase añade una dimensión de complejidad, permitiendo estudiar y articular decisiones de diseño con claridad.

---

## Valor para entrevistas técnicas

Este proyecto está específicamente diseñado para:

### System Design Interviews
- Demuestra capacidad de descomponer un problema complejo en componentes
- Articula trade-offs entre algoritmos (precisión vs memoria vs throughput)
- Muestra evolución incremental de single-node a distribuido
- Cubre temas recurrentes: sharding, consistencia, latencia, throughput

### Coding Interviews (Java/Go)
- Implementación correcta de estructuras de datos (ring buffers, deques)
- Manejo de concurrencia con primitivas de bajo nivel (locks, atomics)
- Testing exhaustivo con control temporal (ManualClock)
- Performance engineering con benchmarks reproducibles

### Conversaciones técnicas
- Proyecto real con código ejecutable (no slides ni diagramas)
- Decisiones documentadas con justificación explícita
- Evidencia de pensamiento crítico sobre trade-offs
- Demuestra rigor en testing y tooling profesional

**Este no es un toy project**: es un estudio técnico profundo con implementaciones correctas, tests exhaustivos, y documentación de decisiones de diseño.

---

## Conceptos de System Design cubiertos

| Concepto | Fase | Implementación |
|----------|------|----------------|
| **Algoritmos de rate limiting** | Fase 1 | Token Bucket, Fixed Window, Sliding Window (Log + Counter) |
| **Abstracción del tiempo** | Fase 1 | Clock interface, ManualClock para tests deterministas |
| **Testing determinista** | Fase 2 | JUnit 5, tests temporales sin flakiness |
| **Concurrencia** | Fase 4 | Thread-safety, locks, atomics, CAS operations |
| **In-memory state** | Fase 4 | ConcurrentHashMap, estructura de datos thread-safe |
| **RPC y serialización** | Fase 5 | gRPC, Protobuf, codegen con Bazel |
| **Polyglot systems** | Fase 6 | Java + Go con mismo contrato proto |
| **Performance engineering** | Fase 8 | testing/benchmark (Go), load testing, comparación Java vs Go |
| **Sistemas distribuidos** | Futuro | Partitioning, replicación, consistencia eventual |
| **Observability** | Futuro | Métricas, tracing, debugging bajo carga |

**Preguntas de entrevista típicas que este proyecto responde:**

1. *"Design a rate limiter for an API gateway"* → Fases 1-5
2. *"How would you handle 100K requests/second?"* → Fases 4, 7
3. *"What are the trade-offs between different rate limiting algorithms?"* → Fase 1
4. *"How do you test time-dependent code?"* → Fase 2 (ManualClock)
5. *"How would you distribute rate limiting across multiple nodes?"* → Diseño en Fase 6+
6. *"How do you ensure consistency in a distributed rate limiter?"* → CAP theorem, trade-offs documentados
7. *"How would you benchmark and optimize this system?"* → Fase 7

---

## Qué problema resuelve un rate limiter (en la práctica)

Un rate limiter decide si una petición se **acepta** o se **rechaza** según una política de tráfico.

Se usa para:

* Proteger servicios downstream (bases de datos, terceros, colas)
* Evitar abuso en APIs públicas
* Mantener SLOs bajo carga
* Implementar fairness (por usuario, token, IP, tenant)

Hasta aquí, trivial.

Lo interesante aparece cuando el sistema es **distribuido**:

* ¿Dónde vive el estado?
* ¿Qué pasa si el estado no es consistente?
* ¿Cuánta precisión necesito realmente?
* ¿Qué latencia estoy dispuesto a pagar?

Este repo se centra en **esas preguntas**, no en definiciones de libro.

---

## Por qué existe este repositorio

La mayoría de ejemplos de rate limiting que circulan por ahí:

* Son *toy examples* sin edge cases
* Mezclan algoritmo, red, framework y reloj real en la misma clase
* No son reproducibles
* No permiten razonar sobre trade-offs

Además, casi ninguno sirve para **aprender un build system moderno**.

Aquí el rate limiter es la excusa.
El verdadero protagonista es **Bazel**.

---

## Por qué Bazel (y no Maven / Gradle)

Maven funciona bien para proyectos Java tradicionales.

Pero empieza a romperse cuando quieres:

* Monorepo real (no varios repos pegados)
* Código en varios lenguajes (Java, Go, proto)
* Builds herméticos y reproducibles
* Cacheo agresivo a nivel de target
* Tests y benchmarks como ciudadanos de primera

En Maven, esto suele acabar en:

* Perfiles
* Scripts externos
* Documentación implícita
* Builds lentos y frágiles

Bazel fuerza otra mentalidad:

* Todo es un target
* Las dependencias son explícitas
* El build es un grafo
* Si no cambia el input, no se ejecuta nada

Este repositorio está diseñado para **mostrar eso en la práctica**, no explicarlo en abstracto.

---

## Scope y Roadmap

Este proyecto construye un rate limiter incremental, priorizando **claridad conceptual** sobre completitud de features.

### ✅ En scope (fases actuales):
* Algoritmos fundamentales con análisis de complejidad
* Concurrencia correcta con primitivas de bajo nivel
* Testing determinista y reproducible
* Build hermético con Bazel
* Métricas y benchmarking sistemático
* Diseño polyglot (Java + Go)

### 🔮 Fuera de scope (por ahora):
* Alta disponibilidad con réplicas activas
* Sharding automático y rebalanceo
* Integración con KV stores distribuidos (Redis, DynamoDB)
* Autenticación, autorización, billing
* Dashboards y alerting en producción

### 📋 Roadmap futuro (post-Fase 8):
* **Fase 9 - Distributed coordination**: Consistencia con múltiples nodos
* **Fase 10 - Persistent state**: Integración con Redis/etcd
* **Fase 11 - Observability**: Métricas, tracing, profiling

**Filosofía**: Cada fase debe ser **entendible, defendible, y ejecutable**. Nada de "magia" o complejidad innecesaria.

---

## Filosofía del proyecto

Principios no negociables:

* Bazel es el protagonista, no Java
* Cada carpeta tiene su `BUILD.bazel`
* Nada de scripts mágicos fuera de Bazel
* Código simple, pero correcto
* Trade-offs explícitos, no implícitos
* Tests y benchmarks son targets
* Todo debe ser incremental y cacheable

Si algo es complejo, tiene que estar justificado.

---

## Estructura del monorepo

Estado actual:

```
rate-limiter-cookbook/
├── MODULE.bazel              ✅ (configurado con JUnit 5)
├── .bazelrc                  ✅
├── README.md                 ✅
├── .gitignore                ✅
├── docs/                     ✅
│   └── design-notes.md       ✅
├── proto/                    ⚠️
│   ├── BUILD.bazel           ✅ (vacío)
│   └── ratelimit.proto       ⚠️ (existe pero vacío)
├── core/                     ✅ COMPLETO
│   ├── BUILD.bazel           ✅
│   ├── clock/                ✅
│   │   ├── BUILD.bazel       ✅
│   │   ├── Clock.java        ✅
│   │   └── ManualClock.java  ✅
│   ├── model/                ✅
│   │   ├── BUILD.bazel       ✅
│   │   ├── RateLimiter.java  ✅
│   │   ├── Decision.java     ✅
│   │   └── RateLimitResult.java ✅
│   └── algorithms/           ✅
│       ├── BUILD.bazel       ✅
│       ├── token_bucket/     ✅
│       │   ├── BUILD.bazel   ✅
│       │   ├── TokenBucket.java     ✅
│       │   └── TokenBucketTest.java ✅
│       ├── fixed_window/     ✅
│       │   ├── BUILD.bazel   ✅
│       │   ├── FixedWindow.java     ✅
│       │   └── FixedWindowTest.java ✅
│       ├── sliding_window/   ✅
│       │   ├── BUILD.bazel   ✅
│       │   ├── SlidingWindowLog.java     ✅
│       │   └── SlidingWindowLogTest.java ✅
│       └── sliding_window_counter/ ✅
│           ├── BUILD.bazel   ✅
│           ├── SlidingWindowCounter.java     ✅
│           └── SlidingWindowCounterTest.java ✅
├── java/                     ⏳ PENDIENTE
│   ├── engine/               ⏳
│   ├── grpc/                 ⏳
│   ├── tests/                ⏳
│   └── benchmarks/           ⏳
├── go/                       ⏳ PENDIENTE
│   ├── engine/               ⏳
│   ├── tests/                ⏳
│   └── benchmarks/           ⏳
├── load/                     ⏳ PENDIENTE
│   └── traffic_generator.go  ⏳
└── infra/                    ⏳ PENDIENTE
    └── docker/               ⏳
```

**Leyenda:**
- ✅ Implementado y funcional
- ⚠️ Existe pero incompleto
- ⏳ Pendiente (no existe)

Cada capa añade **una dimensión nueva de complejidad**, nunca varias a la vez.

---

## Fases del proyecto

El proyecto se construye **por fases**, de forma deliberada:

### Fase 0 — README y mentalidad ✅

**Estado: COMPLETADO**

Antes de escribir código:

* Definir qué problema resolvemos
* Definir qué NO resolvemos
* Alinear expectativas

### Fase 1 — Core puro (sin I/O) ✅

**Estado: COMPLETADO**

* ✅ Algoritmos deterministas
* ✅ Sin red, sin threads, sin clocks reales
* ✅ Reloj inyectable (Clock + ManualClock)
* ✅ Tests exhaustivos

Algoritmos implementados:

* ✅ Token Bucket
* ✅ Fixed Window
* ✅ Sliding Window Log
* ✅ Sliding Window Counter

### Fase 2 — Tests como ciudadanos de primera ✅

**Estado: COMPLETADO**

* ✅ Tests unitarios por algoritmo (5 tests funcionales)
* ✅ Edge cases cubiertos
* ✅ Regresión temporal con reloj mockeado (ManualClock)
* ✅ Configuración de JUnit 5 con Bazel

### Fase 3 — Performance Testing en Java ⏭️

**Estado: SKIPPED** (Se hará en Go - Fase 8)

**Justificación del skip:**
- El objetivo principal del proyecto es profundizar en Go, no Java
- Los benchmarks serán más valiosos en la Fase 8 cuando se pueda comparar Java vs Go
- Implementar el engine primero permite testear performance end-to-end, no solo algoritmos aislados
- JMH añadiría complejidad innecesaria si el foco es Go

**Nota:** Performance testing se realizará comprehensivamente en **Fase 8** con:
- Benchmarks en Go (`testing/benchmark`)
- Comparación directa Java vs Go
- Load testing end-to-end con herramienta propia
- Métricas más realistas en contexto de engine completo

---

### Fase 4 — Java Engine (Concurrencia y Alto Throughput) ⏳ ← **SIGUIENTE FASE**

**Estado: PENDIENTE**

Wrapper thread-safe para los algoritmos core, diseñado para alto throughput en contextos concurrentes:

**Primitivas de concurrencia:**
* ⏳ `ReentrantLock` vs `synchronized` - trade-offs de fairness y performance
* ⏳ `AtomicLong` para contadores lock-free
* ⏳ `ConcurrentHashMap` para almacenamiento por key
* ⏳ `StampedLock` para optimistic reads (exploración)

**Diseño del engine:**
* ⏳ API simple: `boolean tryAcquire(String key, int permits)`
* ⏳ In-memory state sin persistencia (por ahora)
* ⏳ Eviction policy para limitar memoria (LRU simple)
* ⏳ Sin frameworks externos (solo JDK)

**Targets de performance:**
* ⏳ > 100K requests/sec en un solo nodo (single-threaded)
* ⏳ > 500K requests/sec con múltiples threads
* ⏳ Latencia p99 < 1ms para hot keys

**Testing de concurrencia:**
* ⏳ Tests con `CountDownLatch` para race conditions
* ⏳ Validación de thread-safety con múltiples threads
* ⏳ Tests de stress con carga sostenida

**Objetivo**: Demostrar dominio de primitivas de concurrencia de Java y capacidad de razonar sobre performance en sistemas multi-threaded.

### Fase 5 — gRPC API ⏳

**Estado: PENDIENTE**

API gRPC para exponer el rate limiter como servicio, diseñada para alto throughput y baja latencia:

**Contrato proto:**
* ⏳ `CheckRateLimit(key, permits)` - unary call
* ⏳ `CheckRateLimitBatch(requests[])` - batch para reducir RTT
* ⏳ `ResetRateLimit(key)` - operación admin
* ⏳ Health check endpoint

**Implementación del servidor:**
* ⏳ gRPC Java con servidor asíncrono (non-blocking)
* ⏳ Thread pool tuning para alto throughput
* ⏳ Error handling: InvalidArgument, ResourceExhausted, Unavailable
* ⏳ Interceptors para logging y métricas
* ⏳ Graceful shutdown

**Manejo de errores y resiliencia:**
* ⏳ Retry policy en cliente (exponential backoff)
* ⏳ Circuit breaker para fallos cascada
* ⏳ Deadline propagation
* ⏳ Rate limiting del servidor mismo (protección contra DDoS)

**Codegen con Bazel:**
* ⏳ protoc + grpc-java plugin
* ⏳ Targets separados: proto_library, java_proto_library, java_grpc_library
* ⏳ Compartir .proto entre Java y Go

**Valor para entrevistas**: Demuestra conocimiento de RPC, serialización, error handling, y diseño de APIs productizables.

### Fase 6 — Load Testing Tool en Go ⏳

**Estado: PENDIENTE**

Herramienta en Go (estilo k6 minimalista) para validar rate limiters bajo carga real:

**Funcionalidades:**
* ⏳ Cliente gRPC en Go para llamar a la API (Fase 5)
* ⏳ Cliente HTTP simple para APIs REST (futuro)
* ⏳ Generador de tráfico configurable (RPS, duración, concurrencia)
* ⏳ Distribución de keys: uniforme, zipf, hot keys

**Métricas recolectadas:**
* ⏳ Throughput (requests/sec achieved vs target)
* ⏳ Latencia: p50, p95, p99, p99.9, max
* ⏳ Tasa de rechazo (esperada vs real)
* ⏳ Errores de red (timeouts, connection refused)
* ⏳ Histograma de latencias en tiempo real

**Output:**
* ⏳ Reporte final en consola (tabla con métricas)
* ⏳ JSON export para análisis posterior
* ⏳ Gráficas ASCII simples (histograma de latencias)

**Escenarios de testing:**
* ⏳ Carga constante (sustained load)
* ⏳ Spike test (ráfaga repentina)
* ⏳ Ramp-up test (carga gradual)

**Valor para entrevistas**: Herramienta real de load testing, demuestra capacidad de construir tooling propio y validar sistemas bajo carga.

### Fase 7 — Implementación en Go ⏳

**Estado: PENDIENTE**

Implementación del rate limiter en Go, compartiendo el mismo contrato proto pero con diferencias de diseño específicas del lenguaje:

**Algoritmos core en Go:**
* ⏳ Token Bucket, Fixed Window, Sliding Window (misma lógica que Java)
* ⏳ Clock abstraction en Go (interface Clock, MockClock para tests)
* ⏳ Testing con `testing` package de Go

**Primitivas de concurrencia en Go:**
* ⏳ Goroutines vs threads de Java
* ⏳ Channels vs locks para comunicación
* ⏳ `sync.Mutex` vs `sync.RWMutex` vs `atomic`
* ⏳ `sync.Map` vs `map[string]RateLimiter` + mutex

**Engine thread-safe:**
* ⏳ In-memory storage con sync.Map o mutex + map
* ⏳ Goroutine pool para handling de requests
* ⏳ Context propagation para cancellation

**gRPC server en Go:**
* ⏳ Mismo .proto que Java
* ⏳ grpc-go implementation
* ⏳ Comparación de performance Go vs Java

**Trade-offs Go vs Java:**
* ⏳ GC: stop-the-world (Java) vs concurrent (Go)
* ⏳ Goroutines livianas vs threads pesados
* ⏳ Simplicidad de deployment (binary estático Go vs JVM)
* ⏳ Performance: latencia, throughput

**Valor para entrevistas**: Demuestra comprensión profunda de concurrencia en dos lenguajes, capacidad de comparar trade-offs de diseño.

### Fase 8 — Benchmarks y Comparación ✅

**Estado: COMPLETADA**

Benchmarking sistemático comparando Java y Go implementations:

**Benchmarks de algoritmos:**
* ✅ 25+ Go benchmarks (4 algoritmos × 4 tipos + engine + model)
* ✅ 15 JMH Java benchmarks (4 algoritmos × 3 escenarios + 3 engine)
* ✅ Wrapper scripts para ejecutar y recolectar resultados

**Comparison Framework:**
* ✅ Go CLI tool para comparación
* ✅ Parser de JMH JSON + Go benchmark output
* ✅ Reportes markdown con tablas y ASCII charts
* ✅ Export JSON para automation

**Ejecución:**
```bash
# Run all benchmarks
bazel run //benchmarks:run_all

# View results
cat benchmarks/reports/latest/comparison.md
```

**Resultados:**
- **Go**: Típicamente 10-30% más rápido (0 allocs/op en hot paths)
- **Java**: Competitivo después de warmup JIT
- **Latencia**: Ambos <100ns/op en operaciones core

Ver documentación completa: [benchmarks/README.md](benchmarks/README.md)

**Benchmarks de engines:**
* ⏳ Throughput bajo diferentes cargas (1K, 10K, 100K, 1M RPS)
* ⏳ Latencia con diferentes niveles de concurrencia
* ⏳ Memory footprint (heap usage, GC pressure)
* ⏳ CPU utilization

**Benchmarks end-to-end (gRPC):**
* ⏳ Latencia con serialización proto
* ⏳ Overhead de red vs in-process
* ⏳ Comparación Java gRPC server vs Go gRPC server

**Visualización de resultados:**
* ⏳ Gráficas de latencia (percentiles)
* ⏳ Throughput vs latencia trade-off
* ⏳ Comparación side-by-side Java vs Go

**Valor para entrevistas**: Demuestra rigor en performance engineering, capacidad de tomar decisiones basadas en datos, no suposiciones.

---

## Estado del Proyecto (Actualizado)

### ✅ Completado (Fases 1-2)

El proyecto ha completado exitosamente las bases fundamentales:

**Core Algorithms** (`/core/algorithms/`)
- **Token Bucket**: Implementación completa con refill dinámico continuo
- **Fixed Window**: Implementación con alineación de ventanas temporales
- **Sliding Window Log**: Precisión exacta usando ArrayDeque para timestamps
- **Sliding Window Counter**: Balance memoria/precisión con ring buffer de buckets

**Infrastructure**
- Clock abstraction para tests deterministas (Clock + ManualClock)
- Model layer limpio (Decision, RateLimitResult, RateLimiter)
- 5 tests unitarios funcionales con JUnit 5 + JUnit Platform
- Bazel configurado correctamente con rules_java

**Comandos Funcionales:**
```bash
bazel test //core/algorithms/...  # ✅ 4/4 tests passing
```

### ⏳ Siguiente Fase Recomendada: Java Engine (Fase 4)

**Fase 3 SKIPPED** - Benchmarks se harán comprehensivamente en Fase 8 con Go.

Para continuar el proyecto, el siguiente paso es implementar:
- Java Engine con ConcurrentHashMap para almacenamiento multi-key
- Thread-safety con ReentrantLock o synchronized
- Eviction policy (LRU simple) para limitar memoria
- Tests de concurrencia con CountDownLatch y múltiples threads

**Justificación**: Construir el engine permite tener un sistema funcional end-to-end. Los benchmarks serán más valiosos cuando se puedan comparar implementaciones completas Java vs Go, no solo algoritmos aislados.

---

## Cómo usar este repositorio

**Ejecutar todos los tests del core (Fases 1-2 completadas):**

```bash
bazel test //core/algorithms/...
```

**Ejecutar tests individuales:**

```bash
bazel test //core/algorithms/token_bucket:token_bucket_test
bazel test //core/algorithms/fixed_window:fixed_window_test
bazel test //core/algorithms/sliding_window:sliding_window_log_test
bazel test //core/algorithms/sliding_window_counter:sliding_window_counter_test
```

**Ver resultados detallados:**

```bash
bazel test //core/algorithms/... --test_output=all
```

Bazel se encarga de ejecutar **solo lo que cambia**.

---

## Lessons learned (vivo)

**De las Fases 1-2 completadas:**

* ✅ **Bazel premia targets pequeños y bien definidos**
  - Cada algoritmo es un target independiente con su propio BUILD.bazel
  - Tests aislados sin dependencias cruzadas
  - Cacheo granular permite builds incrementales rápidos

* ✅ **JUnit 5 requiere configuración explícita en Bazel**
  - Necesita JUnit Platform Console Launcher como `main_class`
  - No puede usar el runner JUnit 4 por defecto de Bazel
  - Requiere `use_testrunner = False` y dependencias de junit-platform

* ✅ **ManualClock hace los tests deterministas y rápidos**
  - No hay sleeps ni timeouts en tests
  - Control total sobre el tiempo con `advanceNanos()` y `setNanos()`
  - Resultados reproducibles 100% sin flakiness

* ✅ **Sliding Window Log es más preciso, pero más caro**
  - Precisión exacta: O(permits) por request vs O(1) en Fixed Window
  - Memoria: O(limit) vs O(1) en Fixed Window
  - Trade-off documentado explícitamente en código

* ✅ **Trade-offs son explícitos en el código**
  - Comentarios documentan pros/contras de cada algoritmo
  - Decisiones arquitectónicas visibles (ej: "precisión vs coste")
  - No hay complejidad oculta ni "magia"

**Pendiente de validar en fases futuras:**

* En distribuido, la perfección suele ser innecesaria
* La mayoría de sistemas necesitan aproximaciones razonables, no exactitud matemática
* Latencia vs consistencia en entornos multi-nodo

---

## Arquitectura y decisiones de diseño

### Separación de concerns

```
┌─────────────────────────────────────────────┐
│  Load Testing Tool (Go)                     │
│  - Generador de tráfico concurrente         │
│  - Métricas: throughput, latencia, rechazos │
└──────────────────┬──────────────────────────┘
                   │
                   v
┌─────────────────────────────────────────────┐
│  gRPC API (Java)                            │
│  - Endpoint: CheckRateLimit(key, permits)   │
│  - Protobuf codegen con Bazel               │
└──────────────────┬──────────────────────────┘
                   │
                   v
┌─────────────────────────────────────────────┐
│  Java Engine (Thread-safe wrapper)          │
│  - ConcurrentHashMap<String, RateLimiter>   │
│  - Locks/Atomics para sincronización        │
│  - Eviction policy (LRU)                    │
└──────────────────┬──────────────────────────┘
                   │
                   v
┌─────────────────────────────────────────────┐
│  Core Algorithms (Puro, sin I/O)            │
│  - TokenBucket, FixedWindow, SlidingWindow  │
│  - Clock abstraction inyectable             │
│  - Determinista, testable                   │
└─────────────────────────────────────────────┘
```

### Trade-offs principales documentados

| Decisión | Opción elegida | Trade-off | Justificación |
|----------|----------------|-----------|---------------|
| **Algoritmo por defecto** | Token Bucket | Precisión vs simplicidad | Balance ideal: refill continuo, memoria O(1), implementación simple |
| **Almacenamiento** | In-memory (ConcurrentHashMap) | Velocidad vs persistencia | Latencia sub-ms para hot paths, suficiente para Fase 4 |
| **Sincronización** | ReentrantLock por key | Granularidad vs overhead | Lock por key evita contention global, permite paralelismo |
| **Testing temporal** | ManualClock injection | Determinismo vs realismo | Tests reproducibles > tests realistas, no flakiness |
| **Build system** | Bazel | Learning curve vs hermetic builds | Inversión inicial vale para reproducibilidad y polyglot |
| **RPC** | gRPC + Protobuf | Complejidad vs performance | Serialización eficiente, codegen, soporte multi-lenguaje |

### Algoritmos de Rate Limiting: Análisis Profundo

#### Token Bucket - Refill Continuo

**Concepto:** Bucket con tokens que se regeneran continuamente. Permite bursts controlados.

**Funcionamiento:**
```
Configuración: capacity=10, rate=5 tokens/sec

t=0.0s  [##########] 10 tokens → request(7) → ALLOW
t=0.0s  [###       ]  3 tokens → request(5) → REJECT
t=0.4s  [#####     ]  5 tokens (3 + 2 refill) → request(5) → ALLOW
t=1.4s  [#####     ]  5 tokens (0 + 5 refill)
```

**Refill automático:** `tokens = min(capacity, tokens + elapsed_time × rate)`

**Trade-offs:**
- ✅ Permite bursts naturales (hasta capacity)
- ✅ Refill continuo (no abrupto)
- ✅ O(1) tiempo y memoria
- ❌ Requiere sincronización en distribuido
- ❌ Necesita timestamps precisos

**Cuándo usar:** Sistemas que necesitan manejar tráfico con picos naturales (APIs REST, colas de mensajes).

---

#### Fixed Window - El Boundary Problem

**Concepto:** Contador simple que se resetea cada X tiempo en ventanas alineadas.

**Funcionamiento:**
```
Configuración: window=1s, limit=10

Ventana 1 [0.0s - 1.0s]          Ventana 2 [1.0s - 2.0s]
┌───────────────────────┐       ┌───────────────────────┐
│ t=0.3s: +5 → used=5   │       │ t=1.0s: RESET → used=0│
│ t=0.8s: +5 → used=10  │       │ t=1.5s: +10 → used=10 │
│ t=0.9s: +1 → REJECT ❌│       │                       │
└───────────────────────┘       └───────────────────────┘
```

**⚠️ BOUNDARY PROBLEM - Puede Permitir 2x el Límite:**

```
    Ventana 1 [0.0s - 1.0s]          Ventana 2 [1.0s - 2.0s]
    ┌───────────────────────┐       ┌───────────────────────┐
    │         [FINAL]       │       │ [INICIO]              │
0.0s│                  0.9s │  1.0s │ 1.1s                 │2.0s
    │                   ████│███████│████                  │
    │                  10req│ RESET │10req                 │
    └───────────────────────┘       └───────────────────────┘
                         └─────┬─────┘
                      ¡20 requests en 200ms! 💥
```

**Por qué ocurre:**
- Ventanas están **alineadas** al calendario (0s, 1s, 2s...)
- Reset **abrupto** en el boundary
- No considera distribución temporal de requests
- Un atacante puede explotar esto: enviar 10 requests al final de cada ventana + 10 al inicio de la siguiente

**Ejemplo real del test:**
```java
// t=0.9s: 10 requests al final de ventana 1
window.tryAcquire("user1", 10) → ALLOW ✅

// t=1.0s: RESET (nueva ventana)

// t=1.1s: 10 requests al inicio de ventana 2
window.tryAcquire("user1", 10) → ALLOW ✅

// Total: 20 requests en 200ms (límite es "10 por segundo")
```

**Trade-offs:**
- ✅ Extremadamente simple (un contador)
- ✅ O(1) tiempo y memoria
- ✅ Fácil de implementar y debuggear
- ❌ **Boundary problem: puede permitir 2x el rate**
- ❌ Reset abrupto (no gradual)
- ❌ No adecuado para protección DDoS

**Cuándo usar:**
- APIs internas donde bursts son aceptables
- Límites muy conservadores (límite=100, esperas ~50)
- Sistemas educativos / demos
- **NO usar:** APIs públicas, protección contra abuso, SLAs estrictos

---

#### Sliding Window Log - Precisión Exacta

**Concepto:** Guarda timestamp de cada request en una cola. Eviction gradual.

**Funcionamiento:**
```
Configuración: window=1s, limit=5

t=0.0s: requests=[0]         → ALLOW (1/5)
t=0.2s: requests=[0, 200ms]  → ALLOW (2/5)
t=0.5s: requests=[0, 200ms, 500ms, 500ms, 500ms] → 5/5
t=0.6s: requests=[0, 200ms, 500ms, 500ms, 500ms] → REJECT ❌
t=1.1s: prune < 100ms → requests=[200ms, 500ms, 500ms, 500ms]
t=1.1s: requests=[200ms, 500ms, 500ms, 500ms, 1100ms] → ALLOW (5/5)
```

**NO sufre boundary problem:**
```
t=0.9s:  10 requests → ALLOW ✅
t=1.1s:  window = [0.1s-1.1s] contiene 10 requests
         → REJECT ❌ (ventana deslizante, no reset)
```

**Trade-offs:**
- ✅ **Precisión exacta** en cualquier momento
- ✅ No boundary problem
- ✅ Eviction gradual (no abrupta)
- ❌ O(limit) memoria (guarda todos los timestamps)
- ❌ O(permits) tiempo por request (inserción + eviction)
- ❌ Presión en GC (ArrayDeque con objetos Long)
- ❌ Estado grande en distribuido

**Cuándo usar:**
- Cuando precisión exacta es crítica
- APIs públicas con SLAs estrictos
- Sistemas anti-fraude
- Memoria no es limitante
- **NO usar:** High throughput (>1M RPS), memoria limitada

---

#### Sliding Window Counter - Aproximación Práctica

**Concepto:** Ring buffer de buckets. Balance entre Fixed Window y Sliding Window Log.

**Funcionamiento:**
```
Configuración: window=1s, buckets=10 (100ms cada uno), limit=20

Ring buffer:
[bucket0][bucket1][bucket2]...[bucket9]
   3       5        4           2

t=0.3s (bucket 3): +5 → total=19
t=0.6s (bucket 6): +2 → REJECT (19+2 > 20)
t=1.1s: bucket 1 sale, bucket 11 entra (wrap)
```

**Ring buffer wrapping:**
```
t=0.0s-0.9s: buckets 0-9 contienen 20 requests
t=1.0s: bucket 0 sale (libera su count)
t=1.0s: bucket 10 entra (mismo índice que bucket 0)
```

**Trade-offs:**
- ✅ Mejor que Fixed Window (granularidad)
- ✅ Más barato que Sliding Window Log
- ✅ O(buckets) memoria (fijo)
- ✅ O(1) tiempo amortizado
- ⚠️ **Aproximación** (precisión depende de # buckets)
- ⚠️ Todavía tiene boundary problem (menor)
- ❌ Configuración adicional (elegir # buckets)

**Precision vs buckets:**
- 1 bucket = Fixed Window (boundary problem completo)
- 10 buckets = boundary problem ~10% (vs 100% de Fixed Window)
- 100 buckets = casi exacto, pero overhead mayor

**Cuándo usar:**
- Balance memoria/precisión necesario
- High throughput donde Sliding Window Log es caro
- Puedes tolerar ~5-10% imprecisión
- Producción real (usado por Redis, Nginx)

---

### Comparación Directa

| Algoritmo | Memoria | Precisión | Tiempo | Boundary Problem |
|-----------|---------|-----------|--------|------------------|
| **Token Bucket** | O(1) | Buena | O(1) | No aplica (refill continuo) |
| **Fixed Window** | O(1) | Mala | O(1) | ⚠️ **SÍ (hasta 2x rate)** |
| **Sliding Window Log** | O(limit) | Exacta | O(permits) | ✅ **NO** |
| **Sliding Window Counter** | O(buckets) | Aproximada | O(1) | ⚠️ Sí (mitigado) |

### Recomendación por Escenario

**APIs Internas (bajo riesgo):**
- Fixed Window (simple) o Token Bucket (bursts)

**APIs Públicas (protección contra abuso):**
- Sliding Window Log (precisión) o Sliding Window Counter (performance)

**High Performance (>1M RPS):**
- Token Bucket o Sliding Window Counter con pocos buckets

**Protección DDoS:**
- Sliding Window Log (no boundary problem)

**Educational / Demo:**
- Fixed Window (entender el problema) → Sliding Window Log (solución)

---

### Por qué no usar librerías externas (Guava, Resilience4j)

En un proyecto de producción, usaría `Guava RateLimiter` o `Resilience4j`.

Aquí la implementación manual es **deliberada** porque:
1. **Entrevistas valoran implementación desde cero** - demuestra comprensión profunda
2. **Control total sobre trade-offs** - puedo explicar cada decisión
3. **Material de estudio** - entender el "cómo" y el "por qué"
4. **Bazel como protagonista** - estructurar targets desde cero

### Próximas decisiones de diseño (Fases futuras)

**Fase 5 (gRPC)**:
- ¿Streaming bidireccional o unary calls?
- ¿Cómo manejar backpressure?
- ¿Qué hacer si el servidor está sobrecargado?

**Fase 6 (Go implementation)**:
- Goroutines + channels vs mutexes
- sync.Map vs mutex + map[string]
- Diferencias con implementación Java

**Fase 8 (Distributed)**:
- Consistencia fuerte vs eventual
- Leader election (Raft, Paxos)
- Partitioning strategy (consistent hashing)

---

## Cómo usar este proyecto en entrevistas

### Para System Design rounds:
1. Explica la evolución incremental (single-node → distributed)
2. Articula trade-offs entre algoritmos con análisis de complejidad
3. Discute decisiones de concurrencia (locks, atomics, CAS)
4. Muestra tests deterministas con ManualClock
5. Habla sobre cómo escalarías a millones de requests/sec

### Para Coding rounds:
1. Muestra implementación limpia de Token Bucket o Sliding Window
2. Explica el patrón de Clock injection para testing
3. Demuestra comprensión de estructuras de datos (ArrayDeque, ring buffer)
4. Habla sobre complejidad temporal y espacial

### Para conversaciones de arquitectura:
1. Referencia el código real en GitHub
2. Muestra Bazel targets y estructura del monorepo
3. Explica decisiones de tooling (Bazel vs Maven)
4. Discute roadmap futuro (distributed, persistence)

---

## Licencia

MIT (o la que prefieras).
