# Rate Limiter Cookbook (Bazel-first) 🧱

Este repo existe para aprender **Bazel** y **system design** construyendo un rate limiter distribuido, paso a paso.
No es un producto. Es un **cookbook técnico**: cada fase añade una pieza útil, con trade-offs explicados.

## Qué problema resuelve un rate limiter
Un rate limiter decide si una petición entra o se rechaza para:
- Proteger backends y dependencias (DB, APIs externas).
- Evitar “noisy neighbors” y abusos.
- Mantener SLOs bajo picos y ataques.
- Hacer fair-sharing entre tenants/keys.

## Por qué este repo existe
- Para aprender Bazel de forma práctica (targets, cache, incrementalidad, polyglot).
- Para comparar algoritmos reales (precisión vs coste vs estado).
- Para tener un monorepo defendible en entrevista: decisiones técnicas, no “Hello world”.

## Por qué Bazel (vs Maven/Gradle)
Bazel brilla en este tipo de repo porque:
- **Incremental real**: recompila y re-testeaa solo lo afectado.
- **Cache reproducible**: mismas entradas → mismos outputs.
- **Monorepo polyglot**: Java + Go + Proto con un solo grafo de build.
- **Tests y benchmarks como targets**: no scripts sueltos.

## Qué NO intenta resolver
- No “global distributed consistency” perfecta (CAP pega fuerte).
- No incluye aún storage distribuido real (Redis, etcd) en fases iniciales.
- No optimiza al extremo desde el día 1 (primero correctitud + claridad).

## Estructura del repo
- `core/`: algoritmos puros, deterministas, sin I/O.
- `java/`: engine thread-safe y server gRPC.
- `go/`: implementación Go del contrato.
- `proto/`: contrato común.
- `load/`: generador de tráfico reproducible.
- `docs/`: notas de diseño y lecciones.

## Cómo ejecutar (FASE 0-1)
### Tests del core
```bash
bazel test //core/...
bazel query //core/...
````
Fases

FASE 0: README + mentalidad (este archivo)

FASE 1: core puro (sin I/O) + tests deterministas

FASE 2: tests como ciudadanos de primera + edge/regresión temporal

FASE 3: Java engine thread-safe (alto throughput)

FASE 4: gRPC API (proto + server Java)

FASE 5: Go implementation (mismo proto)

FASE 6: Benchmarks (JMH / testing.B)

FASE 7: Load testing reproducible

Lessons learned (se rellena por fases)

(pendiente)

### `docs/design-notes.md` (arranca ya, aunque crezca luego)

```md
# Design notes

## Mental model
Un rate limiter decide: ALLOW / REJECT basándose en (key, now, policy).
En distribuido, el gran problema no es el algoritmo: es el **estado**.

## Trade-offs (primeros)
- Token Bucket: buen “burst”, buen promedio, fácil de razonar.
- Fixed Window: barato, pero tiene boundary spikes.
- Sliding Window (log): preciso, pero coste O(n) y estado por request.
- Sliding Window Counter: aproximación buena, estado acotado, más compleja.

## Determinismo (FASE 1)
Todo el core se diseña para:
- no usar reloj real
- no usar threads
- no tener I/O
- ser testeable con un clock fake